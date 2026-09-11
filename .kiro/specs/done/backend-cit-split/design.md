# Design Document: Backend CIT Split

## Overview

Restructure the CMS backend into a Go workspace with three modules: a shared `pkg/` module containing cross-cutting infrastructure code, the existing `backend/` (ATM) module refactored to import from `pkg/`, and a new `backend-cit/` skeleton module for CIT domain development. Both backends share the same PostgreSQL database, Redis instance, and JWT authentication.

---

## Architecture

### Module Topology

```
CMS2/                          (repository root)
├── go.work                    (Go workspace: links all modules)
├── go.work.sum
├── pkg/                       (shared infrastructure module)
│   ├── go.mod                 (github.com/cimb-niaga/cms/pkg)
│   ├── auth/                  (token service, blacklist, claims, types)
│   ├── middleware/            (RequireAuth, RequireRoles, rate limiter)
│   ├── config/                (env-based config loader)
│   └── response/              (JSON response envelope)
├── backend/                   (ATM backend module - existing, refactored)
│   ├── go.mod                 (github.com/cimb-niaga/cms/backend)
│   ├── cmd/api/main.go
│   ├── internal/              (ATM-specific: handlers, services, repos)
│   ├── migrations/            (sole owner of all DB migrations)
│   ├── queries/               (ATM sqlc queries)
│   └── sqlc.yaml
└── backend-cit/               (CIT backend module - new skeleton)
    ├── go.mod                 (github.com/cimb-niaga/cms/backend-cit)
    ├── cmd/api/main.go
    ├── internal/              (CIT domain placeholders)
    ├── queries/               (CIT sqlc queries - future)
    └── sqlc.yaml
```

### Dependency Direction (Acyclic)

```
backend/ ──────→ pkg/
backend-cit/ ──→ pkg/
```

Neither `backend/` nor `backend-cit/` depends on the other. `pkg/` depends on no internal module. This ensures the Go compiler rejects circular dependencies at build time.

### Shared Infrastructure via `pkg/`

The extraction boundary is drawn at code that both backends need to function identically: authentication, authorization middleware, configuration loading, and response formatting. Business logic (handlers, services, domain types) stays in each backend's `internal/` package.

---

## Components

### 1. Go Workspace (`go.work`)

```go
go 1.25.0

use (
    ./backend
    ./backend-cit
    ./pkg
)
```

The workspace enables local module resolution during development. In CI, `go build ./...` from root validates all modules compile together.

### 2. Shared Package (`pkg/`)

#### `pkg/auth`

Extracted from `backend/internal/auth`. Contains all authentication primitives that both services need.

**Exported types and functions:**

```go
package auth

// --- Types ---
type AuthIdentity struct {
    UserID     int64
    Username   string
    Role       string
    IsKaryawan bool
    VendorID   *int64
}

type TokenConfig struct {
    SecretKey          []byte
    AccessTokenExpiry  time.Duration
    RefreshTokenExpiry time.Duration
}

type AccessTokenClaims struct {
    UserID     int64  `json:"id"`
    Username   string `json:"username"`
    Role       string `json:"role"`
    IsKaryawan bool   `json:"is_karyawan"`
    VendorID   *int64 `json:"vendor_id,omitempty"`
    jwt.RegisteredClaims
}

type RefreshTokenClaims struct {
    UserID int64 `json:"id"`
    jwt.RegisteredClaims
}

// --- Interfaces ---
type TokenBlacklist interface {
    Add(ctx context.Context, jti string, ttl time.Duration) error
    IsBlacklisted(ctx context.Context, jti string) (bool, error)
}

type UserRepository interface {
    FindByUsername(ctx context.Context, username string) (*UserRecord, error)
    UpdateLastLogin(ctx context.Context, userID int64) error
    GetUserProfile(ctx context.Context, userID int64) (*UserRecord, error)
}

type RateLimiter interface {
    Check(ctx context.Context, username, ip string) error
    IncrementFailed(ctx context.Context, username, ip string) error
    ResetUsername(ctx context.Context, username string) error
}

type Provider interface {
    Authenticate(ctx context.Context, username, password string) (*AuthIdentity, error)
    Supports(authSource string) bool
}

// --- Concrete implementations ---
type TokenService struct { ... }
func NewTokenService(config TokenConfig, blacklist TokenBlacklist) *TokenService
func (ts *TokenService) GenerateTokenPair(identity *AuthIdentity) (accessToken, refreshToken string, err error)
func (ts *TokenService) ValidateAccessToken(tokenStr string) (*AccessTokenClaims, error)
func (ts *TokenService) ValidateRefreshToken(ctx context.Context, tokenStr string) (*RefreshTokenClaims, error)
func (ts *TokenService) BlacklistRefreshToken(ctx context.Context, refreshTokenStr string) error

type RedisTokenBlacklist struct { ... }
func NewRedisTokenBlacklist(client *redis.Client) *RedisTokenBlacklist

// --- Errors ---
var (
    ErrInvalidCredentials    error
    ErrAccountInactive       error
    ErrPortalMismatch        error
    ErrUnsupportedAuthSource error
    ErrLDAPNotConfigured     error
    ErrRateLimited           error
    ErrServiceUnavailable    error
    ErrTokenExpired          error
    ErrTokenInvalid          error
)

type ValidationError struct { Field, Message string }
type RateLimitError struct { RetryAfter int }
```

**What stays in `backend/internal/auth`:** The `Service` struct (login orchestration), `LocalProvider`, `LoginRequest`/`LoginResponse`/`UserProfile` types — these are ATM-backend-specific business logic. The CIT backend does not issue tokens; it only validates them.

#### `pkg/middleware`

Extracted from `backend/internal/middleware`.

```go
package middleware

// AuthContext is injected into request context after token validation.
type AuthContext struct {
    UserID     int64
    Username   string
    Role       string
    IsKaryawan bool
    VendorID   *int64
}

// RequireAuth validates the Bearer token and injects AuthContext.
func RequireAuth(tokenService *auth.TokenService) func(http.Handler) http.Handler

// RequireRoles checks if the authenticated user's role is in allowedRoles.
func RequireRoles(allowedRoles ...string) func(http.Handler) http.Handler

// GetAuthContext extracts AuthContext from request context.
func GetAuthContext(ctx context.Context) (*AuthContext, bool)

// RateLimitConfig holds rate limiting configuration.
type RateLimitConfig struct {
    MaxPerUsername int
    MaxPerIP       int
    Window         time.Duration
}

// RateLimiter enforces login rate limits using Redis.
type RateLimiter struct { ... }
func NewRateLimiter(redisClient *redis.Client, config RateLimitConfig) *RateLimiter
```

#### `pkg/config`

Extracted from `backend/internal/config`, extended to support both backends.

```go
package config

// Config holds shared application configuration.
type Config struct {
    // JWT
    JWTSecret          []byte
    AccessTokenExpiry  time.Duration
    RefreshTokenExpiry time.Duration

    // Database
    DatabaseURL        string
    DatabaseReplicaURL string // new: read replica support

    // Redis
    RedisURL string

    // Rate Limiting
    RateLimitUsername int
    RateLimitIP      int
    RateLimitWindow  time.Duration

    // Server
    Port string

    // LDAP (optional, used by ATM backend)
    LDAPURL          string
    LDAPBaseDN       string
    LDAPBindDN       string
    LDAPBindPassword string
}

// Load reads configuration from environment variables.
// DefaultPort determines the fallback port when PORT is unset.
func Load(defaultPort string) (*Config, error)
```

The `Load` function accepts a `defaultPort` parameter so ATM backend defaults to `"8080"` and CIT backend defaults to `"8081"`.

#### `pkg/response`

Provides the consistent JSON response envelope.

```go
package response

// Envelope is the standard API response wrapper.
type Envelope struct {
    Success bool        `json:"success"`
    Data    interface{} `json:"data,omitempty"`
    Error   *ErrorBody  `json:"error,omitempty"`
    Meta    *Meta       `json:"meta,omitempty"`
}

type ErrorBody struct {
    Code    string       `json:"code"`
    Message string       `json:"message"`
    Details []FieldError `json:"details,omitempty"`
}

type FieldError struct {
    Field   string `json:"field"`
    Message string `json:"message"`
}

type Meta struct {
    Page       int `json:"page,omitempty"`
    PerPage    int `json:"per_page,omitempty"`
    TotalRows  int `json:"total_rows,omitempty"`
    TotalPages int `json:"total_pages,omitempty"`
}

// WriteSuccess writes a 200 success response.
func WriteSuccess(w http.ResponseWriter, data interface{})

// WriteCreated writes a 201 success response.
func WriteCreated(w http.ResponseWriter, data interface{})

// WriteError writes an error response with the given HTTP status.
func WriteError(w http.ResponseWriter, status int, code, message string)

// WriteValidationError writes a 422 response with field errors.
func WriteValidationError(w http.ResponseWriter, details []FieldError)
```

### 3. ATM Backend (`backend/`) — Refactored

After extraction, `backend/internal/` retains:

| Package | Contents |
|---------|----------|
| `internal/auth` | `Service` (login flow orchestrator), `LocalProvider`, `LoginRequest`, `LoginResponse`, `UserProfile` — imports types/interfaces from `pkg/auth` |
| `internal/db` | pgx pool setup (unchanged) |
| `internal/handler` | ATM portal, auth, DMAA forecast handlers — uses `pkg/middleware` and `pkg/response` |
| `internal/repository` | `AuthRepository` implements `pkg/auth.UserRepository`, plus ATM-specific repos |
| `internal/service` | ATM business logic (ATM portal, DMAA forecast) |

**Import path changes in `backend/`:**

```go
// Before:
import "github.com/cimb-niaga/cms/backend/internal/auth"
import "github.com/cimb-niaga/cms/backend/internal/middleware"
import "github.com/cimb-niaga/cms/backend/internal/config"

// After:
import "github.com/cimb-niaga/cms/pkg/auth"
import "github.com/cimb-niaga/cms/pkg/middleware"
import "github.com/cimb-niaga/cms/pkg/config"
import "github.com/cimb-niaga/cms/pkg/response"

// ATM-specific auth logic stays:
import authsvc "github.com/cimb-niaga/cms/backend/internal/auth"
```

The `backend/internal/auth` package is renamed conceptually to hold only the login service logic. It imports `pkg/auth` for shared types (`AuthIdentity`, `TokenService`, etc.) and implements ATM-specific flows.

### 4. CIT Backend (`backend-cit/`) — Skeleton

```
backend-cit/
├── cmd/api/main.go              (functional: Chi + health + graceful shutdown)
├── internal/
│   ├── cit/                     (placeholder: package cit)
│   │   └── cit.go
│   ├── journal/                 (placeholder: package journal)
│   │   └── journal.go
│   ├── dsr/                     (placeholder: package dsr)
│   │   └── dsr.go
│   ├── reconciliation/          (placeholder: package reconciliation)
│   │   └── reconciliation.go
│   ├── integration/             (placeholder: package integration)
│   │   └── integration.go
│   ├── handler/                 (placeholder: package handler)
│   │   └── handler.go
│   ├── service/                 (placeholder: package service)
│   │   └── service.go
│   └── repository/              (placeholder: package repository)
│       └── repository.go
├── queries/                     (empty directory for future sqlc)
├── sqlc.yaml                    (CIT-specific sqlc config, points to own queries/)
├── Dockerfile                   (multi-stage build)
└── .env.example
```

**`backend-cit/cmd/api/main.go` implementation:**

```go
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

    "github.com/cimb-niaga/cms/pkg/auth"
    "github.com/cimb-niaga/cms/pkg/config"
    custommw "github.com/cimb-niaga/cms/pkg/middleware"
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

    ctx := context.Background()

    // PostgreSQL primary pool
    dbPool, err := pgxpool.New(ctx, cfg.DatabaseURL)
    if err != nil {
        slog.Error("failed to create database pool", "error", err)
        os.Exit(1)
    }
    defer dbPool.Close()

    // Redis
    redisOpts, err := redis.ParseURL(cfg.RedisURL)
    if err != nil {
        slog.Error("failed to parse Redis URL", "error", err)
        os.Exit(1)
    }
    redisClient := redis.NewClient(redisOpts)
    defer redisClient.Close()

    // Token service (validation only — CIT does not issue tokens)
    tokenBlacklist := auth.NewRedisTokenBlacklist(redisClient)
    tokenService := auth.NewTokenService(auth.TokenConfig{
        SecretKey:          cfg.JWTSecret,
        AccessTokenExpiry:  cfg.AccessTokenExpiry,
        RefreshTokenExpiry: cfg.RefreshTokenExpiry,
    }, tokenBlacklist)

    r := chi.NewRouter()
    r.Use(middleware.RequestID)
    r.Use(middleware.RealIP)
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)

    r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(`{"status":"ok"}`))
    })

    // Protected route group (placeholder for future CIT endpoints)
    r.Group(func(r chi.Router) {
        r.Use(custommw.RequireAuth(tokenService))
        // CIT routes will be mounted here
    })

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
        slog.Info("starting CIT backend", "addr", addr)
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            slog.Error("HTTP server error", "error", err)
            os.Exit(1)
        }
    }()

    <-done
    slog.Info("shutting down CIT backend...")

    shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()

    if err := srv.Shutdown(shutdownCtx); err != nil {
        slog.Error("server shutdown error", "error", err)
        os.Exit(1)
    }

    slog.Info("CIT backend stopped gracefully")
}
```

### 5. Docker Compose (Updated)

```yaml
name: CMS-Backend

services:
  backend:
    build:
      context: .
      dockerfile: backend/Dockerfile
    ports:
      - "8080:8080"
    env_file:
      - .env
    depends_on:
      redis:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/health"]
      interval: 10s
      timeout: 5s
      retries: 3

  backend-cit:
    build:
      context: .
      dockerfile: backend-cit/Dockerfile
    ports:
      - "8081:8081"
    env_file:
      - .env
    depends_on:
      redis:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8081/health"]
      interval: 10s
      timeout: 5s
      retries: 3

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 3

networks:
  cms-net:
    driver: bridge
```

Both services share the same `.env` file (same `DATABASE_URL`, `JWT_SECRET`, `REDIS_URL`). The CIT backend reads `PORT=8081` from its own default.

### 6. Dockerfile for CIT Backend

```dockerfile
# -- Build stage --
FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.work go.work.sum ./
COPY pkg/ ./pkg/
COPY backend-cit/ ./backend-cit/
WORKDIR /src/backend-cit
RUN go build -o /app/backend-cit ./cmd/api

# -- Runtime stage --
FROM alpine:3.20
RUN apk add --no-cache wget
COPY --from=builder /app/backend-cit /usr/local/bin/backend-cit
ENTRYPOINT ["backend-cit"]
```

---

## Interfaces

### Module Dependency Graph

```
pkg/auth        → github.com/golang-jwt/jwt/v5, github.com/redis/go-redis/v9, github.com/google/uuid
pkg/middleware  → pkg/auth, github.com/go-chi/chi/v5, github.com/redis/go-redis/v9
pkg/config      → (stdlib only)
pkg/response    → (stdlib only)

backend/        → pkg/*, github.com/jackc/pgx/v5, github.com/go-chi/chi/v5, ...
backend-cit/    → pkg/*, github.com/jackc/pgx/v5, github.com/go-chi/chi/v5, ...
```

### Cross-Service Contract

Both backends do NOT communicate directly. They share state through:

1. **PostgreSQL** — same tables, migrations owned by ATM backend
2. **Redis** — same instance for token blacklist and rate limiting
3. **JWT tokens** — same secret, same claims structure via `pkg/auth`

No RPC, no message bus, no shared file system between services.

---

## Data Models

No new tables are introduced. The CIT backend accesses existing CIT-related tables (`cit_orders`, `cit_handover_evidences`, `cit_journals`, `cit_dsr_uploads`, `cit_reconciliation_results`) through its own sqlc-generated repository layer, but schema migrations remain in `backend/migrations/`.

### Shared Configuration Data Model

```go
// pkg/config.Config — shared between both backends
type Config struct {
    JWTSecret          []byte        // required, min 32 bytes
    AccessTokenExpiry  time.Duration // default: 15m
    RefreshTokenExpiry time.Duration // default: 7d
    DatabaseURL        string        // required: primary (write)
    DatabaseReplicaURL string        // optional: read replica
    RedisURL           string        // required
    RateLimitUsername  int           // default: 5
    RateLimitIP        int           // default: 20
    RateLimitWindow    time.Duration // default: 15m
    Port               string        // default: "8080" or "8081"
    LDAPURL            string        // optional
    LDAPBaseDN         string        // optional
    LDAPBindDN         string        // optional
    LDAPBindPassword   string        // optional
}
```

---

## Error Handling

### Shared Error Strategy (`pkg/auth` + `pkg/response`)

- Sentinel errors define known failure modes (e.g., `ErrTokenExpired`, `ErrTokenInvalid`)
- Structured `ValidationError` and `RateLimitError` carry contextual data
- `pkg/response` provides consistent JSON envelope for all error responses
- Both backends use the same error codes and message formats
- Middleware returns 401/403 directly via `pkg/response.WriteError`

### Fail-Closed Behavior

- If Redis is unavailable during token blacklist check → reject token (ErrServiceUnavailable)
- If Redis is unavailable during rate limit check → reject request (ErrServiceUnavailable)
- This behavior is encoded in `pkg/auth.TokenService` and `pkg/middleware.RateLimiter`

---

## Migration Strategy (Execution Order)

The restructuring must be done in a specific order to maintain a compilable codebase at each step:

1. **Create `pkg/` module** — copy shared code, define `go.mod`
2. **Create `go.work`** — link all three modules
3. **Update `backend/`** — change imports from `internal/` to `pkg/`, verify tests pass
4. **Remove extracted code from `backend/internal/`** — delete files now in `pkg/`
5. **Create `backend-cit/` skeleton** — module + main + placeholders
6. **Update Docker Compose** — add CIT service
7. **Verify full workspace builds** — `go build ./...` from root

Steps 1-2 can be done without breaking the existing backend (the workspace just adds new modules). Step 3-4 is the critical refactoring point where import paths change.

---

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system — essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: Token Generation/Validation Round-Trip

*For any* valid `AuthIdentity` (with non-zero UserID, non-empty Username, non-empty Role), generating a token pair using `TokenService.GenerateTokenPair` and then validating the access token using `TokenService.ValidateAccessToken` with the same secret key SHALL return `AccessTokenClaims` where UserID, Username, Role, IsKaryawan, and VendorID match the original identity.

**Validates: Requirements 2.3, 6.4**

### Property 2: RequireAuth Middleware Token Enforcement

*For any* HTTP request, if the Authorization header contains a valid Bearer token (generated by the shared `TokenService`), the `RequireAuth` middleware SHALL inject an `AuthContext` into the request context with claims matching the token, and pass the request to the next handler. *For any* request with a missing, malformed, expired, or tampered Authorization header, the middleware SHALL return HTTP 401 without calling the next handler.

**Validates: Requirements 2.4, 6.2**

### Property 3: RequireRoles Middleware Access Control

*For any* authenticated request with `AuthContext.Role = R` and a set of allowed roles `S`, if `R` is in `S` (case-insensitive), the `RequireRoles` middleware SHALL pass the request to the next handler. If `R` is not in `S`, the middleware SHALL return HTTP 403 without calling the next handler. If `S` is empty, any authenticated user SHALL be allowed.

**Validates: Requirements 2.4, 6.3**

### Property 4: Configuration Loading Correctness

*For any* set of environment variables where `JWT_SECRET` (>= 32 bytes), `DATABASE_URL`, and `REDIS_URL` are all non-empty, calling `config.Load(defaultPort)` SHALL return a non-nil `Config` with all fields correctly mapped from their respective environment variables. *For any* set where any required variable is missing or invalid (e.g., JWT_SECRET < 32 bytes), `config.Load` SHALL return an error. When `PORT` is unset, `Config.Port` SHALL equal the provided `defaultPort` argument.

**Validates: Requirements 2.5, 9.3, 9.4**

### Property 5: Response Envelope JSON Consistency

*For any* success payload passed to `response.WriteSuccess`, the JSON output SHALL always contain `{"success": true, "data": <payload>}`. *For any* error written via `response.WriteError(status, code, message)`, the JSON output SHALL always contain `{"success": false, "error": {"code": <code>, "message": <message>}}`. The HTTP Content-Type header SHALL always be `application/json` for both cases.

**Validates: Requirements 2.6, 7.5**

### Property 6: Token Blacklist Add/Check Consistency

*For any* JTI string and positive TTL duration, after calling `TokenBlacklist.Add(ctx, jti, ttl)` successfully, calling `TokenBlacklist.IsBlacklisted(ctx, jti)` within the TTL window SHALL return `(true, nil)`. *For any* JTI that was never added, `IsBlacklisted` SHALL return `(false, nil)`.

**Validates: Requirements 6.5**
