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

	"github.com/cimb-niaga/cms/backend/internal/auth"
	"github.com/cimb-niaga/cms/backend/internal/config"
	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/handler"
	custommw "github.com/cimb-niaga/cms/backend/internal/middleware"
	"github.com/cimb-niaga/cms/backend/internal/repository"
	"github.com/cimb-niaga/cms/backend/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load configuration from environment
	cfg, err := config.Load()
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
	defer redisClient.Close()

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
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Initialize auth dependencies
	tokenBlacklist := auth.NewRedisTokenBlacklist(redisClient)
	tokenService := auth.NewTokenService(auth.TokenConfig{
		SecretKey:          cfg.JWTSecret,
		AccessTokenExpiry:  cfg.AccessTokenExpiry,
		RefreshTokenExpiry: cfg.RefreshTokenExpiry,
	}, tokenBlacklist)

	userRepo := repository.NewAuthRepository(dbPool)

	rateLimiter := custommw.NewRateLimiter(redisClient, custommw.RateLimitConfig{
		MaxPerUsername: cfg.RateLimitUsername,
		MaxPerIP:       cfg.RateLimitIP,
		Window:         cfg.RateLimitWindow,
	})

	// Create auth service with local provider
	localProvider := auth.NewLocalProvider(userRepo)
	authService := auth.NewService(
		[]auth.Provider{localProvider},
		tokenService,
		userRepo,
		rateLimiter,
	)

	// Create and mount auth handler
	authHandler := handler.NewAuthHandler(authService, tokenService, userRepo, rateLimiter)
	r.Mount("/api/v1/auth", authHandler.Routes())

	// Create and mount ATM Portal handler, protected by RequireAuth
	atmPortalService := service.NewAtmPortalService(db.New(dbPool))
	atmPortalHandler := handler.NewAtmPortalHandler(atmPortalService)
	r.With(custommw.RequireAuth(tokenService)).Mount("/api/v1/atm-portal", atmPortalHandler.Routes())

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
