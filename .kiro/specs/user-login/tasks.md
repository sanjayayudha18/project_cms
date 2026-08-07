# Implementation Plan: User Login

## Overview

Implementation plan for the user-login feature covering database migration, backend auth system (Go + Chi + Redis + JWT), and frontend auth refactoring for both CompanyPortal and VendorPortal. Tasks are ordered by dependency: DB migration → backend core → backend handlers → frontend refactoring → integration tests.

## Tasks

- [x] 1. Database Migration for Development Auth
  Create migration `003_add_local_dev_auth` that enables local credential login for internal users during development.
  **Validates: Requirement 1**
  - [x] 1.1 Create `backend/migrations/003_add_local_dev_auth.up.sql` with: expand `users_auth_source_chk` to include 'local_dev', modify `users_password_hash_chk` to allow password_hash when auth_source='local_dev', modify `users_vendor_logic_chk` to allow (is_karyawan=true AND auth_source='local_dev' AND vendor_id IS NULL), update internal users to auth_source='local_dev' with bcrypt hash of default dev password
  - [x] 1.2 Create `backend/migrations/003_add_local_dev_auth.down.sql` that reverts internal users to auth_source='ldap', removes password_hash, and restores original constraints
  - [x] 1.3 Run migration up and verify constraints are correctly applied — test that valid combinations succeed and invalid combinations are rejected by the DB

- [x] 2. Backend Project Initialization
  Initialize the Go backend project structure with module, dependencies, and core configuration loading.
  **Validates: Requirements 2, 4, 6, 7**
  - [x] 2.1 Initialize `backend/go.mod` with module path and Go 1.23+; add dependencies: chi/v5, golang-jwt/v5, golang.org/x/crypto, go-redis/v9, validator/v10, google/uuid, flyingmutant/rapid (test)
  - [x] 2.2 Create `backend/internal/config/config.go` — struct for app configuration loaded from environment: JWTSecret ([]byte, min 32 bytes), AccessTokenExpiry (default 15min), RefreshTokenExpiry (default 7 days), RedisURL, DatabaseURL, RateLimitUsername (default 5), RateLimitIP (default 20), RateLimitWindow (default 15min)
  - [x] 2.3 Create `backend/cmd/api/main.go` — entrypoint that loads config, connects to PostgreSQL and Redis, wires dependencies, starts Chi HTTP server

- [x] 3. Auth Provider Interface and Local Provider
  Implement the auth provider abstraction layer with the local bcrypt provider implementation.
  **Validates: Requirements 2, 6**
  - [x] 3.1 Create `backend/internal/auth/provider.go` — define Provider interface with Authenticate(ctx, username, password) and Supports(authSource) methods; define AuthIdentity struct
  - [x] 3.2 Create `backend/internal/auth/errors.go` — define sentinel errors: ErrInvalidCredentials, ErrAccountInactive, ErrPortalMismatch, ErrUnsupportedAuthSource, ErrLDAPNotConfigured, ErrRateLimited, ErrServiceUnavailable, ErrValidation
  - [x] 3.3 Create `backend/internal/auth/repository.go` — define UserRepository interface with FindByUsername and UpdateLastLogin; define UserRecord struct matching DB columns
  - [x] 3.4 Create `backend/internal/auth/local_provider.go` — implement LocalProvider: Supports returns true for "local" and "local_dev"; Authenticate uses bcrypt cost 12 constant-time comparison
  - [x] 3.5 Create `backend/internal/auth/local_provider_test.go` — unit tests: valid password returns identity, wrong password returns ErrInvalidCredentials, user not found returns error
  - [x] 3.6 Write property test `backend/internal/auth/local_provider_property_test.go` — Property 1: Bcrypt Round-Trip Verification using rapid

- [x] 4. Token Service
  Implement JWT token generation, validation, refresh, and blacklisting via Redis.
  **Validates: Requirements 4, 8**
  - [x] 4.1 Create `backend/internal/auth/token_service.go` — TokenService with GenerateTokenPair using HMAC-SHA256; access token (15min) with id, username, role, is_karyawan, vendor_id, iat, exp; refresh token (7 days) with id, jti UUID, iat, exp
  - [x] 4.2 Implement ValidateAccessToken — verify signature, check expiry, confirm required claims present
  - [x] 4.3 Implement RefreshTokens — validate refresh token, check JTI against Redis blacklist, generate new token pair with new JTI
  - [x] 4.4 Create `backend/internal/auth/token_blacklist.go` — TokenBlacklist interface and Redis implementation with key pattern blacklist:jti:{jti}
  - [x] 4.5 Implement BlacklistRefreshToken — parse token to extract JTI and remaining TTL, add to Redis blacklist
  - [x] 4.6 Create `backend/internal/auth/token_service_test.go` — unit tests: generate pair valid, expired rejected, tampered rejected, blacklisted JTI rejected, missing claims detection
  - [x] 4.7 Write property test `backend/internal/auth/token_property_test.go` — Property 3: Access Token Claims Completeness, Property 4: Refresh Token Uniqueness and Rotation, Property 5: Token Signature Integrity

- [x] 5. Rate Limiter
  Implement Redis-backed rate limiting for login attempts per username and per IP.
  **Validates: Requirement 7**
  - [x] 5.1 Create `backend/internal/middleware/rate_limiter.go` — RateLimiter struct with Redis client; keys rate:login:user:{username} and rate:login:ip:{ip} with TTL 900s
  - [x] 5.2 Implement Check(ctx, username, ip) — INCR counters, if exceeds limit return ErrRateLimited with Retry-After; if Redis unavailable return ErrServiceUnavailable (fail-closed)
  - [x] 5.3 Implement IncrementFailed(ctx, username, ip) — increment both counters after auth failure
  - [x] 5.4 Implement ResetUsername(ctx, username) — delete username counter on successful login; IP counter unchanged
  - [x] 5.5 Create `backend/internal/middleware/rate_limiter_test.go` — unit tests: allows up to limit, blocks at threshold+1, correct Retry-After, reset clears username only, Redis failure returns 503
  - [x] 5.6 Write property test `backend/internal/middleware/rate_limiter_property_test.go` — Property 8: Username Rate Limit Threshold, Property 9: IP Rate Limit Threshold, Property 10: Rate Limit Reset on Success

- [x] 6. Auth Service Login Orchestration
  Implement the core auth service that orchestrates the full login flow in the correct order.
  **Validates: Requirements 2, 3, 6, 7, 12**
  - [x] 6.1 Create `backend/internal/auth/service.go` — Service struct with providers, tokenService, userRepo, rateLimiter; define LoginRequest, LoginResponse, UserProfile structs
  - [x] 6.2 Implement Login with ordered evaluation: (1) input validation, (2) rate limit check, (3) user lookup, (4) is_active check, (5) credential verification, (6) portal type restriction, (7) generate tokens, (8) update last_login_at + reset rate limit
  - [x] 6.3 Implement provider selection logic: find provider where Supports(authSource) is true; ldap without LDAP provider returns error; unknown auth_source returns error
  - [x] 6.4 Implement portal type validation: after credential success, check is_karyawan vs portal_type; missing/invalid X-Portal-Type returns 422
  - [x] 6.5 Create `backend/internal/auth/service_test.go` — unit tests: successful login, invalid credentials generic error, deleted user generic error, inactive 403, portal mismatch, validation before auth, rate limit blocks
  - [x] 6.6 Write property test `backend/internal/auth/service_property_test.go` — Property 2: Auth Error Uniformity, Property 14: Portal Type Isolation, Property 15: Auth Provider Selection, Property 16: Input Validation Priority, Property 17: Password Length Boundaries

- [x] 7. RBAC Middleware
  Implement authentication and role-based authorization middleware for protected endpoints.
  **Validates: Requirement 5**
  - [x] 7.1 Create `backend/internal/middleware/rbac.go` — RequireAuth middleware: extract Bearer token, validate via TokenService, inject AuthContext into request context; return 401 for invalid tokens
  - [x] 7.2 Implement RequireRoles(allowedRoles ...string) — check user role against allowedRoles; return 403 if not permitted; empty allowedRoles allows any authenticated user
  - [x] 7.3 Implement GetAuthContext(ctx) helper for downstream handlers
  - [x] 7.4 Create `backend/internal/middleware/rbac_test.go` — unit tests: valid token passes, missing header 401, expired 401, invalid signature 401, role mismatch 403, empty roles allows all, all 9 DB roles recognized
  - [x] 7.5 Write property test `backend/internal/middleware/rbac_property_test.go` — Property 6: RBAC Role Authorization, Property 7: Auth Context Injection

- [x] 8. Auth Handler and HTTP Routes
  Implement the HTTP handler layer with all auth endpoints and error response formatting.
  **Validates: Requirements 3, 4, 8, 12**
  - [x] 8.1 Create `backend/internal/handler/auth_handler.go` — AuthHandler with Routes() returning chi.Router: POST /login, POST /refresh, POST /logout, GET /me
  - [x] 8.2 Implement Login handler — parse JSON body, read X-Portal-Type header, call authService.Login, return 200 with access_token + user + set httpOnly cookie for refresh_token
  - [x] 8.3 Implement Refresh handler — read refresh_token cookie, return new access_token + user + rotated cookie; 401 if invalid
  - [x] 8.4 Implement Logout handler — blacklist JTI if valid; always clear cookie + return 200 (idempotent)
  - [x] 8.5 Implement Me handler — extract AuthContext, query user profile, return JSON
  - [x] 8.6 Create `backend/internal/handler/error_response.go` — standard error format {error, message, details} with helpers for 401, 403, 422, 429, 503
  - [x] 8.7 Wire routes in main.go — mount at /api/v1/auth, apply rate limiter to login, apply RBAC to protected routes
  - [x] 8.8 Write property test `backend/internal/auth/session_property_test.go` — Property 11: Blacklist Prevents Token Reuse, Property 12: Idempotent Logout

- [x] 9. Database Repository Implementation
  Implement sqlc-based repository for user queries needed by the auth service.
  **Validates: Requirements 1, 2, 3**
  - [x] 9.1 Create `backend/queries/auth.sql` — sqlc queries: FindUserByUsername, UpdateLastLogin, GetUserProfile
  - [x] 9.2 Create `backend/sqlc.yaml` configuration for code generation
  - [x] 9.3 Run sqlc generate and verify generated Go code compiles
  - [x] 9.4 Create `backend/internal/repository/auth_repository.go` — adapter implementing auth.UserRepository using sqlc-generated code

- [ ] 10. CompanyPortal Auth Store Refactoring
  Refactor existing auth store to use DB role strings, add rate limit state, wire to real backend.
  **Validates: Requirements 3, 9, 10, 11, 12**
  - [~] 10.1 Refactor store.ts — replace Role type with DbRole union of 9 DB roles; AuthUser uses single role: DbRole; add username, vendorId fields
  - [~] 10.2 Update login function — add X-Portal-Type: company header; handle all error responses; add rateLimitRetryAfter state
  - [~] 10.3 Update token refresh — single-flight refresh; on failure redirect to /login?redirect={path}
  - [~] 10.4 Add isAuthLoading on page load — initialize() attempts refresh via cookie
  - [~] 10.5 Remove ROLE_NAV_PERMISSIONS map from store
  - [~] 10.6 Update store.test.ts — fix tests for new Role type and single-role model

- [ ] 11. CompanyPortal Navigation Refactoring
  Refactor navigation config to use DB role strings and update filterNavByRoles logic.
  **Validates: Requirement 11**
  - [~] 11.1 Refactor navigation.ts — change NavItem.roles to (DbRole | "*")[]; update all NAV_CONFIG entries to DB role strings per design role mapping
  - [~] 11.2 Refactor filterNavByRoles — accept single userRole: DbRole; ADMIN/ADMIN_PARAM see all; others see only their allowed items
  - [~] 11.3 Update Sidebar component to pass single role to filterNavByRoles
  - [~] 11.4 Update navigation.test.ts for new role types
  - [~] 11.5 Write property test navigation.property.test.ts — Property 13: Navigation Filtering by Role using fast-check

- [ ] 12. CompanyPortal Login Page Refactoring
  Refactor login page to connect to real backend with rate limit display and accessibility.
  **Validates: Requirements 9, 10, 12**
  - [~] 12.1 Refactor login.tsx — use username field; handle all error types with Indonesian messages; redirect to stored path or /dashboard on success
  - [~] 12.2 Add rate limit lockout UI — parse Retry-After, countdown in "M menit S detik" format, disable submit
  - [~] 12.3 Add form validation — non-whitespace required, max lengths (128/72), Enter key submits
  - [~] 12.4 Ensure accessibility — htmlFor/id labels, 3px --red-100 focus halo, aria-live="polite" errors
  - [~] 12.5 Add already-authenticated redirect to /dashboard
  - [~] 12.6 Write property test login.property.test.tsx — Property 18: Whitespace-Only Input Rejection, Property 19: Retry-After Countdown Formatting using fast-check

- [ ] 13. CompanyPortal API Client Update
  Update API client to attach Bearer token and handle 401 with automatic refresh.
  **Validates: Requirement 10**
  - [~] 13.1 Refactor client.ts — add Authorization: Bearer header from store; response interceptor catches 401, triggers single-flight refresh
  - [~] 13.2 Implement single-flight refresh — module-level promise ensures one refresh in-flight; queue concurrent 401s
  - [~] 13.3 Remove stub mode fallback for auth endpoints

- [ ] 14. CompanyPortal Route Guards
  Implement protected route wrapper with role-based access and 403 page.
  **Validates: Requirements 10, 11**
  - [~] 14.1 Refactor _protected.tsx — check isAuthLoading first; redirect unauthenticated to /login?redirect={path}; show 403 if role not in allowed list
  - [~] 14.2 Create Forbidden.tsx — 403 page with "Anda tidak memiliki akses ke halaman ini" and link to /dashboard
  - [~] 14.3 Ensure navigation renders within 1 render cycle after auth state available

- [ ] 15. VendorPortal Auth Refactoring
  Refactor VendorPortal from simulated auth to real backend API integration.
  **Validates: Requirements 10, 12**
  - [~] 15.1 Refactor AuthContext.tsx — remove simulated data; implement real auth state with access token in memory, hit backend with X-Portal-Type: vendor
  - [~] 15.2 Refactor LoginPage.tsx — real backend login with vendor portal type; add "Vendor Portal" subtitle; form validation, rate limit handling, CIMB branding, accessibility
  - [~] 15.3 Refactor useAuth.ts — token refresh on 401; clear state + redirect on failure; preserve redirect URL
  - [~] 15.4 Refactor ProtectedRoute.tsx — check isAuthLoading; redirect unauthenticated to /login?redirect={path}
  - [~] 15.5 Create/update VendorPortal API client with Bearer token and single-flight refresh

- [ ] 16. Integration Testing
  Write integration tests verifying full login flow end-to-end against PostgreSQL and Redis.
  **Validates: Requirements 1-12 (integration coverage)**
  - [~] 16.1 Create integration_test.go — setup test DB with migration, Redis test instance; test full login flow HTTP to DB to response
  - [~] 16.2 Integration tests for: company login, vendor login, portal mismatch, rate limit enforcement, token refresh, logout blacklists JTI, refresh after logout fails
  - [~] 16.3 Redis failure scenario — verify 503 when Redis unreachable (fail-closed)

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "1.2"] },
    { "id": 1, "tasks": ["1.3"] },
    { "id": 2, "tasks": ["2.1", "2.2", "2.3"] },
    { "id": 3, "tasks": ["3.1", "3.2", "3.3", "4.1", "4.4", "5.1", "9.1", "9.2"] },
    { "id": 4, "tasks": ["3.4", "3.5", "4.2", "4.3", "4.5", "5.2", "5.3", "5.4", "9.3"] },
    { "id": 5, "tasks": ["3.6", "4.6", "4.7", "5.5", "5.6", "9.4"] },
    { "id": 6, "tasks": ["6.1", "6.2", "6.3", "6.4", "7.1", "7.2", "7.3"] },
    { "id": 7, "tasks": ["6.5", "6.6", "7.4", "7.5"] },
    { "id": 8, "tasks": ["8.1", "8.2", "8.3", "8.4", "8.5", "8.6"] },
    { "id": 9, "tasks": ["8.7", "8.8"] },
    { "id": 10, "tasks": ["10.1", "10.2", "10.3", "10.4", "10.5", "15.1", "15.2", "15.3", "15.4", "15.5", "16.1"] },
    { "id": 11, "tasks": ["10.6", "16.2", "16.3"] },
    { "id": 12, "tasks": ["11.1", "11.2", "11.3", "12.1", "12.2", "12.3", "12.4", "12.5", "13.1", "13.2", "13.3"] },
    { "id": 13, "tasks": ["11.4", "11.5", "12.6"] },
    { "id": 14, "tasks": ["14.1", "14.2", "14.3"] }
  ]
}
```

## Notes

- Backend property tests use [rapid](https://github.com/flyingmutant/rapid) for Go
- Frontend property tests use [fast-check](https://github.com/dubzzz/fast-check) for TypeScript
- Tasks 3-5 and 9 can be developed in parallel after Task 2 completes
- Tasks 10-15 (frontend) can be developed in parallel after Task 8 completes
- All property tests include tag format: `// Feature: user-login, Property {N}: {title}`
