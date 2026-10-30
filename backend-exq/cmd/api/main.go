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

	"github.com/cimb-niaga/cms/backend-exq/internal/audit"
	"github.com/cimb-niaga/cms/backend-exq/internal/exportjob"
	"github.com/cimb-niaga/cms/backend-exq/internal/handler"
	"github.com/cimb-niaga/cms/pkg/auth"
	"github.com/cimb-niaga/cms/pkg/config"
	"github.com/cimb-niaga/cms/pkg/health"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
	"github.com/cimb-niaga/cms/pkg/reports"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg, err := config.Load("8081")
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Invalid worker env = refuse to start (Req 10.7, 11.7).
	workerCfg, err := exportjob.LoadWorkerConfig()
	if err != nil {
		slog.Error("invalid export worker config", "error", err)
		os.Exit(1)
	}

	// DSR late report start date (K5: same value as backend and RETRY_DSR_LATE_START_DATE).
	// Invalid = refuse to start.
	dsrLateStart, err := reports.ParseDsrLateStartDate(os.Getenv("DSR_LATE_START_DATE"))
	if err != nil {
		slog.Error("invalid DSR_LATE_START_DATE", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// PostgreSQL primary pool
	dbPool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to create database pool", "error", err)
		os.Exit(1)
	}
	defer dbPool.Close()

	// Read-replica pool; unset = reuse the primary (not closed twice).
	dbReadPool := dbPool
	if cfg.DatabaseReplicaURL != "" {
		replicaPool, err := pgxpool.New(ctx, cfg.DatabaseReplicaURL)
		if err != nil {
			slog.Error("failed to create read-replica pool", "error", err)
			os.Exit(1)
		}
		defer replicaPool.Close()
		dbReadPool = replicaPool
	} else {
		slog.Warn("DATABASE_REPLICA_URL not set; reads use the primary")
	}

	// Redis
	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		slog.Error("failed to parse Redis URL", "error", err)
		os.Exit(1)
	}
	redisClient := redis.NewClient(redisOpts)
	defer func() { _ = redisClient.Close() }()

	// Token service (validation only — CIT does not issue tokens)
	tokenBlacklist := auth.NewRedisTokenBlacklist(redisClient)
	tokenService := auth.NewTokenService(auth.TokenConfig{
		SecretKey:          cfg.JWTSecret,
		AccessTokenExpiry:  cfg.AccessTokenExpiry,
		SessionMaxLifetime: cfg.SessionMaxLifetime,
	}, tokenBlacklist)

	r := chi.NewRouter()
	// Client IP first: X-Forwarded-For only from TRUSTED_PROXIES (QC F13).
	r.Use(custommw.TrustedRealIP(cfg.TrustedProxies))
	r.Use(middleware.RequestID)
	r.Use(custommw.RequestLogger(slog.Default())) // BE-04: JSON slog, msg "http_request"
	r.Use(middleware.Recoverer)
	r.Use(custommw.BodyLimit(1<<20, nil)) // BE-08: no upload endpoints here

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Readiness (BE-04): pings the DB and Redis; 503 when any fails.
	readyChecks := map[string]func(context.Context) error{
		"db":    dbPool.Ping,
		"redis": func(ctx context.Context) error { return redisClient.Ping(ctx).Err() },
	}
	if dbReadPool != dbPool {
		readyChecks["db_replica"] = dbReadPool.Ping
	}
	r.Get("/ready", health.Ready(2*time.Second, readyChecks))

	// Export job pipeline. Worker first: its Wake is the service's wake hook.
	worker := exportjob.NewWorker(dbPool, dbReadPool, exportjob.NewProducers(dsrLateStart), workerCfg)
	svc := exportjob.NewService(dbPool, audit.NewWriter(), worker.Wake)
	janitor := exportjob.NewJanitor(dbPool, workerCfg.JanitorInterval)
	mountExports(r, custommw.RequireAuth(tokenService), handler.NewExportsHandler(svc))

	workerCtx, cancelWorkers := context.WithCancel(context.Background())
	defer cancelWorkers()
	go worker.Run(workerCtx)
	go janitor.Run(workerCtx)

	addr := ":" + cfg.Port
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Info("starting backend-exq", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	<-done
	slog.Info("shutting down CIT backend...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Order (design): stop HTTP, stop claiming/janitor, then let running jobs
	// finish (<=10 s) before the pools close; unfinished jobs go back to queued.
	srvErr := srv.Shutdown(shutdownCtx)
	slog.Info("http server stopped", "error", srvErr)
	cancelWorkers()
	slog.Info("export workers: waiting for running jobs", "grace", workerShutdownGrace)
	worker.Wait(workerShutdownGrace)
	slog.Info("export workers stopped")
	if srvErr != nil {
		slog.Error("server shutdown error", "error", srvErr)
		os.Exit(1)
	}

	slog.Info("CIT backend stopped gracefully")
}

const workerShutdownGrace = 10 * time.Second

// mountExports registers /api/v1/exports. Everything except the file download
// shares a 30 s timeout; the download gets the 5 min lease (chi routes the
// explicit /jobs/{jobID}/file ahead of the "/*" catch-all of the mounted subrouter).
func mountExports(r chi.Router, authMW func(http.Handler) http.Handler, h *handler.ExportsHandler) {
	r.Route("/api/v1/exports", func(r chi.Router) {
		r.Use(authMW)
		r.With(middleware.Timeout(exportjob.DownloadLease)).Get("/jobs/{jobID}/file", h.Download)
		r.With(middleware.Timeout(30*time.Second)).Mount("/", h.Routes())
	})
}
