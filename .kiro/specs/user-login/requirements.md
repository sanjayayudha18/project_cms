# Requirements Document

## Introduction

Login feature untuk Cash Management System (CMS) CIMB Niaga. Sistem mendukung dua tipe user (internal karyawan dan vendor) yang saat ini seluruhnya login menggunakan credential lokal (username + password). Arsitektur auth dirancang dengan abstraksi auth source agar LDAP dapat dipasang di production tanpa perubahan handler/service layer. Session berbasis JWT (access token + refresh token), dilindungi rate limiting, dan dikendalikan oleh RBAC middleware dengan 9 role yang sudah didefinisikan di database.

## Glossary

- **Auth_Service**: Backend service layer yang menangani proses autentikasi, termasuk credential validation dan token issuance
- **Auth_Provider**: Interface abstraksi untuk autentikasi yang memiliki implementasi Local_Provider (password bcrypt) dan dapat diperluas ke LDAP_Provider di production
- **Local_Provider**: Implementasi Auth_Provider yang memvalidasi password menggunakan bcrypt hash dari database
- **Token_Service**: Komponen yang meng-generate, memvalidasi, dan me-refresh JWT access token dan refresh token
- **RBAC_Middleware**: HTTP middleware yang memverifikasi akses berdasarkan role user terhadap endpoint yang dilindungi
- **Login_Page**: Halaman frontend React untuk form login user
- **Rate_Limiter**: Middleware yang membatasi jumlah percobaan login per username/IP dalam window waktu tertentu
- **Access_Token**: JWT short-lived token yang digunakan untuk mengautentikasi setiap API request
- **Refresh_Token**: JWT long-lived token yang digunakan untuk mendapatkan Access_Token baru tanpa login ulang
- **CMS**: Cash Management System — aplikasi utama
- **Internal_User**: User karyawan CIMB Niaga (is_karyawan=true), saat ini login lokal dengan rencana LDAP di production
- **Vendor_User**: User vendor eksternal (is_karyawan=false, memiliki vendor_id), login lokal secara permanen
- **Auth_Source**: Kolom di tabel users yang menunjukkan mekanisme autentikasi ('local', 'local_dev', atau 'ldap')
- **Migration_Service**: Komponen database migration yang mengubah schema dan seed data
- **JTI**: JWT Token Identifier — unique ID per token untuk blacklisting
- **CompanyPortal**: Frontend app untuk user internal CIMB (React + Vite, di-deploy terpisah)
- **VendorPortal**: Frontend app untuk user vendor eksternal (React + Vite, di-deploy terpisah)
- **Portal_Type**: Identifier yang dikirim saat login untuk menentukan portal mana yang diakses ('company' atau 'vendor')

## Requirements

### Requirement 1: Database Migration untuk Development Auth

**User Story:** As a developer, I want all users to be able to login with local credentials during development, so that the team can develop and test the full login flow without LDAP infrastructure.

#### Acceptance Criteria

1. THE Migration_Service SHALL add a new auth_source value 'local_dev' to the users_auth_source_chk constraint
2. WHEN the migration runs, THE Migration_Service SHALL update all existing internal users (is_karyawan=true) to auth_source='local_dev' and populate password_hash with a bcrypt-hashed default development password
3. THE Migration_Service SHALL modify the users_vendor_logic_chk constraint to allow the combination (is_karyawan=true AND auth_source='local_dev' AND vendor_id IS NULL AND password_hash IS NOT NULL)
4. THE Migration_Service SHALL modify the users_password_hash_chk constraint to allow password_hash IS NOT NULL when auth_source='local_dev'
5. THE Migration_Service SHALL preserve the original constraint logic so that auth_source='ldap' users still require password_hash IS NULL
6. THE Migration_Service SHALL include a reversible down migration that restores auth_source='ldap', removes password_hash for internal users, and reinstates original constraints

---

### Requirement 2: Auth Provider Abstraction

**User Story:** As a developer, I want an authentication provider interface, so that LDAP can be plugged in for production without modifying the login handler or service layer.

#### Acceptance Criteria

1. THE Auth_Service SHALL define an Auth_Provider interface with a method that accepts username (string, 1–128 characters) and password (string, 1–128 characters) and returns the authenticated user's identity (user id, username, role, is_karyawan, vendor_id) or an error
2. THE Local_Provider SHALL implement Auth_Provider by comparing the provided password against the stored bcrypt password_hash in the database using constant-time comparison
3. THE Auth_Service SHALL select the Auth_Provider implementation based on the user's auth_source field: 'local' and 'local_dev' use Local_Provider, 'ldap' uses LDAP_Provider
4. IF auth_source is 'ldap' and no LDAP_Provider is configured, THEN THE Auth_Service SHALL return an error indicating LDAP authentication is not available in this environment
5. THE Auth_Provider interface SHALL be defined in a standalone Go package so that future LDAP_Provider implementations require no changes to handler or service code
6. IF a user's auth_source contains an unrecognized value (not 'local', 'local_dev', or 'ldap'), THEN THE Auth_Service SHALL return an error indicating an unsupported authentication source

---

### Requirement 3: Login Endpoint

**User Story:** As a CMS user (internal or vendor), I want to login with my username and password, so that I can access the system according to my role.

#### Acceptance Criteria

1. WHEN a user submits valid username and password to POST /api/v1/auth/login, THE Auth_Service SHALL authenticate the user via the appropriate Auth_Provider and return the Access_Token in the JSON response body and set the Refresh_Token as an httpOnly secure cookie with SameSite=Strict
2. WHEN authentication succeeds, THE Auth_Service SHALL update the user's last_login_at timestamp in the database
3. WHEN a user submits an invalid username, THE Auth_Service SHALL return a generic authentication error without revealing whether the username exists
4. WHEN a user submits a valid username but incorrect password, THE Auth_Service SHALL return the same generic authentication error as an invalid username
5. IF the user account has is_active=false, THEN THE Auth_Service SHALL return an error indicating the account is inactive
6. IF the user account has deleted_at not null, THEN THE Auth_Service SHALL return the same generic authentication error as an invalid username
7. THE Auth_Service SHALL validate that both username and password fields are non-empty and do not exceed 255 characters, and return a structured validation error identifying the failing field(s) before attempting authentication
8. THE Auth_Service SHALL evaluate login checks in the following order: input validation, user lookup (treating not-found and deleted_at as generic error), is_active check, then credential verification via Auth_Provider

---

### Requirement 4: JWT Token Management

**User Story:** As a CMS user, I want my session managed via JWT tokens, so that I can make authenticated API requests without re-entering credentials for each action.

#### Acceptance Criteria

1. THE Token_Service SHALL generate an Access_Token containing user id, username, role, is_karyawan, and vendor_id (if applicable) as claims
2. THE Token_Service SHALL set Access_Token expiry to 15 minutes
3. THE Token_Service SHALL generate a Refresh_Token containing user id and a unique JTI claim with expiry of 7 days
4. WHEN a valid Refresh_Token is presented to POST /api/v1/auth/refresh, THE Token_Service SHALL issue a new Access_Token and a new Refresh_Token (rotating the JTI)
5. WHEN an expired or invalid Refresh_Token is presented, THE Token_Service SHALL return an authentication error requiring re-login
6. THE Token_Service SHALL sign tokens using HMAC-SHA256 with a secret key loaded from environment configuration (minimum 32 bytes)
7. THE Token_Service SHALL include an issued-at (iat) claim in both tokens for auditing purposes

---

### Requirement 5: RBAC Middleware

**User Story:** As a system administrator, I want endpoint access controlled by user roles, so that each user can only access features appropriate to their role.

#### Acceptance Criteria

1. THE RBAC_Middleware SHALL extract and validate the Access_Token from the Authorization header (Bearer scheme) on every protected request, where validation includes verifying the HMAC-SHA256 signature, checking token expiry, and confirming required claims (id, username, role, is_karyawan, vendor_id) are present
2. WHEN the Access_Token is missing, malformed, has an invalid signature, or is expired, THE RBAC_Middleware SHALL return HTTP 401 Unauthorized with a JSON error response following the standard error format
3. WHEN the Access_Token is valid but the user's role is not in the endpoint's allowed roles list, THE RBAC_Middleware SHALL return HTTP 403 Forbidden with a JSON error response following the standard error format
4. THE RBAC_Middleware SHALL recognize all 9 roles defined in the database as valid role values: ADMIN, ADMIN_PARAM, ATM-USER, ATM-SPV, BRANCH-USER, BRANCH-SPV, BRANCH-ATM-USER, BRANCH-ATM-SPV, VENDOR-USER
5. THE RBAC_Middleware SHALL allow endpoint handlers to declare required roles via route configuration or decorator pattern
6. IF an endpoint does not declare any required roles, THEN THE RBAC_Middleware SHALL allow access to any authenticated user with a valid Access_Token regardless of role
7. THE RBAC_Middleware SHALL inject authenticated user context (id, username, role, is_karyawan, vendor_id) into the request context for downstream handlers, where vendor_id is null for non-vendor users

---

### Requirement 6: Password Hashing

**User Story:** As a security officer, I want passwords stored using industry-standard hashing, so that user credentials are protected even if the database is compromised.

#### Acceptance Criteria

1. THE Auth_Service SHALL hash passwords using bcrypt with a cost factor of 12
2. THE Auth_Service SHALL never store, log, or return plaintext passwords in any response body, application log, or database field other than the password_hash column
3. WHEN verifying a login attempt, THE Local_Provider SHALL use bcrypt's constant-time comparison to prevent timing attacks
4. WHEN a user changes their password, THE Auth_Service SHALL hash the new password using bcrypt with the current cost factor of 12 before storing it
5. THE Auth_Service SHALL enforce a minimum password length of 8 characters and a maximum password length of 72 bytes
6. IF a password hashing or comparison operation fails due to an internal error, THEN THE Auth_Service SHALL return a generic authentication error to the caller without exposing internal failure details

---

### Requirement 7: Login Rate Limiting

**User Story:** As a security officer, I want login attempts rate-limited, so that brute force attacks are mitigated.

#### Acceptance Criteria

1. THE Rate_Limiter SHALL limit failed login attempts to 5 per username within a 15-minute fixed window
2. THE Rate_Limiter SHALL limit failed login attempts to 20 per IP address within a 15-minute fixed window
3. WHEN the rate limit is exceeded for a username, THE Rate_Limiter SHALL return HTTP 429 Too Many Requests with a Retry-After header indicating remaining lockout time in seconds
4. WHEN the rate limit is exceeded for an IP address, THE Rate_Limiter SHALL return HTTP 429 Too Many Requests with a Retry-After header indicating remaining lockout time in seconds
5. WHEN a successful login occurs, THE Rate_Limiter SHALL reset the failed attempt counter for that username only, leaving the IP address counter unchanged
6. THE Rate_Limiter SHALL store attempt counters in Redis with TTL matching the rate limit window (900 seconds)
7. IF Redis is unavailable, THEN THE Rate_Limiter SHALL reject the login attempt with HTTP 503 Service Unavailable and log the connectivity failure
8. THE Rate_Limiter SHALL increment the failed attempt counter only for requests that result in authentication failure (invalid credentials or non-existent username)

---

### Requirement 8: Session Management

**User Story:** As a CMS user, I want my session managed properly including logout and automatic refresh, so that I have a secure and seamless experience.

#### Acceptance Criteria

1. WHEN a user calls the logout endpoint with a valid Refresh_Token (sent via httpOnly cookie), THE Auth_Service SHALL invalidate that Refresh_Token by adding its JTI to the Redis blacklist, clear the Refresh_Token cookie, and return HTTP 200 with an empty body
2. IF the Refresh_Token cookie is missing, expired, or malformed on the logout request, THEN THE Auth_Service SHALL still clear the Refresh_Token cookie and return HTTP 200 without error (idempotent logout)
3. THE Auth_Service SHALL maintain a token blacklist in Redis keyed by Refresh_Token JTI with TTL set to the token's remaining time until natural expiry
4. WHEN validating a Refresh_Token, THE Token_Service SHALL check the Redis blacklist and reject blacklisted tokens with the same authentication error as an expired token
5. IF Redis is unavailable during Refresh_Token blacklist validation, THEN THE Token_Service SHALL reject the refresh request and return an authentication error (fail-closed)
6. THE Auth_Service SHALL provide a GET /api/v1/auth/me endpoint that returns the current authenticated user's profile (id, username, full_name, email, role, is_karyawan, vendor_id) as a JSON object
7. WHEN the Access_Token expires, THE frontend client SHALL automatically attempt one token refresh using the Refresh_Token cookie before clearing auth state and redirecting to the Login_Page

---

### Requirement 9: Frontend Login Page

**User Story:** As a CMS user, I want a login page that follows the CIMB design system, so that I can securely enter my credentials with a professional and branded experience.

#### Acceptance Criteria

1. THE Login_Page SHALL display the CIMB Niaga branding (logo and primary red color as accent, following the 60/30/10 rule from the design system) on a centered card (--n-0 surface) over a --n-50 page background
2. THE Login_Page SHALL provide a username input field with a visible "Username" label and a password input field with a visible "Password" label, each with a placeholder matching the label text, and a maximum input length of 128 characters for username and 72 characters for password
3. THE Login_Page SHALL provide a submit button labeled "Login" styled as primary action (--red-500 background) that displays a loading indicator and is disabled while a login request is in progress
4. WHEN the login request fails with an authentication error, THE Login_Page SHALL display an error message below the form indicating that the credentials are invalid, without revealing whether the username or password was incorrect
5. WHEN the login request succeeds, THE Login_Page SHALL redirect the user to the originally requested protected page if one was stored before redirect, or to the default dashboard route (/dashboard) otherwise
6. THE Login_Page SHALL validate that both fields contain at least one non-whitespace character before enabling form submission, treating whitespace-only input as empty
7. THE Login_Page SHALL support form submission via Enter key press in any input field
8. WHEN the rate limit is exceeded (HTTP 429), THE Login_Page SHALL display a message indicating the remaining lockout duration in minutes and seconds, parsed from the Retry-After header value (in seconds), and disable the submit button until the lockout expires
9. THE Login_Page SHALL be accessible: all inputs have associated labels via htmlFor/id pairing, focus states use a 3px --red-100 box-shadow halo, and error messages are announced to screen readers via an aria-live="polite" region
10. IF the user is already authenticated when navigating to /login, THEN THE Login_Page SHALL redirect the user to the default dashboard route (/dashboard) without displaying the login form

---

### Requirement 10: Frontend Auth State Management

**User Story:** As a frontend developer, I want auth state managed centrally with automatic token refresh, so that protected routes and API calls work seamlessly.

#### Acceptance Criteria

1. THE frontend auth module SHALL store the Access_Token in memory (not localStorage) to mitigate XSS token theft
2. THE frontend auth module SHALL attach the Access_Token as Bearer token in the Authorization header for all API requests via a TanStack Query global config or fetch wrapper
3. WHEN an API request returns HTTP 401, THE frontend auth module SHALL attempt exactly one token refresh, queuing any concurrent 401 responses behind the same refresh call, and only redirect to the Login_Page if the single refresh attempt fails
4. WHEN token refresh fails, THE frontend auth module SHALL clear auth state and redirect the user to the Login_Page, preserving the originally requested URL as a query parameter so the user can be redirected back after re-login
5. THE frontend auth module SHALL expose an isAuthenticated boolean, an isAuthLoading boolean, and the current user profile (id, username, full_name, email, role, is_karyawan, vendor_id) for use by route guards and UI components
6. THE Auth_Service SHALL set the Refresh_Token as an httpOnly secure cookie with SameSite=Strict and a Max-Age matching the Refresh_Token expiry (7 days) so it persists across page reloads while remaining inaccessible to JavaScript
7. WHEN the application loads or the page is refreshed, THE frontend auth module SHALL attempt a token refresh using the persisted httpOnly cookie, exposing isAuthLoading=true until the refresh resolves, before route guards evaluate authentication state

---

### Requirement 11: Role-Based Frontend Navigation

**User Story:** As a CMS user, I want to see only the menu items and features relevant to my role, so that the interface is clean and I can find what I need quickly.

#### Acceptance Criteria

1. THE frontend route configuration SHALL declare an allowed roles list for each route/page as a centralized single source of truth for role-to-menu mapping, covering all 9 defined roles (ADMIN, ADMIN_PARAM, ATM-USER, ATM-SPV, BRANCH-USER, BRANCH-SPV, BRANCH-ATM-USER, BRANCH-ATM-SPV, VENDOR-USER)
2. WHEN the frontend renders navigation, THE frontend auth module SHALL read the user's role from the authenticated user context and render only the sidebar menu items whose route's allowed roles list includes the current user's role
3. WHEN a user manually navigates to a URL whose allowed roles list does not include the current user's role, THE frontend auth module SHALL display a 403 Forbidden page instead of the protected content and provide a navigation link back to the dashboard
4. WHERE the user's role is ADMIN or ADMIN_PARAM, THE frontend auth module SHALL render all menu items regardless of individual route role restrictions
5. WHERE the user's role is VENDOR-USER, THE frontend auth module SHALL render only vendor-specific menu items: DSR upload, invoice, and vendor dashboard
6. WHERE the user's role is ATM-USER, ATM-SPV, BRANCH-USER, BRANCH-SPV, BRANCH-ATM-USER, or BRANCH-ATM-SPV, THE frontend auth module SHALL render only the menu items explicitly assigned to that role in the centralized route configuration's allowed roles list — no menu item shall appear unless the role is present in that route's allowed roles array
7. IF the authenticated user's role does not match any entry in the route configuration's allowed roles lists, THEN THE frontend auth module SHALL render only the default dashboard menu item and display no additional navigation items
8. WHEN the authenticated user context becomes available after initial page load, THE frontend auth module SHALL render the role-filtered navigation within 1 render cycle (no intermediate state showing unfiltered or empty navigation to the user)

---

### Requirement 12: Vendor Portal Login

**User Story:** As a vendor user, I want to login through my own dedicated portal, so that my experience is tailored to vendor operations without seeing internal employee features.

#### Acceptance Criteria

1. THE VendorPortal SHALL provide its own /login page served from the VendorPortal-Vite application as a separate React app deployment
2. THE VendorPortal login page SHALL submit credentials to the same backend endpoint (POST /api/v1/auth/login) used by the CompanyPortal
3. THE login request from both portals SHALL include a Portal_Type identifier (via X-Portal-Type request header) with value 'company' for CompanyPortal or 'vendor' for VendorPortal
4. WHEN a user with is_karyawan=true authenticates via a request with Portal_Type='vendor', THE Auth_Service SHALL return HTTP 403 Forbidden with a JSON error response containing a distinct error type indicating this user cannot access the vendor portal
5. WHEN a user with is_karyawan=false authenticates via a request with Portal_Type='company', THE Auth_Service SHALL return HTTP 403 Forbidden with a JSON error response containing a distinct error type indicating this user cannot access the company portal
6. IF the X-Portal-Type header is missing or contains a value other than 'company' or 'vendor', THEN THE Auth_Service SHALL return a structured validation error identifying the invalid header before attempting authentication
7. THE VendorPortal login page SHALL display the CIMB Niaga branding with a "Vendor Portal" subtitle on a centered card (--n-0 surface) over a --n-50 page background with a --red-500 primary action submit button, and SHALL apply the same form validation rules (non-empty, whitespace-only treated as empty, maximum 128 characters for username and 72 characters for password), accessibility attributes (label-input pairing via htmlFor/id, 3px --red-100 focus halo, aria-live="polite" error region), and rate-limit handling (HTTP 429 lockout display with countdown parsed from Retry-After header) as defined in Requirement 9
8. WHEN vendor login succeeds, THE VendorPortal SHALL redirect the user to the originally requested protected route if one was stored before redirect, or to the default vendor dashboard route (/dashboard) otherwise
9. THE VendorPortal auth state management SHALL follow the same patterns as defined in Requirement 10: Access_Token stored in memory, Refresh_Token as httpOnly secure cookie with SameSite=Strict, automatic token refresh on HTTP 401, and isAuthLoading guard on page load
10. THE Auth_Service SHALL evaluate Portal_Type restriction after credential verification succeeds but before issuing tokens, so that the generic authentication error is returned for invalid credentials regardless of Portal_Type mismatch
11. WHEN the login request fails with a portal mismatch error (HTTP 403), THE VendorPortal login page SHALL display an error message indicating the user account is not authorized for this portal, distinct from the generic invalid credentials message
