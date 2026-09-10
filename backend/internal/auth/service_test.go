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
	findResult        *pkgauth.UserRecord
	findErr           error
	updateLoginErr    error
	getProfileResult  *pkgauth.UserRecord
	getProfileErr     error
	markExpiredErr    error
	markExpiredCalled bool
	markExpiredUserID int64

	incrementFailedLoginResult int32
	incrementFailedLoginErr    error
	incrementFailedLoginCalled bool

	lockAccountErr     error
	lockAccountCalled  bool
	lockAccountUserID  int64
	lockAccountUntil   time.Time

	resetLockoutErr    error
	resetLockoutCalled bool

	findByIDResult *pkgauth.UserRecord
	findByIDErr    error

	setPasswordErr    error
	setPasswordCalled bool

	setInitialPasswordErr    error
	setInitialPasswordCalled bool
	setInitialPasswordUserID int64

	deactivateErr    error
	deactivateCalled bool
	deactivateUserID int64

	reactivateErr    error
	reactivateCalled bool
	reactivateUserID int64
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

func (m *stubUserRepo) MarkPasswordExpired(_ context.Context, userID int64) error {
	m.markExpiredCalled = true
	m.markExpiredUserID = userID
	return m.markExpiredErr
}

func (m *stubUserRepo) IncrementFailedLogin(_ context.Context, _ int64) (int32, error) {
	m.incrementFailedLoginCalled = true
	return m.incrementFailedLoginResult, m.incrementFailedLoginErr
}

func (m *stubUserRepo) LockAccount(_ context.Context, userID int64, until time.Time) error {
	m.lockAccountCalled = true
	m.lockAccountUserID = userID
	m.lockAccountUntil = until
	return m.lockAccountErr
}

func (m *stubUserRepo) ResetLockout(_ context.Context, _ int64) error {
	m.resetLockoutCalled = true
	return m.resetLockoutErr
}

func (m *stubUserRepo) FindByID(_ context.Context, _ int64) (*pkgauth.UserRecord, error) {
	return m.findByIDResult, m.findByIDErr
}

func (m *stubUserRepo) SetPassword(_ context.Context, _ int64, _ string) error {
	m.setPasswordCalled = true
	return m.setPasswordErr
}

func (m *stubUserRepo) SetInitialPassword(_ context.Context, userID int64, _ string) error {
	m.setInitialPasswordCalled = true
	m.setInitialPasswordUserID = userID
	return m.setInitialPasswordErr
}

func (m *stubUserRepo) Deactivate(_ context.Context, userID int64) error {
	m.deactivateCalled = true
	m.deactivateUserID = userID
	return m.deactivateErr
}

func (m *stubUserRepo) Reactivate(_ context.Context, userID int64) error {
	m.reactivateCalled = true
	m.reactivateUserID = userID
	return m.reactivateErr
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

func TestService_Login_LocalPassword_Expired_Rejected(t *testing.T) {
	changedAt := time.Now().Add(-91 * 24 * time.Hour)
	user := activeKaryawanUser()
	user.PasswordChangedAt = &changedAt

	provider := &stubProvider{
		authenticateID: &pkgauth.AuthIdentity{UserID: user.ID, Username: user.Username, Role: user.Role, IsKaryawan: user.IsKaryawan},
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

	if resp != nil {
		t.Error("expected nil response for expired password")
	}
	if refreshToken != "" {
		t.Error("expected empty refresh token for expired password")
	}
	if !errors.Is(err, pkgauth.ErrPasswordExpired) {
		t.Errorf("expected pkgauth.ErrPasswordExpired, got: %v", err)
	}
	if !repo.markExpiredCalled {
		t.Error("expected MarkPasswordExpired to be called")
	}
	if repo.markExpiredUserID != user.ID {
		t.Errorf("expected MarkPasswordExpired called with userID=%d, got %d", user.ID, repo.markExpiredUserID)
	}
}

func TestService_Login_LocalPassword_WarningWindow_IncludesDaysLeft(t *testing.T) {
	changedAt := time.Now().Add(-83 * 24 * time.Hour) // 7 days left
	user := activeKaryawanUser()
	user.PasswordChangedAt = &changedAt

	provider := &stubProvider{
		authenticateID: &pkgauth.AuthIdentity{UserID: user.ID, Username: user.Username, Role: user.Role, IsKaryawan: user.IsKaryawan},
	}
	repo := &stubUserRepo{findResult: user}
	rl := &stubRateLimiter{}

	svc := newServiceUnderTest(provider, repo, rl)

	resp, _, err := svc.Login(context.Background(), LoginRequest{
		Username:   "john.admin",
		Password:   "Password123!",
		PortalType: "company",
		IP:         "127.0.0.1",
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resp.PasswordDaysLeft == nil {
		t.Fatal("expected PasswordDaysLeft to be set in warning window")
	}
	if *resp.PasswordDaysLeft != 7 {
		t.Errorf("expected PasswordDaysLeft=7, got %d", *resp.PasswordDaysLeft)
	}
	if repo.markExpiredCalled {
		t.Error("expected MarkPasswordExpired NOT to be called when only in warning window")
	}
}

func TestService_Login_LDAPAccount_SkipsPasswordExpiryPolicy(t *testing.T) {
	// An LDAP account with a very old password_changed_at (would be "expired"
	// under the local policy) must never be rejected — LDAP/Entra owns its
	// own password policy.
	changedAt := time.Now().Add(-9999 * 24 * time.Hour)
	user := activeKaryawanUser()
	user.AuthSource = "ldap"
	user.PasswordChangedAt = &changedAt

	provider := &stubProvider{
		supportsFunc:   func(authSource string) bool { return authSource == "ldap" },
		authenticateID: &pkgauth.AuthIdentity{UserID: user.ID, Username: user.Username, Role: user.Role, IsKaryawan: user.IsKaryawan},
	}
	repo := &stubUserRepo{findResult: user}
	rl := &stubRateLimiter{}

	svc := newServiceUnderTest(provider, repo, rl)

	resp, _, err := svc.Login(context.Background(), LoginRequest{
		Username:   "john.admin",
		Password:   "Password123!",
		PortalType: "company",
		IP:         "127.0.0.1",
	})

	if err != nil {
		t.Fatalf("expected no error (policy skipped for ldap), got: %v", err)
	}
	if resp.PasswordDaysLeft != nil {
		t.Errorf("expected PasswordDaysLeft=nil for ldap account, got %v", *resp.PasswordDaysLeft)
	}
	if repo.markExpiredCalled {
		t.Error("expected MarkPasswordExpired NOT to be called for ldap account")
	}
}

func TestService_Login_LocalPassword_FailedAttempts_BelowThreshold_NotLocked(t *testing.T) {
	// 2 failed attempts (new count returned by IncrementFailedLogin) must not
	// trigger a lock — the threshold is 3.
	user := activeKaryawanUser()

	provider := &stubProvider{authenticateErr: pkgauth.ErrInvalidCredentials}
	repo := &stubUserRepo{findResult: user, incrementFailedLoginResult: 2}
	rl := &stubRateLimiter{}

	svc := newServiceUnderTest(provider, repo, rl)

	_, _, err := svc.Login(context.Background(), LoginRequest{
		Username:   "john.admin",
		Password:   "WrongPassword!",
		PortalType: "company",
		IP:         "127.0.0.1",
	})

	if !errors.Is(err, pkgauth.ErrInvalidCredentials) {
		t.Errorf("expected pkgauth.ErrInvalidCredentials, got: %v", err)
	}
	if !repo.incrementFailedLoginCalled {
		t.Error("expected IncrementFailedLogin to be called")
	}
	if repo.lockAccountCalled {
		t.Error("expected LockAccount NOT to be called at 2 failed attempts")
	}
}

func TestService_Login_LocalPassword_FailedAttempts_ReachesThreshold_Locked(t *testing.T) {
	// The 3rd failed attempt (new count = 3) must lock the account for
	// LockoutDuration (30 minutes).
	user := activeKaryawanUser()

	provider := &stubProvider{authenticateErr: pkgauth.ErrInvalidCredentials}
	repo := &stubUserRepo{findResult: user, incrementFailedLoginResult: pkgauth.MaxFailedLogins}
	rl := &stubRateLimiter{}

	svc := newServiceUnderTest(provider, repo, rl)

	before := time.Now()
	_, _, err := svc.Login(context.Background(), LoginRequest{
		Username:   "john.admin",
		Password:   "WrongPassword!",
		PortalType: "company",
		IP:         "127.0.0.1",
	})
	after := time.Now()

	if !errors.Is(err, pkgauth.ErrInvalidCredentials) {
		t.Errorf("expected pkgauth.ErrInvalidCredentials, got: %v", err)
	}
	if !repo.lockAccountCalled {
		t.Fatal("expected LockAccount to be called at the 3rd failed attempt")
	}
	if repo.lockAccountUserID != user.ID {
		t.Errorf("expected LockAccount called with userID=%d, got %d", user.ID, repo.lockAccountUserID)
	}
	minUntil := before.Add(pkgauth.LockoutDuration)
	maxUntil := after.Add(pkgauth.LockoutDuration)
	if repo.lockAccountUntil.Before(minUntil) || repo.lockAccountUntil.After(maxUntil) {
		t.Errorf("expected lockAccountUntil ~= now+30m, got %v (want between %v and %v)", repo.lockAccountUntil, minUntil, maxUntil)
	}
}

func TestService_Login_LocalPassword_Locked_RejectedEvenWithCorrectPassword(t *testing.T) {
	lockedUntil := time.Now().Add(10 * time.Minute)
	user := activeKaryawanUser()
	user.LockedUntil = &lockedUntil

	provider := &stubProvider{
		// Would succeed if reached — the lockout check must reject before this.
		authenticateID: &pkgauth.AuthIdentity{UserID: user.ID, Username: user.Username, Role: user.Role, IsKaryawan: user.IsKaryawan},
	}
	repo := &stubUserRepo{findResult: user}
	rl := &stubRateLimiter{}

	svc := newServiceUnderTest(provider, repo, rl)

	resp, refreshToken, err := svc.Login(context.Background(), LoginRequest{
		Username:   "john.admin",
		Password:   "Password123!", // correct password
		PortalType: "company",
		IP:         "127.0.0.1",
	})

	if resp != nil {
		t.Error("expected nil response for locked account")
	}
	if refreshToken != "" {
		t.Error("expected empty refresh token for locked account")
	}
	if !errors.Is(err, pkgauth.ErrAccountLocked) {
		t.Errorf("expected pkgauth.ErrAccountLocked, got: %v", err)
	}
}

func TestService_Login_LocalPassword_LockExpired_AllowsRetry(t *testing.T) {
	// locked_until 31 minutes in the past: auto-unlocked, login proceeds.
	lockedUntil := time.Now().Add(-31 * time.Minute)
	user := activeKaryawanUser()
	user.LockedUntil = &lockedUntil

	provider := &stubProvider{
		authenticateID: &pkgauth.AuthIdentity{UserID: user.ID, Username: user.Username, Role: user.Role, IsKaryawan: user.IsKaryawan},
	}
	repo := &stubUserRepo{findResult: user}
	rl := &stubRateLimiter{}

	svc := newServiceUnderTest(provider, repo, rl)

	resp, _, err := svc.Login(context.Background(), LoginRequest{
		Username:   "john.admin",
		Password:   "Password123!",
		PortalType: "company",
		IP:         "127.0.0.1",
	})

	if err != nil {
		t.Fatalf("expected no error (lock auto-expired), got: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

func TestService_Login_LocalPassword_Success_ResetsLockout(t *testing.T) {
	user := activeKaryawanUser()

	provider := &stubProvider{
		authenticateID: &pkgauth.AuthIdentity{UserID: user.ID, Username: user.Username, Role: user.Role, IsKaryawan: user.IsKaryawan},
	}
	repo := &stubUserRepo{findResult: user}
	rl := &stubRateLimiter{}

	svc := newServiceUnderTest(provider, repo, rl)

	_, _, err := svc.Login(context.Background(), LoginRequest{
		Username:   "john.admin",
		Password:   "Password123!",
		PortalType: "company",
		IP:         "127.0.0.1",
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !repo.resetLockoutCalled {
		t.Error("expected ResetLockout to be called on successful login")
	}
}

func TestService_Login_LDAPAccount_NeverLocked(t *testing.T) {
	// Even if locked_until were somehow set on an LDAP account, the lockout
	// policy must not apply to it, and a failed login must not increment the
	// per-account counter.
	lockedUntil := time.Now().Add(10 * time.Minute)
	user := activeKaryawanUser()
	user.AuthSource = "ldap"
	user.LockedUntil = &lockedUntil

	provider := &stubProvider{
		supportsFunc:    func(authSource string) bool { return authSource == "ldap" },
		authenticateErr: pkgauth.ErrInvalidCredentials,
	}
	repo := &stubUserRepo{findResult: user}
	rl := &stubRateLimiter{}

	svc := newServiceUnderTest(provider, repo, rl)

	_, _, err := svc.Login(context.Background(), LoginRequest{
		Username:   "john.admin",
		Password:   "WrongPassword!",
		PortalType: "company",
		IP:         "127.0.0.1",
	})

	if !errors.Is(err, pkgauth.ErrInvalidCredentials) {
		t.Errorf("expected pkgauth.ErrInvalidCredentials (not ErrAccountLocked), got: %v", err)
	}
	if repo.incrementFailedLoginCalled {
		t.Error("expected IncrementFailedLogin NOT to be called for ldap account")
	}
	if repo.lockAccountCalled {
		t.Error("expected LockAccount NOT to be called for ldap account")
	}
}
