package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/cimb-niaga/cms/backend/internal/approval"
	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/auth"
	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/handler"
	"github.com/cimb-niaga/cms/backend/internal/repository"
	"github.com/cimb-niaga/cms/backend/internal/rolemgmt"
	"github.com/cimb-niaga/cms/backend/internal/service"
	"github.com/cimb-niaga/cms/pkg/config"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"

	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load configuration from environment
	cfg, err := config.Load("8080")
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Connect to PostgreSQL
	ctx := context.Background()
	dbPool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to create database pool", "error", err)
		os.Exit(1)
	}
	defer dbPool.Close()

	if err := dbPool.Ping(ctx); err != nil {
		slog.Error("failed to ping database", "error", err)
		os.Exit(1)
	}
	slog.Info("connected to PostgreSQL")

	// Read-replica pool, used only by the master-data CSV export for now (plan
	// T5.1; the other "swap dbPool for the dbRead pool" notes below are still
	// pending). Heavy reads belong on the replica (CLAUDE.md Sec 6). When
	// DATABASE_REPLICA_URL is unset (local/dev) the export reads the primary.
	// A configured-but-unreachable replica is a startup failure, like the primary.
	dbReadPool := dbPool
	if cfg.DatabaseReplicaURL != "" {
		replicaPool, err := pgxpool.New(ctx, cfg.DatabaseReplicaURL)
		if err != nil {
			slog.Error("failed to create read-replica pool", "error", err)
			os.Exit(1)
		}
		defer replicaPool.Close()
		if err := replicaPool.Ping(ctx); err != nil {
			slog.Error("failed to ping read replica", "error", err)
			os.Exit(1)
		}
		dbReadPool = replicaPool
		slog.Info("connected to PostgreSQL read replica")
	} else {
		slog.Warn("DATABASE_REPLICA_URL not set; master-data export reads from the primary")
	}

	// Connect to Redis
	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		slog.Error("failed to parse Redis URL", "error", err)
		os.Exit(1)
	}
	redisClient := redis.NewClient(redisOpts)
	defer func() { _ = redisClient.Close() }()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		slog.Error("failed to ping Redis", "error", err)
		os.Exit(1)
	}
	slog.Info("connected to Redis")

	// Set up Chi router
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Health check endpoint
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Initialize auth dependencies
	tokenBlacklist := pkgauth.NewRedisTokenBlacklist(redisClient)
	tokenService := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          cfg.JWTSecret,
		AccessTokenExpiry:  cfg.AccessTokenExpiry,
		RefreshTokenExpiry: cfg.RefreshTokenExpiry,
	}, tokenBlacklist)

	userRepo := repository.NewAuthRepository(dbPool)

	// auditWriter is shared by every module that writes audit_logs (change-
	// password here, approvals below) — one writer, one DB pool.
	auditWriter := audit.NewWriter(dbPool)

	rateLimiter := custommw.NewRateLimiter(redisClient, custommw.RateLimitConfig{
		MaxPerUsername: cfg.RateLimitUsername,
		MaxPerIP:       cfg.RateLimitIP,
		Window:         cfg.RateLimitWindow,
	})

	// Create auth service with local provider
	localProvider := auth.NewLocalProvider(userRepo)
	authService := auth.NewService(
		[]pkgauth.Provider{localProvider},
		tokenService,
		userRepo,
		rateLimiter,
	)
	changePasswordService := auth.NewChangePasswordService(userRepo, auditWriter)

	// Create and mount auth handler
	authHandler := handler.NewAuthHandler(authService, tokenService, userRepo, rateLimiter, changePasswordService)
	r.Mount("/api/v1/auth", authHandler.Routes())

	// APPACCESS/ADMIN/ADMIN_PARAM: set a target user's initial password,
	// forcing a change on their next login (Auth-Local-Lifecycle Task 6),
	// plus the full admin user CRUD (Admin User & Vendor Management spec):
	// list/get/create/update (UserAdminService) and disable/enable (the
	// existing DeactivateUserService, previously unwired). ADMIN is the
	// superuser role and must reach every admin area, matching vendors/atms.
	setInitialPasswordService := auth.NewSetInitialPasswordService(userRepo, auditWriter)
	userAdminRepo := repository.NewUserAdminRepository(dbPool)
	userAdminService := auth.NewUserAdminService(userAdminRepo, auditWriter)
	deactivateService := auth.NewDeactivateUserService(userRepo, auditWriter)
	adminUserHandler := handler.NewAdminUserHandler(setInitialPasswordService, userAdminService, deactivateService)
	r.With(
		custommw.RequireAuth(tokenService),
		custommw.RequireRoles("APPACCESS", "ADMIN", "ADMIN_PARAM"),
	).Mount("/api/v1/admin/users", adminUserHandler.Routes())

	// (vendor and ATM admin CRUD are wired further down, next to the master-data
	// change service they stage through -- plan.md T4.1/T4.2.)

	// APPACCESS/ADMIN-only: Role Management (.kiro/specs/role-management) —
	// data-driven menu/feature permission catalog + mapping. Self-guarded by
	// the static RequireRoles here (not by rolemgmt.RequirePermission itself)
	// to avoid a bootstrap deadlock ("who grants APPACCESS permission to
	// manage permissions") — see design.md "Middleware". Immediate-apply,
	// audit-only (documented deviation from Golden Rule #3, recorded in
	// project-context.md Sec 12).
	// ponytail: swap dbPool for the dbRead pool on ListRoles/ListCatalog when
	// DATABASE_REPLICA_URL wiring lands (same TODO convention as above).
	roleMgmtRepo := rolemgmt.NewRepository(dbPool, dbPool)
	roleMgmtService := rolemgmt.NewPermissionService(roleMgmtRepo, dbPool)
	roleMgmtHandler := handler.NewRoleMgmtHandler(roleMgmtService)
	r.With(
		custommw.RequireAuth(tokenService),
		custommw.RequireRoles("APPACCESS", "ADMIN"),
	).Mount("/api/v1/admin/roles", roleMgmtHandler.Routes())

	// Create and mount ATM Portal handler, protected by RequireAuth
	atmPortalService := service.NewAtmPortalService(db.New(dbPool))
	atmPortalHandler := handler.NewAtmPortalHandler(atmPortalService)
	r.With(custommw.RequireAuth(tokenService)).Mount("/api/v1/atm-portal", atmPortalHandler.Routes())

	// Create and mount DMAA Forecast handler (read-only viewer, read-replica
	// bound; currently the shared pool until the replica pool is wired).
	// ponytail: swap dbPool for the dbRead pool when DATABASE_REPLICA_URL wiring lands
	dmaaForecastService := service.NewDmaaForecastService(db.New(dbPool))
	dmaaForecastHandler := handler.NewDmaaForecastHandler(dmaaForecastService)
	r.With(
		custommw.RequireAuth(tokenService),
		custommw.RequireRoles("ATM-USER", "ATM-SPV", "BRANCH-ATM-USER", "BRANCH-ATM-SPV", "ADMIN", "ADMIN_PARAM"),
	).Mount("/api/v1/dmaa-forecast", dmaaForecastHandler.Routes())

	// Create and mount DSR upload handler. ATM-specific env vars are read
	// directly here (not via pkg/config.Config) since that struct is shared
	// with backend-cit and these settings are ATM-only.
	dsrUploadDir := getenvDefault("DSR_UPLOAD_DIR", "FTP_DATA/DSR")
	dsrRetryBaseURL := getenvDefault("DSR_RETRY_SCHEDULER_BASE_URL", "http://localhost:8090")
	dsrProcessAuth := os.Getenv("DSR_RETRY_SCHEDULER_AUTH")
	dsrHTTPClient := &http.Client{Timeout: 30 * time.Second}
	dsrService := service.NewDsrService(db.New(dbPool), redisClient, dsrHTTPClient, dsrUploadDir, dsrRetryBaseURL, dsrProcessAuth)
	dsrHandler := handler.NewDsrUploadHandler(dsrService)
	r.With(custommw.RequireAuth(tokenService)).Mount("/api/v1/dsr", dsrHandler.Routes())

	// Build the master-data maker-checker engine (plan.md Fase 2) ahead of
	// the approval handler below, since the approval handler now routes
	// through it (not the raw orchestrator) so the apply-on-approve hook
	// (T2.4) actually fires on the live approve/reject path.
	//
	// ApplierRegistry: each entity_type registers its applier as its own
	// CRUD support lands (Fase 3/4) -- "vendor" (T2.3) + "vendor_branch"
	// (T3.1) so far.
	// ponytail: swap dbPool for the dbRead pool on List/Count when
	// DATABASE_REPLICA_URL wiring lands (same TODO convention as above).
	masterDataApplierRegistry := service.ApplierRegistry{
		"vendor":         service.VendorApplier{},
		"vendor_branch":  service.VendorBranchApplier{},
		"vendor_vault":   service.VendorVaultApplier{},
		"vendor_pic":     service.VendorPicApplier{},
		"atm":            service.ATMApplier{},
		"vendor_package": service.VendorPackageApplier{},
		"atm_assignment": service.ATMAssignmentApplier{},
	}
	masterDataChangeRepo := repository.NewMasterDataChangeRepository(dbPool)

	// Create and mount the approval handler. Mounted behind RequireAuth only
	// (no RequireRoles): submit must be reachable by any authenticated maker
	// regardless of role, since role != approval hierarchy (RBAC-Setup). The
	// real per-request authorization (maker != checker, actor == effective
	// approver) is enforced inside the orchestrator itself, not at the route.
	// (auditWriter created earlier, shared with the change-password service.)
	//
	// masterDataApprovalService wraps approvalOrchestrator: for every
	// document_type except "master_data" it is a pure passthrough
	// (identical behavior to the raw orchestrator); for "master_data" it
	// also runs the apply-on-approve hook once the final approval step
	// passes. Wiring it here (not the raw orchestrator) is what makes T3.1+
	// admin endpoints' approvals actually take effect via this shared
	// endpoint.
	approvalRepo := approval.NewRepository(dbPool)
	approvalOrchestrator := approval.NewOrchestrator(approvalRepo, approvalRepo, approvalRepo, auditWriter, nil)
	masterDataApprovalService := service.NewMasterDataApprovalService(approvalOrchestrator, masterDataChangeRepo, dbPool, masterDataApplierRegistry, auditWriter)
	approvalHandler := handler.NewApprovalHandler(masterDataApprovalService, approvalRepo).WithMasterDataDetail(masterDataChangeRepo).WithApplyRetry(masterDataApprovalService)
	r.With(custommw.RequireAuth(tokenService)).Mount("/api/v1/approvals", approvalHandler.Routes())

	// ADMIN/ADMIN_PARAM-only: read access to the master-data maker-checker
	// change log (plan.md T2.7) -- list (filter entity_type/status) + get-by-id
	// with payload/before for diff display.
	masterDataChangeService := service.NewMasterDataChangeService(masterDataChangeRepo, approvalOrchestrator, auditWriter)
	adminMasterDataChangeHandler := handler.NewAdminMasterDataChangeHandler(masterDataChangeService)
	// masterDataAdmin is the single route guard for every master-data admin
	// mount below (plan.md T3.6): authenticated + ADMIN/ADMIN_PARAM. The same
	// role set is re-checked in MasterDataChangeService.Submit for every write,
	// so a mount that forgets this group still can't stage a change.
	masterDataAdmin := r.With(
		custommw.RequireAuth(tokenService),
		custommw.RequireRoles("ADMIN", "ADMIN_PARAM"),
	)

	// ADMIN/ADMIN_PARAM-only: vendor CRUD (Admin User & Vendor Management spec,
	// retrofitted onto maker-checker by plan.md T4.1): reads are direct, but
	// create/update/disable/enable stage a change via masterDataChangeService and
	// return 202; VendorApplier applies it once approved. Mounted before the
	// /vendors/{vendorID}/... sub-resources below, as before.
	// ponytail: swap dbPool for the dbRead pool on List/Count when
	// DATABASE_REPLICA_URL wiring lands (same TODO convention as above).
	vendorAdminRepo := repository.NewVendorAdminRepository(dbPool)
	vendorAdminService := service.NewVendorAdminService(vendorAdminRepo, masterDataChangeService)
	adminVendorHandler := handler.NewAdminVendorHandler(vendorAdminService)
	masterDataAdmin.Mount("/api/v1/admin/vendors", adminVendorHandler.Routes())

	// ADMIN/ADMIN_PARAM-only: ATM master-data CRUD (Admin ATM Management spec,
	// retrofitted onto maker-checker by plan.md T4.2): reads are direct, but
	// create/update/disable/enable stage a change via masterDataChangeService and
	// return 202; ATMApplier applies it once approved. The existing
	// /api/v1/atm-portal mount stays untouched (read-only monitoring). Mounted
	// before /atms/{atmID}/assignments below, as before.
	// ponytail: swap dbPool for the dbRead pool on List/Get/ListLocations when
	// DATABASE_REPLICA_URL wiring lands (same TODO convention as above).
	atmAdminRepo := repository.NewATMAdminRepository(dbPool)
	atmAdminService := service.NewATMAdminService(atmAdminRepo, masterDataChangeService)
	adminATMHandler := handler.NewAdminATMHandler(atmAdminService)
	masterDataAdmin.Mount("/api/v1/admin/atms", adminATMHandler.Routes())

	masterDataAdmin.Mount("/api/v1/admin/master-data/changes", adminMasterDataChangeHandler.Routes())

	// CSV export of master data (plan.md T5.1): streamed in keyset pages from the
	// read-replica pool (primary when no replica is configured); read-only, no
	// approval involved. Same masterDataAdmin guard, re-checked in the exporter.
	masterDataExportRepo := repository.NewMasterDataExportRepository(dbReadPool)
	masterDataExporter := service.NewMasterDataExporter(masterDataExportRepo)
	adminMasterDataExportHandler := handler.NewAdminMasterDataExportHandler(masterDataExporter)
	masterDataAdmin.Mount("/api/v1/admin/master-data/export", adminMasterDataExportHandler.Routes())

	// ADMIN/ADMIN_PARAM-only: vendor branch CRUD (plan.md T3.1) -- staged via
	// masterDataChangeService.Submit (maker-checker-native, D1), applied by
	// VendorBranchApplier once approved via /api/v1/approvals above.
	vendorBranchAdminRepo := repository.NewVendorBranchAdminRepository(dbPool)
	vendorBranchAdminService := service.NewVendorBranchAdminService(vendorBranchAdminRepo, masterDataChangeService)
	adminVendorBranchHandler := handler.NewAdminVendorBranchHandler(vendorBranchAdminService)
	masterDataAdmin.Mount("/api/v1/admin/vendors/{vendorID}/branches", adminVendorBranchHandler.Routes())

	// ADMIN/ADMIN_PARAM-only: vendor vault CRUD (plan.md T3.2), same
	// maker-checker-native flow as branches above.
	vendorVaultAdminRepo := repository.NewVendorVaultAdminRepository(dbPool)
	vendorVaultAdminService := service.NewVendorVaultAdminService(vendorVaultAdminRepo, masterDataChangeService)
	adminVendorVaultHandler := handler.NewAdminVendorVaultHandler(vendorVaultAdminService)
	masterDataAdmin.Mount("/api/v1/admin/vendors/{vendorID}/vaults", adminVendorVaultHandler.Routes())

	// ADMIN/ADMIN_PARAM-only: vendor PIC CRUD (plan.md T3.3), same
	// maker-checker-native flow; list also returns non-blocking warnings.
	vendorPicAdminRepo := repository.NewVendorPicAdminRepository(dbPool)
	vendorPicAdminService := service.NewVendorPicAdminService(vendorPicAdminRepo, masterDataChangeService)
	adminVendorPicHandler := handler.NewAdminVendorPicHandler(vendorPicAdminService)
	masterDataAdmin.Mount("/api/v1/admin/vendors/{vendorID}/pics", adminVendorPicHandler.Routes())

	// ADMIN/ADMIN_PARAM-only: vendor package CRUD (plan.md T3.4), same
	// maker-checker-native flow; price is a decimal string (numeric(20,2)).
	vendorPackageAdminRepo := repository.NewVendorPackageAdminRepository(dbPool)
	vendorPackageAdminService := service.NewVendorPackageAdminService(vendorPackageAdminRepo, masterDataChangeService)
	adminVendorPackageHandler := handler.NewAdminVendorPackageHandler(vendorPackageAdminService)
	masterDataAdmin.Mount("/api/v1/admin/vendors/{vendorID}/packages", adminVendorPackageHandler.Routes())

	// ADMIN/ADMIN_PARAM-only: ATM assignment (kelolaan) CRUD (plan.md T3.5),
	// same maker-checker-native flow; overlap is a clean 409 at submit time and
	// again at apply time (exclusion constraint, see ATMAssignmentApplier).
	// Mounted beside /api/v1/admin/atms above -- chi picks the more specific
	// {atmID}/assignments pattern (route-mount cleanup is T3.6).
	atmAssignmentAdminRepo := repository.NewATMAssignmentAdminRepository(dbPool)
	atmAssignmentAdminService := service.NewATMAssignmentAdminService(atmAssignmentAdminRepo, masterDataChangeService)
	adminATMAssignmentHandler := handler.NewAdminATMAssignmentHandler(atmAssignmentAdminService)
	masterDataAdmin.Mount("/api/v1/admin/atms/{atmID}/assignments", adminATMAssignmentHandler.Routes())

	// CSV import of master data (plan.md T5.3 dry-run, T5.4 confirm). It validates
	// and stages a write flow, so it reads the PRIMARY (dbPool), never the
	// replica. Confirm builds the per-entity admin services over a collector
	// (the same validation/payload/"before" code as the single-record
	// endpoints) and stages the result as ONE batch under one approval.
	masterDataImporter := service.NewMasterDataImporter(repository.NewMasterDataExportRepository(dbPool)).WithConfirm(service.ImportConfirmDeps{
		Batches:      repository.NewMasterDataImportBatchRepository(dbPool, dbPool),
		Orchestrator: approvalOrchestrator,
		Audit:        auditWriter,
		Services: func(sub service.ImportSubmitter) service.ImportServices {
			return service.ImportServices{
				Vendors:     service.NewVendorAdminService(vendorAdminRepo, sub),
				Branches:    service.NewVendorBranchAdminService(vendorBranchAdminRepo, sub),
				Vaults:      service.NewVendorVaultAdminService(vendorVaultAdminRepo, sub),
				PICs:        service.NewVendorPicAdminService(vendorPicAdminRepo, sub),
				ATMs:        service.NewATMAdminService(atmAdminRepo, sub),
				Assignments: service.NewATMAssignmentAdminService(atmAssignmentAdminRepo, sub),
			}
		},
	})
	masterDataAdmin.Mount("/api/v1/admin/master-data/import", handler.NewAdminMasterDataImportHandler(masterDataImporter).WithRateLimit(redisClient).Routes())

	// Admin-only: hierarchy/delegation/leave management (RBAC-Setup Task 8),
	// plus the RBAC settings menu's read-only list views + policy create/edit
	// (RBAC Settings Menu spec). Widened to include APPACCESS alongside
	// ADMIN/ADMIN_PARAM (Requirement 3).
	//
	// ponytail: swap dbPool for the dbRead pool when DATABASE_REPLICA_URL
	// wiring lands (same TODO convention as the audit-log repository above).
	adminApprovalHandler := handler.NewAdminApprovalHandler(approvalRepo, auditWriter)
	rbacReadRepo := repository.NewRbacReadRepository(dbPool)
	rbacPolicyStore := repository.NewApprovalPolicyStore(dbPool)
	rbacListHandler := handler.NewRbacListHandler(rbacReadRepo, rbacPolicyStore, auditWriter)
	// AdminApprovalHandler.Routes() and RbacListHandler.Routes() cannot both be
	// Mount()-ed at this prefix (chi panics: two mounts can't share one exact
	// path). Register each handler's routes directly on the shared router
	// instead — chi resolves the static "/users/hierarchy" ahead of the
	// "/users/{id}/hierarchy" param route, so both sets coexist correctly.
	r.With(
		custommw.RequireAuth(tokenService),
		custommw.RequireRoles("ADMIN", "ADMIN_PARAM", "APPACCESS"),
	).Route("/api/v1/admin/approval", func(ar chi.Router) {
		ar.Put("/users/{id}/hierarchy", adminApprovalHandler.SetHierarchy)
		ar.Post("/delegations", adminApprovalHandler.CreateDelegation)
		ar.Delete("/delegations/{id}", adminApprovalHandler.RevokeDelegation)
		ar.Post("/leaves", adminApprovalHandler.CreateLeave)

		ar.Get("/users/hierarchy", rbacListHandler.ListUserHierarchy)
		ar.Get("/delegations", rbacListHandler.ListDelegations)
		ar.Get("/leaves", rbacListHandler.ListLeaves)
		ar.Get("/policies", rbacListHandler.ListPolicies)
		ar.Post("/policies", rbacListHandler.CreatePolicy)
		ar.Put("/policies/{id}", rbacListHandler.UpdatePolicy)
	})

	// Create and mount the Vendor Request handler (DMAA forecast -> CIT
	// vendor replenishment order, self-contained maker-checker state
	// machine — request-replenish-to-vendor spec). Mounted behind
	// RequireAuth only, matching the approval handler above: the per-
	// endpoint role subset (maker/checker/viewer) is applied inside
	// Routes() itself since it differs per route, and actor-level
	// authorization (creator/checker/four-eyes) is enforced in the service.
	vendorRequestService := service.NewVendorRequestService(dbPool)
	vendorRequestHandler := handler.NewVendorRequestHandler(vendorRequestService)
	r.With(custommw.RequireAuth(tokenService)).Mount("/api/v1/vendor-requests", vendorRequestHandler.Routes())

	// Create and mount the Audit Log Viewer handler (read-only, admin-only).
	// ponytail: swap dbPool for the dbRead pool when DATABASE_REPLICA_URL wiring lands
	auditLogRepo := repository.NewAuditLogRepository(dbPool)
	auditLogService := service.NewAuditLogReadService(auditLogRepo)
	auditLogHandler := handler.NewAuditLogHandler(auditLogService)
	r.With(
		custommw.RequireAuth(tokenService),
		custommw.RequireRoles("ADMIN", "ADMIN_PARAM"),
	).Mount("/api/v1/audit-logs", auditLogHandler.Routes())

	// Start HTTP server
	addr := ":" + cfg.Port
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Info("starting HTTP server", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	<-done
	slog.Info("shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped gracefully")
}

// getenvDefault reads an environment variable, returning def when unset/empty.
func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
