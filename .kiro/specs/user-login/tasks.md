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
  Refactor the existing Zustand auth store (`frontend/CompanyPortal-Vite/src/lib/auth/store.ts`) to use DB role strings, wire to real backend API, implement token refresh with single-flight pattern, and handle rate limit state.
  **Validates: Requirements 3, 9, 10, 11, 12**
  - [ ] 10.1 Refactor `src/lib/auth/store.ts` — replace the existing Role type with `DbRole` union type of 9 DB roles (`ADMIN | ADMIN_PARAM | ATM-USER | ATM-SPV | BRANCH-USER | BRANCH-SPV | BRANCH-ATM-USER | BRANCH-ATM-SPV | VENDOR-USER`); update `AuthUser` interface to use single `role: DbRole` field; add `username`, `vendorId` fields; store access token in memory (not localStorage); add `rateLimitRetryAfter: number | null` state; add `isAuthLoading: boolean` state
    - _Requirements: 10.1, 10.5, 10.7_
  - [ ] 10.2 Implement `login` action in store — call `POST /api/v1/auth/login` with JSON body and `X-Portal-Type: company` header; on success extract `access_token` and `user` from response; on error parse error type (`auth_failed`, `account_inactive`, `portal_mismatch`, `rate_limited`, `validation_error`, `service_unavailable`) and set appropriate error state; for 429 parse `Retry-After` header and set `rateLimitRetryAfter`
    - _Requirements: 3.1, 9.4, 9.8, 12.3_
  - [ ] 10.3 Implement `refreshToken` action with single-flight pattern — use module-level `refreshPromise` variable; if a refresh is in-flight, return existing promise; call `POST /api/v1/auth/refresh` (cookie-based); on success update access token and user in store; on failure clear auth state and redirect to `/login?redirect={currentPath}`
    - _Requirements: 10.3, 10.4, 8.7_
  - [ ] 10.4 Implement `initialize` function — on app load, set `isAuthLoading=true`; attempt token refresh via cookie; on success populate auth state; on failure set `isAuthenticated=false`; finally set `isAuthLoading=false`; this enables route guards to wait before evaluating auth
    - _Requirements: 10.7_
  - [ ] 10.5 Implement `logout` action — call `POST /api/v1/auth/logout`; clear access token, user, and auth state from store regardless of response status
    - _Requirements: 8.1, 8.2_
  - [ ]* 10.6 Update `src/lib/auth/store.test.ts` — rewrite unit tests for new DbRole type, single-role model, login error handling, refresh single-flight behavior, initialize flow, and logout idempotency
    - _Requirements: 10.1–10.7_

- [ ] 11. CompanyPortal Navigation Refactoring
  Refactor navigation configuration (`frontend/CompanyPortal-Vite/src/lib/config/navigation.ts`) to use DB role strings and update `filterNavByRoles` logic for single-role filtering.
  **Validates: Requirement 11**
  - [ ] 11.1 Refactor `src/lib/config/navigation.ts` — change `NavItem.roles` type to `(DbRole | "*")[]`; update all `NAV_CONFIG` entries to use DB role strings per the design role mapping table; remove any legacy role mapping constants (e.g., `ROLE_NAV_PERMISSIONS`)
    - _Requirements: 11.1, 11.5, 11.6_
  - [ ] 11.2 Refactor `filterNavByRoles` function — accept single `userRole: DbRole` parameter; if role is `ADMIN` or `ADMIN_PARAM` return all items; for `VENDOR-USER` return only items with vendor-specific routes or `"*"` wildcard; for other roles filter items where `item.roles.includes(userRole)` or `item.roles.includes("*")`; if role matches nothing, return only the default dashboard item
    - _Requirements: 11.2, 11.4, 11.5, 11.6, 11.7_
  - [ ] 11.3 Update Sidebar component — pass single `user.role` (DbRole) to `filterNavByRoles` instead of roles array; ensure navigation renders within 1 render cycle after auth state is available (no intermediate empty/unfiltered state)
    - _Requirements: 11.2, 11.8_
  - [ ]* 11.4 Update `src/lib/config/navigation.test.ts` — rewrite tests for DB role strings, single-role filtering, ADMIN sees all, VENDOR-USER sees vendor items only, unknown role gets dashboard only
    - _Requirements: 11.1–11.8_
  - [ ]* 11.5 Write property test `src/lib/config/navigation.property.test.ts` — **Property 13: Navigation Filtering by Role** — using fast-check, generate arbitrary DbRole values and NavItem configurations; verify ADMIN/ADMIN_PARAM always get all items; verify other roles only get items where their role is in the allowed list or wildcard is present
    - **Property 13: Navigation Filtering by Role**
    - **Validates: Requirements 11.2, 11.4, 11.5, 11.6**

- [ ] 12. CompanyPortal Login Page Refactoring
  Refactor login page (`frontend/CompanyPortal-Vite/src/routes/login.tsx`) to connect to real backend with proper error handling, rate limit lockout display, form validation, and accessibility.
  **Validates: Requirements 9, 10, 12**
  - [ ] 12.1 Refactor `src/routes/login.tsx` — use `username` field (not email); call store `login` action; handle all backend error types with Indonesian messages (`auth_failed` → "Username atau password salah", `account_inactive` → "Akun tidak aktif. Hubungi administrator.", `portal_mismatch` → "Akun tidak memiliki akses ke portal ini", `service_unavailable` → "Layanan sedang tidak tersedia. Coba lagi nanti."); on success redirect to stored path or `/dashboard`
    - _Requirements: 9.4, 9.5, 3.3, 3.4_
  - [ ] 12.2 Implement rate limit lockout UI — when `rateLimitRetryAfter` is set, display countdown formatted as "M menit S detik" (e.g., "9 menit 0 detik"); disable submit button during lockout; decrement countdown every second; clear lockout state when countdown reaches 0
    - _Requirements: 9.8_
  - [ ] 12.3 Implement form validation — both username and password must contain at least one non-whitespace character (trim and check length > 0); enforce max length of 128 chars for username and 72 chars for password via `maxLength` attribute; support form submission via Enter key in any input field; disable submit button when fields are invalid or request is in-flight
    - _Requirements: 9.6, 9.7_
  - [ ] 12.4 Implement accessibility — pair each input with visible label via `htmlFor`/`id`; apply 3px `--red-100` box-shadow focus halo on focus-visible; wrap error messages in `aria-live="polite"` region; apply CIMB branding: logo, centered card (`--n-0` surface) over `--n-50` page background, `--red-500` primary submit button
    - _Requirements: 9.1, 9.9_
  - [ ] 12.5 Implement already-authenticated redirect — if user is authenticated when navigating to `/login`, redirect to `/dashboard` without showing login form; check after `isAuthLoading` resolves
    - _Requirements: 9.10_
  - [ ]* 12.6 Write property test `src/routes/login.property.test.ts` — **Property 18: Whitespace-Only Input Rejection** — using fast-check, generate strings of only whitespace characters and verify form treats them as empty; **Property 19: Retry-After Countdown Formatting** — for any integer N in [1, 900], verify display formats correctly as "M menit S detik" where M=floor(N/60), S=N%60
    - **Property 18: Whitespace-Only Input Rejection**
    - **Property 19: Retry-After Countdown Formatting**
    - **Validates: Requirements 9.6, 9.8**

- [ ] 13. CompanyPortal API Client Update
  Update the API client (`frontend/CompanyPortal-Vite/src/lib/api/client.ts`) to attach Bearer token and handle 401 with automatic single-flight refresh.
  **Validates: Requirement 10**
  - [ ] 13.1 Refactor `src/lib/api/client.ts` — add request interceptor that reads access token from auth store and sets `Authorization: Bearer {token}` header on every request; add response interceptor that catches HTTP 401 responses
    - _Requirements: 10.2_
  - [ ] 13.2 Implement single-flight refresh in response interceptor — on 401, call `store.refreshToken()`; if refresh succeeds, retry original request with new token; if refresh fails, clear auth state and redirect to `/login?redirect={path}`; use module-level promise to ensure only one refresh call is in-flight (queue concurrent 401s behind the same promise)
    - _Requirements: 10.3, 10.4_
  - [ ] 13.3 Remove stub mode and simulated auth fallbacks — delete any mock/stub logic for auth endpoints in the API client; ensure all auth calls go to real backend
    - _Requirements: 10.2_

- [ ] 14. Checkpoint — Verify CompanyPortal auth flow
  - Ensure all CompanyPortal auth-related tests pass, ask the user if questions arise.

- [ ] 15. CompanyPortal Route Guards
  Implement protected route wrapper (`frontend/CompanyPortal-Vite/src/routes/_protected.tsx`) with auth loading guard, role-based access control, and 403 page.
  **Validates: Requirements 10, 11**
  - [ ] 15.1 Refactor `src/routes/_protected.tsx` — check `isAuthLoading` first (show nothing or spinner while resolving); if not authenticated after loading completes, redirect to `/login?redirect={currentPath}`; if authenticated but user role not in route's allowed roles list, render Forbidden component
    - _Requirements: 10.4, 10.7, 11.3_
  - [ ] 15.2 Create `src/components/Forbidden.tsx` — 403 page with message "Anda tidak memiliki akses ke halaman ini" and a link/button navigating back to `/dashboard`; follow CIMB design system (centered card, `--n-0` surface, `--n-50` background)
    - _Requirements: 11.3_
  - [ ] 15.3 Ensure navigation renders within 1 render cycle after auth state available — after `isAuthLoading` flips to false, the next render must show the correctly filtered sidebar without intermediate empty/full states
    - _Requirements: 11.8_

- [ ] 16. VendorPortal Auth Refactoring
  Refactor VendorPortal auth from simulated data to real backend API integration (`frontend/VendorPortal-Vite/src/features/auth/`).
  **Validates: Requirements 10, 12**
  - [ ] 16.1 Refactor `src/features/auth/AuthContext.tsx` — remove simulated JWT/vendor data; implement real auth context with: `accessToken` in memory (not localStorage), `user: AuthUser | null`, `isAuthenticated`, `isAuthLoading` state; on mount attempt token refresh via `POST /api/v1/auth/refresh` (cookie-based) to restore session; expose `login`, `logout`, `refreshToken` methods via context
    - _Requirements: 10.1, 10.5, 10.7, 12.9_
  - [ ] 16.2 Refactor `src/features/auth/LoginPage.tsx` — call `POST /api/v1/auth/login` with `X-Portal-Type: vendor` header; display "Vendor Portal" subtitle on login card; apply CIMB branding (`--n-0` card on `--n-50` background, `--red-500` submit button); implement same form validation rules as CompanyPortal (non-empty, whitespace-only treated as empty, maxLength 128/72); implement accessibility (htmlFor/id label pairing, 3px `--red-100` focus halo, `aria-live="polite"` error region); handle all error types with Indonesian messages including portal mismatch ("Akun tidak memiliki akses ke portal ini"); implement rate limit lockout with "M menit S detik" countdown from Retry-After header; on success redirect to stored path or `/dashboard`
    - _Requirements: 12.1, 12.2, 12.7, 12.8, 12.11_
  - [ ] 16.3 Refactor `src/features/auth/useAuth.ts` — implement `useAuth` hook that consumes AuthContext; expose token refresh on HTTP 401; clear auth state + redirect to `/login?redirect={path}` on refresh failure; preserve redirect URL through login flow
    - _Requirements: 10.3, 10.4, 12.9_
  - [ ] 16.4 Refactor `src/features/auth/ProtectedRoute.tsx` — check `isAuthLoading` first (show loading/nothing while resolving); if not authenticated after load, redirect to `/login?redirect={currentPath}`; wrap children with auth guard
    - _Requirements: 10.7, 12.9_
  - [ ] 16.5 Create/update VendorPortal API client (`src/lib/api/`) — create fetch wrapper or axios instance that attaches `Authorization: Bearer {token}` header from auth context; implement single-flight refresh on 401 (same pattern as CompanyPortal); ensure all API calls route through this client
    - _Requirements: 10.2, 10.3, 12.9_

- [ ] 17. Checkpoint — Verify VendorPortal auth flow
  - Ensure all VendorPortal auth-related tests pass, ask the user if questions arise.

- [ ] 18. Integration Testing
  Write integration tests verifying full login flow end-to-end against PostgreSQL and Redis.
  **Validates: Requirements 1–12 (integration coverage)**
  - [ ] 18.1 Create `backend/internal/auth/integration_test.go` — setup test DB with migration applied, Redis test instance; test full login flow: HTTP POST → rate limit check → DB lookup → bcrypt verify → token generation → cookie set → response validation
    - _Requirements: 1, 2, 3, 4, 6_
  - [ ] 18.2 Write integration tests for portal isolation — company user login with X-Portal-Type: company succeeds; vendor user login with X-Portal-Type: vendor succeeds; company user with X-Portal-Type: vendor returns 403 portal_mismatch; vendor user with X-Portal-Type: company returns 403 portal_mismatch; invalid portal type returns 422
    - _Requirements: 12.3, 12.4, 12.5, 12.6, 12.10_
  - [ ] 18.3 Write integration tests for rate limiting — 5 failed attempts for same username then 6th blocked with 429 + Retry-After header; successful login resets username counter; 20 failed attempts from same IP then 21st blocked
    - _Requirements: 7.1, 7.2, 7.3, 7.5_
  - [ ] 18.4 Write integration tests for token lifecycle — refresh returns new access token + rotated refresh cookie; logout blacklists JTI; refresh after logout returns 401; expired access token returns 401
    - _Requirements: 4.4, 8.1, 8.3, 8.4_
  - [ ] 18.5 Write integration test for Redis failure scenario — verify HTTP 503 returned when Redis is unreachable (fail-closed behavior)
    - _Requirements: 7.7, 8.5_

- [ ] 19. Final checkpoint — Ensure all tests pass
  - Run `go test ./...` for backend and `pnpm test` for both frontend portals. Ensure all tests pass, ask the user if questions arise.

## Notes

- Backend property tests use [rapid](https://github.com/flyingmutant/rapid) for Go
- Frontend property tests use [fast-check](https://github.com/dubzzz/fast-check) for TypeScript
- Tasks 3–5 and 9 can be developed in parallel after Task 2 completes
- Tasks 10–16 (frontend) can be developed in parallel after Task 8 completes
- All property tests include tag format: `// Feature: user-login, Property {N}: {title}`
- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["10.1"] },
    { "id": 1, "tasks": ["10.2", "10.3", "10.4", "10.5", "11.1"] },
    { "id": 2, "tasks": ["10.6", "11.2", "11.3", "13.1", "13.2", "13.3"] },
    { "id": 3, "tasks": ["11.4", "11.5", "12.1", "12.2", "12.3", "12.4", "12.5"] },
    { "id": 4, "tasks": ["12.6", "15.1", "15.2", "15.3"] },
    { "id": 5, "tasks": ["16.1"] },
    { "id": 6, "tasks": ["16.2", "16.3", "16.4", "16.5"] },
    { "id": 7, "tasks": ["18.1", "18.2", "18.3", "18.4", "18.5"] }
  ]
}
```
