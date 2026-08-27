# Implementation Plan: Backend CIT Split

## Overview

Restructure the CMS backend into a Go workspace (`go.work`) with three modules: shared `pkg/` (auth, middleware, config, response), the existing `backend/` refactored to import from `pkg/`, and a new `backend-cit/` skeleton for CIT domain development. Execution follows the migration strategy from the design document to maintain a compilable codebase at each step.

## Tasks

- [ ] 1. Create shared `pkg/` module with extracted infrastructure code
  - [ ] 1.1 Create `pkg/go.mod` with module path `github.com/cimb-niaga/cms/pkg`, Go 1.25.0, and dependencies (chi/v5, jwt/v5, pgx/v5, go-redis/v9, x/crypto, google/uuid)
    - Initialize the module file with all required dependencies
    - _Requirements: 2.1, 2.7_

  - [ ] 1.2 Create `pkg/auth` package — token types, interfaces, and token service
    - Extract `AuthIdentity`, `TokenConfig`, `AccessTokenClaims`, `RefreshTokenClaims` types from `backend/internal/auth`
    - Extract `TokenBlacklist`, `UserRepository`, `RateLimiter`, `Provider` interfaces from `backend/internal/auth`
    - Extract `TokenService` (GenerateTokenPair, ValidateAccessToken, ValidateRefreshToken, BlacklistRefreshToken) from `backend/internal/auth/token_service.go`
    - Extract `RedisTokenBlacklist` from `backend/internal/auth/token_blacklist.go`
    - Extract sentinel errors (`ErrInvalidCredentials`, `ErrTokenExpired`, etc.) and error types (`ValidationError`, `RateLimitError`) from `backend/internal/auth/errors.go`
    - _Requirements: 2.3, 6.4, 6.5_

  - [ ] 1.3 Create `pkg/middleware` package — RequireAuth, RequireRoles, rate limiter
    - Extract `AuthContext` type and `GetAuthContext` helper
    - Extract `RequireAuth` middleware (token validation + context injection) from `backend/internal/middleware/rbac.go`
    - Extract `RequireRoles` middleware from `backend/internal/middleware/rbac.go`
    - Extract `RateLimitConfig` and `RateLimiter` struct from `backend/internal/middleware/rate_limiter.go`
    - Update imports to reference `pkg/auth` for token service and claims types
    - _Requirements: 2.4, 6.2, 6.3_

  - [ ] 1.4 Create `pkg/config` package — environment-based config loader
    - Extract and extend config struct from `backend/internal/config/config.go`
    - Add `DatabaseReplicaURL` field, add `defaultPort` parameter to `Load()` function
    - Support ATM default port "8080" and CIT default port "8081"
    - _Requirements: 2.5, 9.3, 9.4_

  - [ ] 1.5 Create `pkg/response` package — JSON response envelope
    - Create `Envelope`, `ErrorBody`, `FieldError`, `Meta` types
    - Implement `WriteSuccess`, `WriteCreated`, `WriteError`, `WriteValidationError` functions
    - Ensure Content-Type is always `application/json`
    - _Requirements: 2.6, 7.5_

  - [ ]* 1.6 Write property tests for `pkg/auth` token round-trip (Property 1)
    - **Property 1: Token Generation/Validation Round-Trip**
    - For any valid AuthIdentity, GenerateTokenPair → ValidateAccessToken returns matching claims
    - Use `pgregory.net/rapid` for property-based testing
    - **Validates: Requirements 2.3, 6.4**

  - [ ]* 1.7 Write property tests for `pkg/middleware` (Properties 2 & 3)
    - **Property 2: RequireAuth Middleware Token Enforcement**
    - Valid tokens → AuthContext injected; invalid/missing → HTTP 401
    - **Property 3: RequireRoles Middleware Access Control**
    - Role in allowed set → pass; role not in set → HTTP 403
    - **Validates: Requirements 2.4, 6.2, 6.3**

  - [ ]* 1.8 Write property tests for `pkg/config` (Property 4)
    - **Property 4: Configuration Loading Correctness**
    - Required vars present → valid Config; missing vars → error; unset PORT → defaultPort used
    - **Validates: Requirements 2.5, 9.3, 9.4**

  - [ ]* 1.9 Write property tests for `pkg/response` (Property 5)
    - **Property 5: Response Envelope JSON Consistency**
    - WriteSuccess → `{"success":true,"data":...}`; WriteError → `{"success":false,"error":{...}}`
    - **Validates: Requirements 2.6, 7.5**

- [ ] 2. Create Go workspace file
  - [ ] 2.1 Create `go.work` at repository root with Go 1.25.0 and `use` directives for `./backend`, `./backend-cit`, `./pkg`
    - Run `go work sync` to generate `go.work.sum`
    - _Requirements: 1.1, 1.2, 1.3, 1.4_

- [ ] 3. Checkpoint — Verify `pkg/` module compiles independently
  - Ensure `go build ./...` passes in `pkg/` directory, ask the user if questions arise.

- [ ] 4. Update ATM backend to import from shared `pkg/`
  - [ ] 4.1 Add `require github.com/cimb-niaga/cms/pkg` directive to `backend/go.mod`
    - _Requirements: 3.2_

  - [ ] 4.2 Update `backend/internal/auth` — retain only ATM-specific service logic
    - Keep: `Service` (login orchestrator), `LocalProvider`, `LoginRequest`/`LoginResponse`/`UserProfile` types
    - Change imports of token types, interfaces, errors, and token service to `github.com/cimb-niaga/cms/pkg/auth`
    - Remove duplicate type/interface declarations now in `pkg/auth`
    - _Requirements: 3.3, 3.4_

  - [ ] 4.3 Update `backend/internal/middleware` — remove extracted code, re-export or alias from `pkg/middleware`
    - Delete `RequireAuth`, `RequireRoles`, `AuthContext`, `GetAuthContext` code (now in `pkg/middleware`)
    - If the ATM backend's handler imports reference `internal/middleware`, create thin re-exports or update all handler imports to use `pkg/middleware` directly
    - Keep any ATM-backend-specific middleware if it exists
    - _Requirements: 3.3, 3.4_

  - [ ] 4.4 Update `backend/internal/config` — remove extracted code, import from `pkg/config`
    - Replace config struct and `Load` function with import from `pkg/config`
    - Update all files in `backend/` that import `internal/config` to use `pkg/config`
    - _Requirements: 3.3, 3.4_

  - [ ] 4.5 Update `backend/internal/handler` — switch to `pkg/response` and `pkg/middleware` imports
    - Update `auth_handler.go`, `atm_portal_handler.go`, `dmaa_forecast_handler.go`, `error_response.go`
    - Replace any inline response envelope code with `pkg/response` functions
    - _Requirements: 3.3_

  - [ ] 4.6 Update `backend/cmd/api/main.go` — import shared config and middleware from `pkg/`
    - Change config loading to `config.Load("8080")`
    - Update middleware wiring to use `pkg/middleware.RequireAuth` and `pkg/middleware.RequireRoles`
    - _Requirements: 3.3_

  - [ ]* 4.7 Run existing backend tests and fix any import-related failures
    - Execute `go test ./...` in `backend/`
    - Fix test files that reference old import paths
    - Verify all existing tests pass after refactoring
    - _Requirements: 3.5, 10.3_

- [ ] 5. Checkpoint — Verify ATM backend compiles and tests pass
  - Ensure `go build ./...` and `go test ./...` pass in `backend/` directory, ask the user if questions arise.

- [ ] 6. Create CIT backend skeleton
  - [ ] 6.1 Create `backend-cit/go.mod` with module path `github.com/cimb-niaga/cms/backend-cit`, Go 1.25.0, require `pkg`, and dependencies (chi/v5, pgx/v5, go-redis/v9, jwt/v5)
    - _Requirements: 4.1, 4.2, 4.6_

  - [ ] 6.2 Create `backend-cit/cmd/api/main.go` — functional Chi server with health check, shared auth middleware, graceful shutdown
    - Set up slog JSON logger
    - Load config via `pkg/config.Load("8081")`
    - Connect pgxpool (primary), redis client
    - Wire TokenService (validation only) and RequireAuth middleware from `pkg/`
    - Implement `/health` endpoint returning `{"status":"ok"}`
    - Implement graceful shutdown with signal handling (SIGINT, SIGTERM)
    - Set up protected route group placeholder
    - _Requirements: 4.4, 6.2, 6.4_

  - [ ] 6.3 Create CIT internal placeholder packages — package declaration only
    - `backend-cit/internal/cit/cit.go` (package cit)
    - `backend-cit/internal/journal/journal.go` (package journal)
    - `backend-cit/internal/dsr/dsr.go` (package dsr)
    - `backend-cit/internal/reconciliation/reconciliation.go` (package reconciliation)
    - `backend-cit/internal/integration/integration.go` (package integration)
    - `backend-cit/internal/handler/handler.go` (package handler)
    - `backend-cit/internal/service/service.go` (package service)
    - `backend-cit/internal/repository/repository.go` (package repository)
    - _Requirements: 4.3, 4.5, 7.1, 7.2_

  - [ ] 6.4 Create `backend-cit/queries/` directory and `backend-cit/sqlc.yaml` for CIT-specific sqlc config
    - sqlc.yaml should point to `./queries/` and output to `internal/repository/`
    - Create a `.gitkeep` in `queries/` to preserve empty directory
    - _Requirements: 5.5_

  - [ ] 6.5 Create `backend-cit/.env.example` with CIT-specific environment variables
    - Include: APP_ENV, PORT (default 8081), DATABASE_URL, DATABASE_REPLICA_URL, REDIS_URL, JWT_SECRET, LOG_LEVEL
    - _Requirements: 9.1, 9.2_

  - [ ] 6.6 Create `backend-cit/Dockerfile` — multi-stage Go build
    - Build stage: golang:1.25-alpine, copy go.work + pkg/ + backend-cit/, build binary
    - Runtime stage: alpine:3.20, add wget for health check, copy binary
    - _Requirements: 8.6, 4.7_

- [ ] 7. Update Docker Compose
  - [ ] 7.1 Add `backend-cit` service to `docker-compose.yml`
    - Build context `.`, dockerfile `backend-cit/Dockerfile`
    - Port mapping 8081:8081
    - Same env_file as ATM backend (shared DATABASE_URL, JWT_SECRET, REDIS_URL)
    - depends_on redis (condition: service_healthy)
    - Health check: wget to `http://localhost:8081/health`
    - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5_

- [ ] 8. Final checkpoint — Verify full workspace builds
  - Run `go build ./...` from repository root with workspace active
  - Run `go vet ./...` from repository root
  - Run `go build ./...` in `backend-cit/` directory to confirm CIT compiles independently
  - Ensure no circular dependencies exist between modules
  - Ensure all tests pass, ask the user if questions arise.
  - _Requirements: 10.1, 10.2, 10.4, 10.5_

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation — the codebase should compile at every checkpoint
- The migration order (pkg → workspace → backend refactor → remove old → CIT skeleton → Docker) ensures no broken intermediate state
- Property tests use `pgregory.net/rapid` consistent with existing test patterns in the backend
- The CIT backend does NOT issue JWT tokens — it only validates them via shared `pkg/auth.TokenService`
- ATM backend retains sole ownership of `backend/migrations/` — CIT tables are migrated there too
- Placeholder packages in CIT contain only `package <name>` declarations — no business logic

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["1.2", "1.4", "1.5"] },
    { "id": 2, "tasks": ["1.3"] },
    { "id": 3, "tasks": ["1.6", "1.8", "1.9", "2.1"] },
    { "id": 4, "tasks": ["1.7", "4.1"] },
    { "id": 5, "tasks": ["4.2", "4.4"] },
    { "id": 6, "tasks": ["4.3", "4.5", "4.6"] },
    { "id": 7, "tasks": ["4.7"] },
    { "id": 8, "tasks": ["6.1"] },
    { "id": 9, "tasks": ["6.2", "6.3", "6.4", "6.5", "6.6"] },
    { "id": 10, "tasks": ["7.1"] }
  ]
}
```
