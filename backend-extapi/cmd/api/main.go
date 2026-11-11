// Command api is the backend-extapi server (external OAuth2 client-credentials
// API for user management, port 8082).
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/cimb-niaga/cms/backend-extapi/internal/config"
	"github.com/cimb-niaga/cms/backend-extapi/internal/extauth"
	"github.com/cimb-niaga/cms/backend-extapi/internal/extuser"
	"github.com/cimb-niaga/cms/backend-extapi/internal/extuser/pgstore"
	"github.com/cimb-niaga/cms/backend-extapi/internal/handler"
	"github.com/cimb-niaga/cms/backend-extapi/internal/repository"
)

const shutdownGrace = 15 * time.Second

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	if err := run(); err != nil {
		slog.Error("backend-extapi failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}

	ctx := context.Background()
	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("create database pool: %w", err)
	}
	defer db.Close()

	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return errors.New("REDIS_URL is invalid") // never echo the URL (may hold a password)
	}
	rdb := redis.NewClient(redisOpts)
	defer func() { _ = rdb.Close() }()

	tokens := extauth.NewTokenService([]byte(cfg.ExtJWTSecret), cfg.TokenIssuer, cfg.TokenTTL)

	h := handler.NewRouter(handler.Deps{
		TrustedProxies: cfg.TrustedProxies,
		Tokens:         tokens,
		// Primary pool only: a disabled or rotated client must be rejected at once.
		Clients:  repository.NewApiClientRepository(db),
		Failures: extauth.NewTokenFailures(rdb, cfg.TokenFailLimit, cfg.TokenFailWin),
		Rate:     extauth.NewAPIRate(rdb, cfg.APIRatePerMin),
		Users:    extuser.NewService(pgstore.NewTxStore(db), pgstore.NewReader(db)),
		ReadyChecks: map[string]func(context.Context) error{
			"db":    db.Ping,
			"redis": func(ctx context.Context) error { return rdb.Ping(ctx).Err() },
		},
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("starting backend-extapi", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	select {
	case err := <-serveErr:
		return err
	case <-sig:
	}
	slog.Info("shutting down backend-extapi")
	shutCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	slog.Info("backend-extapi stopped")
	return nil
}
