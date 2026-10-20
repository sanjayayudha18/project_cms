# Requirements Document

## Introduction

Restructure the CMS backend into two separate Go services (ATM backend and CIT backend) using a Go workspace (`go.work`). Shared infrastructure code (auth, middleware, config, response envelope) is extracted into a common `pkg/` module. The CIT backend starts as a minimal skeleton with placeholder packages, sharing the same PostgreSQL database, JWT authentication, and cross-cutting concerns (maker-checker, audit, RBAC). A single migration source remains in `backend/migrations/`.

## Glossary

- **Go_Workspace**: A `go.work` file at the repository root that links multiple Go modules for unified local development
- **Shared_Pkg**: The extracted common module at `pkg/` with path `github.com/cimb-niaga/cms/pkg` containing auth, middleware, config, and response envelope code
- **ATM_Backend**: The existing backend service at `backend/` with module path `github.com/cimb-niaga/cms/backend`
- **CIT_Backend**: The new CIT service skeleton at `backend-cit/` with module path `github.com/cimb-niaga/cms/backend-cit`
- **CIT**: Cash in Transit — physical pickup/delivery of cash by vendor
- **Maker_Checker**: Two-person approval control requiring separate maker and checker for state-changing actions
- **RBAC**: Role-Based Access Control enforced at middleware and service layers

## Requirements

### Requirement 1: Go Workspace Setup

**User Story:** As a developer, I want a Go workspace at the repo root that links all backend modules, so that I can develop across services with local module resolution without publishing.

#### Acceptance Criteria

1. THE Go_Workspace SHALL define a `go.work` file at the repository root (`CMS2/go.work`) using Go version 1.25.0.
2. THE Go_Workspace SHALL include `use` directives for `./backend`, `./backend-cit`, and `./pkg`.
3. WHEN a developer runs `go build ./...` from any module directory, THE Go_Workspace SHALL resolve cross-module imports locally without requiring published versions.
4. THE Go_Workspace SHALL include a `go.work.sum` file tracked in version control after initial generation.

### Requirement 2: Shared Package Module Extraction

**User Story:** As a developer, I want shared infrastructure code extracted into a common module, so that both backends reuse the same auth, middleware, config, and response logic without code duplication.

#### Acceptance Criteria

1. THE Shared_Pkg SHALL have module path `github.com/cimb-niaga/cms/pkg` defined in `pkg/go.mod` with Go version 1.25.0.
2. THE Shared_Pkg SHALL contain the following packages: `pkg/auth`, `pkg/middleware`, `pkg/config`, `pkg/response`.
3. THE Shared_Pkg `pkg/auth` package SHALL provide token service interfaces, token validation, token blacklist interfaces, claims types, and auth context types currently in `backend/internal/auth`.
4. THE Shared_Pkg `pkg/middleware` package SHALL provide the `RequireAuth` middleware, `RequireRoles` middleware, `AuthContext` type, `GetAuthContext` helper, and rate limiter currently in `backend/internal/middleware`.
5. THE Shared_Pkg `pkg/config` package SHALL provide environment-based configuration loading for database URLs (primary and replica), Redis URL, JWT secret, token expiry settings, LDAP settings, and port configuration.
6. THE Shared_Pkg `pkg/response` package SHALL provide a consistent JSON response envelope used by all API endpoints across both backends.
7. THE Shared_Pkg SHALL declare dependencies on `github.com/go-chi/chi/v5`, `github.com/golang-jwt/jwt/v5`, `github.com/jackc/pgx/v5`, `github.com/redis/go-redis/v9`, and `golang.org/x/crypto` in its `go.mod`.
8. WHEN a breaking change is made to Shared_Pkg, THE Shared_Pkg SHALL require both ATM_Backend and CIT_Backend to compile successfully before merging.

### Requirement 3: ATM Backend Module Update

**User Story:** As a developer, I want the existing backend to import shared code from the common module, so that it no longer owns infrastructure code that the CIT backend also needs.

#### Acceptance Criteria

1. THE ATM_Backend SHALL retain its existing module path `github.com/cimb-niaga/cms/backend` in `backend/go.mod`.
2. THE ATM_Backend SHALL add a `require` directive for `github.com/cimb-niaga/cms/pkg` in its `go.mod`.
3. WHEN the Shared_Pkg extraction is complete, THE ATM_Backend SHALL import auth, middleware, config, and response packages from `github.com/cimb-niaga/cms/pkg` instead of `backend/internal`.
4. THE ATM_Backend SHALL remove the extracted code from `backend/internal/auth`, `backend/internal/middleware`, and `backend/internal/config` that has been moved to Shared_Pkg, retaining only backend-specific service logic.
5. WHEN the ATM_Backend is built after restructuring, THE ATM_Backend SHALL compile without errors and pass all existing tests.
6. THE ATM_Backend SHALL continue to own the `backend/migrations/` directory as the single migration source for the shared PostgreSQL database.
7. THE ATM_Backend SHALL continue to own the `backend/queries/` directory and `backend/sqlc.yaml` for its own sqlc-generated code.

### Requirement 4: CIT Backend Skeleton

**User Story:** As a developer, I want a minimal CIT backend skeleton with proper folder structure and module definition, so that CIT domain development can begin with shared infrastructure already wired.

#### Acceptance Criteria

1. THE CIT_Backend SHALL have module path `github.com/cimb-niaga/cms/backend-cit` defined in `backend-cit/go.mod` with Go version 1.25.0.
2. THE CIT_Backend SHALL declare a `require` directive for `github.com/cimb-niaga/cms/pkg` in its `go.mod`.
3. THE CIT_Backend SHALL contain the following directory structure:
   - `backend-cit/cmd/api/main.go` (HTTP server entrypoint placeholder)
   - `backend-cit/internal/cit/` (CIT orders, handover evidences domain)
   - `backend-cit/internal/journal/` (CIT journals domain)
   - `backend-cit/internal/dsr/` (CIT DSR domain)
   - `backend-cit/internal/reconciliation/` (CIT reconciliation domain)
   - `backend-cit/internal/integration/` (external system integrations)
   - `backend-cit/internal/handler/` (HTTP handlers)
   - `backend-cit/internal/service/` (business logic)
   - `backend-cit/internal/repository/` (DB access)
4. THE CIT_Backend `cmd/api/main.go` SHALL set up a Chi router with health check endpoint, import shared auth middleware from Shared_Pkg, and configure graceful shutdown.
5. THE CIT_Backend internal packages SHALL contain empty placeholder files (package declaration only, no working handler implementations).
6. THE CIT_Backend SHALL declare dependencies on `github.com/go-chi/chi/v5`, `github.com/jackc/pgx/v5`, `github.com/redis/go-redis/v9`, and `github.com/golang-jwt/jwt/v5` in its `go.mod`.
7. WHEN the CIT_Backend is built, THE CIT_Backend SHALL compile without errors.

### Requirement 5: Shared Database Access

**User Story:** As a system architect, I want both backends to share the same PostgreSQL database, so that CIT data and ATM data remain consistent and cross-module reconciliation is possible.

#### Acceptance Criteria

1. THE CIT_Backend SHALL connect to the same PostgreSQL primary instance (via `DATABASE_URL`) as the ATM_Backend for write operations.
2. THE CIT_Backend SHALL connect to the same PostgreSQL read replica (via `DATABASE_REPLICA_URL`) as the ATM_Backend for reporting and dashboard queries.
3. THE ATM_Backend SHALL remain the sole owner of schema migrations in `backend/migrations/`, including tables used by the CIT_Backend.
4. WHEN a new CIT-related table is needed, THE ATM_Backend migration directory SHALL contain the migration file.
5. THE CIT_Backend SHALL have its own `backend-cit/queries/` directory and `backend-cit/sqlc.yaml` for CIT-specific sqlc-generated query code.

### Requirement 6: Shared JWT Authentication

**User Story:** As a security architect, I want both backends to validate JWT tokens using the same secret and claims structure, so that a user authenticated on one service is recognized by the other.

#### Acceptance Criteria

1. THE CIT_Backend SHALL use the same `JWT_SECRET` environment variable as the ATM_Backend for token validation.
2. THE CIT_Backend SHALL use the `RequireAuth` middleware from Shared_Pkg to validate Bearer tokens on protected routes.
3. THE CIT_Backend SHALL use the `RequireRoles` middleware from Shared_Pkg to enforce RBAC on CIT-specific endpoints.
4. WHEN a JWT token is issued by the ATM_Backend auth service, THE CIT_Backend SHALL accept the token as valid without requiring re-authentication.
5. THE CIT_Backend SHALL respect token blacklist entries created by the ATM_Backend (shared Redis-backed blacklist via same `REDIS_URL`).

### Requirement 7: Cross-Cutting Concerns for CIT Backend

**User Story:** As a compliance officer, I want maker-checker approval and audit logging enforced on the CIT backend, so that all state-changing CIT operations meet the same governance standards as ATM operations.

#### Acceptance Criteria

1. THE CIT_Backend skeleton SHALL include placeholder structure for audit log writing in CIT service layer packages.
2. THE CIT_Backend skeleton SHALL include placeholder structure for maker-checker approval integration in CIT service layer packages.
3. WHEN CIT domain handlers are implemented (future), THE CIT_Backend SHALL write to the shared `audit_logs` table for every state-changing action.
4. WHEN CIT domain handlers are implemented (future), THE CIT_Backend SHALL create `approval_requests` records for financial and master data mutations requiring maker-checker control.
5. THE CIT_Backend SHALL use the consistent JSON response envelope from Shared_Pkg for all API responses.

### Requirement 8: Docker Compose Update

**User Story:** As a developer, I want docker-compose to orchestrate both backends, so that I can run the full system locally with a single command.

#### Acceptance Criteria

1. WHEN `docker compose up` is run, THE Docker_Compose SHALL start both ATM_Backend and CIT_Backend services alongside Redis.
2. THE Docker_Compose SHALL define a `backend-cit` service with its own Dockerfile, port mapping (port 8081 externally), and health check.
3. THE Docker_Compose SHALL share the same Redis instance between ATM_Backend and CIT_Backend services.
4. THE Docker_Compose SHALL pass the same `DATABASE_URL`, `DATABASE_REPLICA_URL`, `JWT_SECRET`, and `REDIS_URL` environment variables to both backend services.
5. THE Docker_Compose CIT_Backend service SHALL depend on Redis being healthy before starting.
6. THE CIT_Backend SHALL have its own Dockerfile (`docker/Dockerfile.backend-cit` or `backend-cit/Dockerfile`) using multi-stage Go build producing a minimal binary image.

### Requirement 9: Environment Configuration

**User Story:** As a developer, I want clear environment configuration for both backends, so that each service can be configured independently while sharing core connection strings.

#### Acceptance Criteria

1. THE CIT_Backend SHALL have its own `.env.example` file documenting all required environment variables.
2. THE CIT_Backend `.env.example` SHALL include: `APP_ENV`, `PORT` (default 8081), `DATABASE_URL`, `DATABASE_REPLICA_URL`, `REDIS_URL`, `JWT_SECRET`, and `LOG_LEVEL`.
3. THE Shared_Pkg config loader SHALL support loading configuration from environment variables for both backends using the same struct patterns.
4. WHEN `PORT` is not set for CIT_Backend, THE CIT_Backend SHALL default to port 8081 to avoid conflict with ATM_Backend on port 8080.

### Requirement 10: Build and Compilation Integrity

**User Story:** As a CI engineer, I want the entire Go workspace to build cleanly from the root, so that the CI pipeline can validate all modules in a single step.

#### Acceptance Criteria

1. WHEN `go build ./...` is run from the repository root with the workspace active, THE Go_Workspace SHALL compile all three modules (pkg, backend, backend-cit) without errors.
2. WHEN `go vet ./...` is run from the repository root, THE Go_Workspace SHALL report no issues across all modules.
3. WHEN `go test ./...` is run from the `backend/` directory, THE ATM_Backend SHALL pass all existing tests after the refactoring.
4. WHEN `go build ./...` is run in `backend-cit/`, THE CIT_Backend SHALL compile successfully with placeholder packages.
5. IF a circular dependency is introduced between modules, THEN THE Go_Workspace SHALL fail to compile, preventing the dependency from being merged.
