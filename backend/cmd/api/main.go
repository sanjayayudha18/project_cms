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

	// APPACCESS-only: set a target user's initial password, forcing a
	// change on their next login (Auth-Local-Lifecycle Task 6), plus the
	// full admin user CRUD (Admin User & Vendor Management spec): list/get/
	// create/update (UserAdminService) and disable/enable (the existing
	// DeactivateUserService, previously unwired).
	setInitialPasswordService := auth.NewSetInitialPasswordService(userRepo, auditWriter)
	userAdminRepo := repository.NewUserAdminRepository(dbPool)
	userAdminService := auth.NewUserAdminService(userAdminRepo, auditWriter)
	deactivateService := auth.NewDeactivateUserService(userRepo, auditWriter)
	adminUserHandler := handler.NewAdminUserHandler(setInitialPasswordService, userAdminService, deactivateService)
	r.With(
		custommw.RequireAuth(tokenService),
		custommw.RequireRoles("APPACCESS"),
	).Mount("/api/v1/admin/users", adminUserHandler.Routes())

	// ADMIN/ADMIN_PARAM-only: vendor master-data CRUD (Admin User & Vendor
	// Management spec) — no vendor admin endpoints existed before this.
	// ponytail: swap dbPool for the dbRead pool on List/Count when
	// DATABASE_REPLICA_URL wiring lands (same TODO convention as above).
	vendorAdminRepo := repository.NewVendorAdminRepository(dbPool)
	vendorAdminService := service.NewVendorAdminService(vendorAdminRepo, auditWriter)
	adminVendorHandler := handler.NewAdminVendorHandler(vendorAdminService)
	r.With(
		custommw.RequireAuth(tokenService),
		custommw.RequireRoles("ADMIN", "ADMIN_PARAM"),
	).Mount("/api/v1/admin/vendors", adminVendorHandler.Routes())

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

	// Create and mount the approval handler. Mounted behind RequireAuth only
	// (no RequireRoles): submit must be reachable by any authenticated maker
	// regardless of role, since role != approval hierarchy (RBAC-Setup). The
	// real per-request authorization (maker != checker, actor == effective
	// approver) is enforced inside the orchestrator itself, not at the route.
	// (auditWriter created earlier, shared with the change-password service.)
	approvalRepo := approval.NewRepository(dbPool)
	approvalOrchestrator := approval.NewOrchestrator(approvalRepo, approvalRepo, approvalRepo, auditWriter, nil)
	approvalHandler := handler.NewApprovalHandler(approvalOrchestrator, approvalRepo)
	r.With(custommw.RequireAuth(tokenService)).Mount("/api/v1/approvals", approvalHandler.Routes())

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
