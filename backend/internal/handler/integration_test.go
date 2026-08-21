//go:build integration

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/cimb-niaga/cms/backend/internal/auth"
	"github.com/cimb-niaga/cms/backend/internal/middleware"
	"github.com/cimb-niaga/cms/backend/internal/repository"
)

// ─── Constants ────────────────────────────────────────────────────────────────

const (
	testJWTSecret   = "integration-test-secret-key-min-32-bytes!"
	testDevPassword = "password123"
	companyPortal   = "company"
	vendorPortal    = "vendor"

	// testCompanyUser and testVendorUser are seeded by 003_seed_roles_users.sql:
	// testCompanyUser has role ADMIN, is_karyawan=true (company portal).
	// testVendorUser has role VENDOR-USER, is_karyawan=false, vendor_id set (vendor portal).
	// Both authenticate with testDevPassword — 003_seed_roles_users.sql's
	// password_hash values are a verified bcrypt hash of "password123".
	testCompanyUser = "Yudha"
	testVendorUser  = "vendor.ssi"
)

// ─── Test Harness ─────────────────────────────────────────────────────────────

// testHarness holds all dependencies needed for integration tests.
type testHarness struct {
	router      http.Handler
	pool        *pgxpool.Pool
	miniRedis   *miniredis.Miniredis
	redisClient *redis.Client
	tokenSvc    *auth.TokenService
}

// setupHarness creates a full integration test stack:
// real PostgreSQL (from DATABASE_URL env) + miniredis for Redis.
func setupHarness(t *testing.T) *testHarness {
	t.Helper()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}

	ctx := context.Background()

	// Connect to PostgreSQL
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connecting to database: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("pinging database: %v", err)
	}

	// Run migrations against the test DB
	runMigrations(t, pool)

	// Start miniredis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("starting miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { redisClient.Close() })

	// Build the full dependency graph
	repo := repository.NewAuthRepository(pool)
	blacklist := auth.NewRedisTokenBlacklist(redisClient)

	tokenCfg := auth.TokenConfig{
		SecretKey:          []byte(testJWTSecret),
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	}
	tokenSvc := auth.NewTokenService(tokenCfg, blacklist)

	localProvider := auth.NewLocalProvider(repo)

	rateLimiterCfg := middleware.RateLimitConfig{
		MaxPerUsername: 5,
		MaxPerIP:       20,
		Window:         15 * time.Minute,
	}
	rateLimiter := middleware.NewRateLimiter(redisClient, rateLimiterCfg)

	authSvc := auth.NewService(
		[]auth.Provider{localProvider},
		tokenSvc,
		repo,
		rateLimiter,
	)

	authHandler := NewAuthHandler(authSvc, tokenSvc, repo, rateLimiter)

	// Build router
	r := chi.NewRouter()
	r.Mount("/api/v1/auth", authHandler.Routes())

	return &testHarness{
		router:      r,
		pool:        pool,
		miniRedis:   mr,
		redisClient: redisClient,
		tokenSvc:    tokenSvc,
	}
}

// runMigrations executes the SQL migration files against the test database,
// skipping entirely if the schema is already present. DATABASE_URL is
// expected to point at the shared `cms` database (see docker-compose.yml),
// which already carries the full migrated schema and seed data. Re-running
// the migrations against it is not safe: ALTER TABLE ... ADD CONSTRAINT has
// no IF NOT EXISTS form in Postgres, so a second run fails on duplicate
// constraint errors even though the CREATE TABLE / INSERT statements
// themselves are idempotent.
func runMigrations(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	var schemaExists bool
	err := pool.QueryRow(ctx, `SELECT to_regclass('public.roles') IS NOT NULL`).Scan(&schemaExists)
	if err != nil {
		t.Fatalf("checking existing schema: %v", err)
	}
	if schemaExists {
		return
	}

	// Order matters: 001.1_vendor_vaults.sql adds an FK to vendor_branches,
	// which is created in 002_cms_tables.sql, so it must run after it.
	// 001_create_db_cms.sql is excluded — it issues CREATE DATABASE, which
	// cannot run from a pool already connected to that database.
	migrations := []string{
		"../../migrations/002_cms_tables.sql",
		"../../migrations/001.1_vendor_vaults.sql",
		"../../migrations/003_seed_roles_users.sql",
		"../../migrations/004_seed_regions_locations.sql",
		"../../migrations/005_seed_vendors.sql",
		"../../migrations/006_seed_vendor_branches_fixed.sql",
		"../../migrations/007_seed_vendor_vaults_hardened.sql",
		"../../migrations/008_seed_atms.sql",
		"../../migrations/009_itm_cashpos.sql",
	}

	for _, path := range migrations {
		sql, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading migration %s: %v", path, err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("running migration %s: %v", path, err)
		}
	}
}

// ─── HTTP Helpers ─────────────────────────────────────────────────────────────

func doLogin(h http.Handler, username, password, portal string) *httptest.ResponseRecorder {
	body := fmt.Sprintf(`{"username":%q,"password":%q}`, username, password)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Portal-Type", portal)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func doRefresh(h http.Handler, refreshCookie string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.Header.Set("Cookie", "refresh_token="+refreshCookie)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func doLogout(h http.Handler, refreshCookie string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.Header.Set("Cookie", "refresh_token="+refreshCookie)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func extractRefreshCookie(w *httptest.ResponseRecorder) string {
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "refresh_token" {
			return cookie.Value
		}
	}
	return ""
}

type loginResponseBody struct {
	AccessToken string `json:"access_token"`
	User        struct {
		ID         int64  `json:"id"`
		Username   string `json:"username"`
		FullName   string `json:"full_name"`
		Email      string `json:"email"`
		Role       string `json:"role"`
		IsKaryawan bool   `json:"is_karyawan"`
		VendorID   *int64 `json:"vendor_id"`
	} `json:"user"`
}

type errorResponseBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// ─── Task 16.1 & 16.2: Full Login Flow Integration Tests ─────────────────────

func TestIntegration_CompanyLogin_Success(t *testing.T) {
	h := setupHarness(t)

	// testCompanyUser is an internal user (is_karyawan=true), role ADMIN
	w := doLogin(h.router, testCompanyUser, testDevPassword, companyPortal)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp loginResponseBody
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	if resp.AccessToken == "" {
		t.Error("access_token should not be empty")
	}
	if resp.User.Username != testCompanyUser {
		t.Errorf("username = %q, want %q", resp.User.Username, testCompanyUser)
	}
	if resp.User.Role != "ADMIN" {
		t.Errorf("role = %q, want ADMIN", resp.User.Role)
	}
	if !resp.User.IsKaryawan {
		t.Error("is_karyawan should be true for internal user")
	}

	// Verify refresh token cookie is set
	cookie := extractRefreshCookie(w)
	if cookie == "" {
		t.Error("refresh_token cookie should be set")
	}
}

func TestIntegration_VendorLogin_Success(t *testing.T) {
	h := setupHarness(t)

	// testVendorUser is a vendor user (is_karyawan=false, vendor_id set), role VENDOR-USER
	w := doLogin(h.router, testVendorUser, testDevPassword, vendorPortal)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp loginResponseBody
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	if resp.User.Username != testVendorUser {
		t.Errorf("username = %q, want %q", resp.User.Username, testVendorUser)
	}
	if resp.User.Role != "VENDOR-USER" {
		t.Errorf("role = %q, want VENDOR-USER", resp.User.Role)
	}
	if resp.User.IsKaryawan {
		t.Error("is_karyawan should be false for vendor user")
	}
	if resp.User.VendorID == nil {
		t.Error("vendor_id should not be nil for vendor user")
	}
}

func TestIntegration_PortalMismatch_InternalOnVendor(t *testing.T) {
	h := setupHarness(t)

	// Internal user trying to login via vendor portal
	w := doLogin(h.router, testCompanyUser, testDevPassword, vendorPortal)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}

	var resp errorResponseBody
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error != "portal_mismatch" {
		t.Errorf("error = %q, want portal_mismatch", resp.Error)
	}
}

func TestIntegration_PortalMismatch_VendorOnCompany(t *testing.T) {
	h := setupHarness(t)

	// Vendor user trying to login via company portal
	w := doLogin(h.router, testVendorUser, testDevPassword, companyPortal)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}

	var resp errorResponseBody
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error != "portal_mismatch" {
		t.Errorf("error = %q, want portal_mismatch", resp.Error)
	}
}

func TestIntegration_RateLimitEnforcement(t *testing.T) {
	h := setupHarness(t)

	// Exhaust 5 failed attempts for a username
	for i := 0; i < 5; i++ {
		w := doLogin(h.router, testCompanyUser, "wrong-password", companyPortal)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401, got %d", i+1, w.Code)
		}
	}

	// 6th attempt should be rate limited
	w := doLogin(h.router, testCompanyUser, "wrong-password", companyPortal)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d: %s", w.Code, w.Body.String())
	}

	// Retry-After header should be present
	retryAfter := w.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Error("Retry-After header should be set on 429")
	}

	// Even correct password should be blocked
	w = doLogin(h.router, testCompanyUser, testDevPassword, companyPortal)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 even with correct pw, got %d", w.Code)
	}
}

func TestIntegration_TokenRefresh(t *testing.T) {
	h := setupHarness(t)

	// Login first to get a refresh token
	w := doLogin(h.router, testCompanyUser, testDevPassword, companyPortal)
	if w.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", w.Code, w.Body.String())
	}

	refreshCookie := extractRefreshCookie(w)
	if refreshCookie == "" {
		t.Fatal("no refresh_token cookie from login")
	}

	// Use the refresh token to get a new access token
	w = doRefresh(h.router, refreshCookie)
	if w.Code != http.StatusOK {
		t.Fatalf("refresh failed: %d %s", w.Code, w.Body.String())
	}

	var resp loginResponseBody
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.AccessToken == "" {
		t.Error("refreshed access_token should not be empty")
	}
	if resp.User.Username != testCompanyUser {
		t.Errorf("username = %q, want %q", resp.User.Username, testCompanyUser)
	}

	// New refresh cookie should be issued (rotation)
	newCookie := extractRefreshCookie(w)
	if newCookie == "" {
		t.Error("new refresh_token cookie should be set after refresh")
	}
	if newCookie == refreshCookie {
		t.Error("refresh token should be rotated (new != old)")
	}
}

func TestIntegration_Logout_BlacklistsJTI(t *testing.T) {
	h := setupHarness(t)

	// Login
	w := doLogin(h.router, testCompanyUser, testDevPassword, companyPortal)
	if w.Code != http.StatusOK {
		t.Fatalf("login failed: %d", w.Code)
	}
	refreshCookie := extractRefreshCookie(w)

	// Logout should succeed
	w = doLogout(h.router, refreshCookie)
	if w.Code != http.StatusOK {
		t.Fatalf("logout failed: %d %s", w.Code, w.Body.String())
	}

	// Verify refresh cookie is cleared (MaxAge < 0)
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "refresh_token" && cookie.MaxAge > 0 {
			t.Error("refresh_token cookie should be cleared after logout")
		}
	}
}

func TestIntegration_RefreshAfterLogout_Fails(t *testing.T) {
	h := setupHarness(t)

	// Login
	w := doLogin(h.router, testCompanyUser, testDevPassword, companyPortal)
	if w.Code != http.StatusOK {
		t.Fatalf("login failed: %d", w.Code)
	}
	refreshCookie := extractRefreshCookie(w)

	// Logout (blacklists the JTI)
	w = doLogout(h.router, refreshCookie)
	if w.Code != http.StatusOK {
		t.Fatalf("logout failed: %d", w.Code)
	}

	// Attempt refresh with the now-blacklisted token
	w = doRefresh(h.router, refreshCookie)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout, got %d: %s",
			w.Code, w.Body.String())
	}
}

// ─── Task 16.3: Redis Failure Scenario ────────────────────────────────────────

func TestIntegration_RedisUnavailable_Returns503(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connecting to database: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	runMigrations(t, pool)

	// Start miniredis then immediately close it to simulate unavailability
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("starting miniredis: %v", err)
	}
	addr := mr.Addr()
	mr.Close() // Redis is now unreachable

	redisClient := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { redisClient.Close() })

	// Build dependencies with dead Redis
	repo := repository.NewAuthRepository(pool)
	blacklist := auth.NewRedisTokenBlacklist(redisClient)

	tokenCfg := auth.TokenConfig{
		SecretKey:          []byte(testJWTSecret),
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	}
	tokenSvc := auth.NewTokenService(tokenCfg, blacklist)
	localProvider := auth.NewLocalProvider(repo)

	rateLimiterCfg := middleware.RateLimitConfig{
		MaxPerUsername: 5,
		MaxPerIP:       20,
		Window:         15 * time.Minute,
	}
	rateLimiter := middleware.NewRateLimiter(redisClient, rateLimiterCfg)

	authSvc := auth.NewService(
		[]auth.Provider{localProvider},
		tokenSvc, repo, rateLimiter,
	)

	authHandler := NewAuthHandler(authSvc, tokenSvc, repo, rateLimiter)

	r := chi.NewRouter()
	r.Mount("/api/v1/auth", authHandler.Routes())

	// Attempt login — should get 503 because Redis is unreachable (fail-closed)
	w := doLogin(r, testCompanyUser, testDevPassword, companyPortal)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when Redis down, got %d: %s",
			w.Code, w.Body.String())
	}

	var resp errorResponseBody
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error != "service_unavailable" {
		t.Errorf("error = %q, want service_unavailable", resp.Error)
	}
}

