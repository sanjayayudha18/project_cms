package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
)

// --- Stub implementations for service tests ---

// stubProvider implements pkgauth.Provider for service tests.
type stubProvider struct {
	supportsFunc    func(authSource string) bool
	authenticateErr error
	authenticateID  *pkgauth.AuthIdentity
}

func (m *stubProvider) Supports(authSource string) bool {
	if m.supportsFunc != nil {
		return m.supportsFunc(authSource)
	}
	return authSource == "local" || authSource == "local_dev"
}

func (m *stubProvider) Authenticate(_ context.Context, _, _ string) (*pkgauth.AuthIdentity, error) {
	if m.authenticateErr != nil {
		return nil, m.authenticateErr
	}
	return m.authenticateID, nil
}

// stubUserRepo implements pkgauth.UserRepository for service tests.
type stubUserRepo struct {
	findResult       *pkgauth.UserRecord
	findErr          error
	updateLoginErr   error
	getProfileResult *pkgauth.UserRecord
	getProfileErr    error
}

func (m *stubUserRepo) FindByUsername(_ context.Context, _ string) (*pkgauth.UserRecord, error) {
	return m.findResult, m.findErr
}

func (m *stubUserRepo) UpdateLastLogin(_ context.Context, _ int64) error {
	return m.updateLoginErr
}

func (m *stubUserRepo) GetUserProfile(_ context.Context, _ int64) (*pkgauth.UserRecord, error) {
	return m.getProfileResult, m.getProfileErr
}

// stubRateLimiter implements pkgauth.RateLimiter for service tests.
type stubRateLimiter struct {
	checkErr          error
	incrementFailedFn func(ctx context.Context, username, ip string) error
	resetUsernameFn   func(ctx context.Context, username string) error
}

func (m *stubRateLimiter) Check(_ context.Context, _, _ string) error {
	return m.checkErr
}

func (m *stubRateLimiter) IncrementFailed(ctx context.Context, username, ip string) error {
	if m.incrementFailedFn != nil {
		return m.incrementFailedFn(ctx, username, ip)
	}
	return nil
}

func (m *stubRateLimiter) ResetUsername(ctx context.Context, username string) error {
	if m.resetUsernameFn != nil {
		return m.resetUsernameFn(ctx, username)
	}
	return nil
}

// --- Test helpers ---

// testTokenConfig returns a TokenConfig suitable for unit tests.
func testTokenConfig() pkgauth.TokenConfig {
	return pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-key-minimum-32-bytes!"),
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	}
}

// noopBlacklist is a TokenBlacklist stub that never blacklists anything.
type noopBlacklist struct{}

func (noopBlacklist) Add(_ context.Context, _ string, _ time.Duration) error { return nil }
func (noopBlacklist) IsBlacklisted(_ context.Context, _ string) (bool, error) {
	return false, nil
}

// newServiceUnderTest creates a Service wired with stubs for unit testing.
func newServiceUnderTest(provider *stubProvider, repo *stubUserRepo, rl *stubRateLimiter) *Service {
	tokenSvc := pkgauth.NewTokenService(testTokenConfig(), noopBlacklist{})
	return NewService([]pkgauth.Provider{provider}, tokenSvc, repo, rl)
}

// activeKaryawanUser returns a typical active karyawan user record for testing.
func activeKaryawanUser() *pkgauth.UserRecord {
	return &pkgauth.UserRecord{
		ID:           1,
		Username:     "john.admin",
		FullName:     "John Admin",
		Email:        "john.admin@crown.local",
		PasswordHash: strPtr("$2a$12$hashed"),
		AuthSource:   "local_dev",
		RoleID:       1,
		Role:         "ADMIN",
		IsKaryawan:   true,
		VendorID:     nil,
		IsActive:     true,
		DeletedAt:    nil,
	}
}

func strPtr(s string) *string { return &s }

// --- Unit tests ---

func TestService_Login_Success(t *testing.T) {
	user := activeKaryawanUser()

	provider := &stubProvider{
		authenticateID: &pkgauth.AuthIdentity{
			UserID:     user.ID,
			Username:   user.Username,
			Role:       user.Role,
			IsKaryawan: user.IsKaryawan,
			VendorID:   user.VendorID,
		},
	}
	repo := &stubUserRepo{findResult: user}
	rl := &stubRateLimiter{}

	svc := newServiceUnderTest(provider, repo, rl)

	resp, refreshToken, err := svc.Login(context.Background(), LoginRequest{
		Username:   "john.admin",
		Password:   "Password123!",
		PortalType: "company",
		IP:         "127.0.0.1",
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil LoginResponse")
	}
	if resp.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
	if refreshToken == "" {
		t.Error("expected non-empty refresh token")
	}

	// Verify user profile in response
	if resp.User.ID != 1 {
		t.Errorf("expected User.ID=1, got %d", resp.User.ID)
	}
	if resp.User.Username != "john.admin" {
		t.Errorf("expected User.Username=john.admin, got %s", resp.User.Username)
	}
	if resp.User.FullName != "John Admin" {
		t.Errorf("expected User.FullName=John Admin, got %s", resp.User.FullName)
	}
	if resp.User.Email != "john.admin@crown.local" {
		t.Errorf("expected User.Email=john.admin@crown.local, got %s", resp.User.Email)
	}
	if resp.User.Role != "ADMIN" {
		t.Errorf("expected User.Role=ADMIN, got %s", resp.User.Role)
	}
	if !resp.User.IsKaryawan {
		t.Error("expected User.IsKaryawan=true")
	}
	if resp.User.VendorID != nil {
		t.Errorf("expected User.VendorID=nil, got %v", resp.User.VendorID)
	}
}

func TestService_Login_InvalidCredentials(t *testing.T) {
	user := activeKaryawanUser()

	provider := &stubProvider{
		authenticateErr: pkgauth.ErrInvalidCredentials,
	}
	repo := &stubUserRepo{findResult: user}
	rl := &stubRateLimiter{}

	svc := newServiceUnderTest(provider, repo, rl)

	resp, refreshToken, err := svc.Login(context.Background(), LoginRequest{
		Username:   "john.admin",
		Password:   "WrongPassword!",
		PortalType: "company",
		IP:         "127.0.0.1",
	})

	if resp != nil {
		t.Error("expected nil response for invalid credentials")
	}
	if refreshToken != "" {
		t.Error("expected empty refresh token for invalid credentials")
	}
	if !errors.Is(err, pkgauth.ErrInvalidCredentials) {
		t.Errorf("expected pkgauth.ErrInvalidCredentials, got: %v", err)
	}
}

func TestService_Login_UserNotFound_GenericError(t *testing.T) {
	// When repo returns nil, nil (user not found), should get generic error
	provider := &stubProvider{}
	repo := &stubUserRepo{
		findResult: nil,
		findErr:    nil,
	}
	rl := &stubRateLimiter{}

	svc := newServiceUnderTest(provider, repo, rl)

	resp, refreshToken, err := svc.Login(context.Background(), LoginRequest{
		Username:   "nonexistent",
		Password:   "Password123!",
		PortalType: "company",
		IP:         "127.0.0.1",
	})

	if resp != nil {
		t.Error("expected nil response for user not found")
	}
	if refreshToken != "" {
		t.Error("expected empty refresh token for user not found")
	}
	if !errors.Is(err, pkgauth.ErrInvalidCredentials) {
		t.Errorf("expected pkgauth.ErrInvalidCredentials (generic error), got: %v", err)
	}
}

func TestService_Login_DeletedUser_GenericError(t *testing.T) {
	// A user with deleted_at set should return the same generic error
	deletedAt := time.Now().Add(-24 * time.Hour)
	user := activeKaryawanUser()
	user.DeletedAt = &deletedAt

	provider := &stubProvider{}
	repo := &stubUserRepo{findResult: user}
	rl := &stubRateLimiter{}

	svc := newServiceUnderTest(provider, repo, rl)

	resp, refreshToken, err := svc.Login(context.Background(), LoginRequest{
		Username:   "john.admin",
		Password:   "Password123!",
		PortalType: "company",
		IP:         "127.0.0.1",
	})

	if resp != nil {
		t.Error("expected nil response for deleted user")
	}
	if refreshToken != "" {
		t.Error("expected empty refresh token for deleted user")
	}
	if !errors.Is(err, pkgauth.ErrInvalidCredentials) {
		t.Errorf("expected pkgauth.ErrInvalidCredentials (generic error), got: %v", err)
	}
}

func TestService_Login_InactiveUser(t *testing.T) {
	user := activeKaryawanUser()
	user.IsActive = false

	provider := &stubProvider{}
	repo := &stubUserRepo{findResult: user}
	rl := &stubRateLimiter{}

	svc := newServiceUnderTest(provider, repo, rl)

	resp, refreshToken, err := svc.Login(context.Background(), LoginRequest{
		Username:   "john.admin",
		Password:   "Password123!",
		PortalType: "company",
		IP:         "127.0.0.1",
	})

	if resp != nil {
		t.Error("expected nil response for inactive user")
	}
	if refreshToken != "" {
		t.Error("expected empty refresh token for inactive user")
	}
	if !errors.Is(err, pkgauth.ErrAccountInactive) {
		t.Errorf("expected pkgauth.ErrAccountInactive, got: %v", err)
	}
}

func TestService_Login_PortalMismatch(t *testing.T) {
	// Karyawan user accessing vendor portal
	user := activeKaryawanUser()

	provider := &stubProvider{
		authenticateID: &pkgauth.AuthIdentity{
			UserID:     user.ID,
			Username:   user.Username,
			Role:       user.Role,
			IsKaryawan: user.IsKaryawan,
			VendorID:   user.VendorID,
		},
	}
	repo := &stubUserRepo{findResult: user}
	rl := &stubRateLimiter{}

	svc := newServiceUnderTest(provider, repo, rl)

	resp, refreshToken, err := svc.Login(context.Background(), LoginRequest{
		Username:   "john.admin",
		Password:   "Password123!",
		PortalType: "vendor", // mismatch: karyawan accessing vendor portal
		IP:         "127.0.0.1",
	})

	if resp != nil {
		t.Error("expected nil response for portal mismatch")
	}
	if refreshToken != "" {
		t.Error("expected empty refresh token for portal mismatch")
	}
	if !errors.Is(err, pkgauth.ErrPortalMismatch) {
		t.Errorf("expected pkgauth.ErrPortalMismatch, got: %v", err)
	}
}

func TestService_Login_ValidationBeforeAuth(t *testing.T) {
	// Empty username should return pkgauth.ValidationError before any auth logic runs.
	provider := &stubProvider{
		authenticateErr: errors.New("should not be called"),
	}
	repo := &stubUserRepo{
		findErr: errors.New("should not be called"),
	}
	rl := &stubRateLimiter{
		checkErr: errors.New("should not be called"),
	}

	svc := newServiceUnderTest(provider, repo, rl)

	resp, refreshToken, err := svc.Login(context.Background(), LoginRequest{
		Username:   "",
		Password:   "Password123!",
		PortalType: "company",
		IP:         "127.0.0.1",
	})

	if resp != nil {
		t.Error("expected nil response for validation error")
	}
	if refreshToken != "" {
		t.Error("expected empty refresh token for validation error")
	}
	if err == nil {
		t.Fatal("expected an error for empty username")
	}

	var validationErr *pkgauth.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected *pkgauth.ValidationError, got: %T (%v)", err, err)
	}
	if validationErr.Field != "username" {
		t.Errorf("expected Field=username, got %s", validationErr.Field)
	}
}

func TestService_Login_RateLimitBlocks(t *testing.T) {
	// Rate limiter returns error - propagated to caller
	rateLimitErr := &pkgauth.RateLimitError{RetryAfter: 540}

	provider := &stubProvider{}
	repo := &stubUserRepo{}
	rl := &stubRateLimiter{checkErr: rateLimitErr}

	svc := newServiceUnderTest(provider, repo, rl)

	resp, refreshToken, err := svc.Login(context.Background(), LoginRequest{
		Username:   "john.admin",
		Password:   "Password123!",
		PortalType: "company",
		IP:         "127.0.0.1",
	})

	if resp != nil {
		t.Error("expected nil response for rate limited request")
	}
	if refreshToken != "" {
		t.Error("expected empty refresh token for rate limited request")
	}
	if err == nil {
		t.Fatal("expected an error for rate limited request")
	}

	var rlErr *pkgauth.RateLimitError
	if !errors.As(err, &rlErr) {
		t.Fatalf("expected *pkgauth.RateLimitError, got: %T (%v)", err, err)
	}
	if rlErr.RetryAfter != 540 {
		t.Errorf("expected RetryAfter=540, got %d", rlErr.RetryAfter)
	}
}
