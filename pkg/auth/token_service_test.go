package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// mockTokenBlacklist is a test double for TokenBlacklist.
type mockTokenBlacklist struct {
	blacklisted map[string]bool
	addErr      error
	checkErr    error
}

func newMockBlacklist() *mockTokenBlacklist {
	return &mockTokenBlacklist{
		blacklisted: make(map[string]bool),
	}
}

func (m *mockTokenBlacklist) Add(_ context.Context, jti string, _ time.Duration) error {
	if m.addErr != nil {
		return m.addErr
	}
	m.blacklisted[jti] = true
	return nil
}

func (m *mockTokenBlacklist) IsBlacklisted(_ context.Context, jti string) (bool, error) {
	if m.checkErr != nil {
		return false, m.checkErr
	}
	return m.blacklisted[jti], nil
}

// testTokenConfig returns a TokenConfig suitable for unit testing.
func testTokenConfig() TokenConfig {
	return TokenConfig{
		SecretKey:          []byte("test-secret-key-minimum-32-bytes!"),
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	}
}

// testIdentity returns an AuthIdentity suitable for testing.
func testIdentity() *AuthIdentity {
	vendorID := int64(10)
	return &AuthIdentity{
		UserID:     1,
		Username:   "john.admin",
		Role:       "ADMIN",
		IsKaryawan: true,
		VendorID:   &vendorID,
	}
}

// signTokenWithKey is a test helper that generates a token signed with a custom key.
func signTokenWithKey(t *testing.T, claims jwt.Claims, key []byte) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return signed
}

func TestTokenService_GenerateTokenPair_Valid(t *testing.T) {
	bl := newMockBlacklist()
	svc := NewTokenService(testTokenConfig(), bl)

	accessToken, refreshToken, err := svc.GenerateTokenPair(testIdentity())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if accessToken == "" {
		t.Error("expected non-empty access token")
	}
	if refreshToken == "" {
		t.Error("expected non-empty refresh token")
	}

	// Verify access token is parseable and contains expected claims
	claims, err := svc.ValidateAccessToken(accessToken)
	if err != nil {
		t.Fatalf("expected access token to be valid, got: %v", err)
	}
	if claims.UserID != 1 {
		t.Errorf("expected UserID=1, got %d", claims.UserID)
	}
	if claims.Username != "john.admin" {
		t.Errorf("expected Username=john.admin, got %s", claims.Username)
	}
	if claims.Role != "ADMIN" {
		t.Errorf("expected Role=ADMIN, got %s", claims.Role)
	}
	if !claims.IsKaryawan {
		t.Error("expected IsKaryawan=true")
	}
	if claims.VendorID == nil || *claims.VendorID != 10 {
		t.Errorf("expected VendorID=10, got %v", claims.VendorID)
	}

	// Verify refresh token is parseable
	refreshClaims, err := svc.ValidateRefreshToken(context.Background(), refreshToken)
	if err != nil {
		t.Fatalf("expected refresh token to be valid, got: %v", err)
	}
	if refreshClaims.UserID != 1 {
		t.Errorf("expected refresh UserID=1, got %d", refreshClaims.UserID)
	}
	if refreshClaims.ID == "" {
		t.Error("expected refresh token to have a JTI")
	}
}

func TestTokenService_ValidateAccessToken_Expired(t *testing.T) {
	bl := newMockBlacklist()
	cfg := testTokenConfig()
	svc := NewTokenService(cfg, bl)

	// Create a token that expired 1 hour ago
	now := time.Now().Add(-2 * time.Hour)
	claims := AccessTokenClaims{
		UserID:     1,
		Username:   "john.admin",
		Role:       "ADMIN",
		IsKaryawan: true,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)), // expired 1h45m ago
		},
	}

	tokenStr := signTokenWithKey(t, claims, cfg.SecretKey)

	_, err := svc.ValidateAccessToken(tokenStr)
	if !errors.Is(err, ErrTokenExpired) {
		t.Errorf("expected ErrTokenExpired, got: %v", err)
	}
}

func TestTokenService_ValidateAccessToken_Tampered(t *testing.T) {
	bl := newMockBlacklist()
	cfg := testTokenConfig()
	svc := NewTokenService(cfg, bl)

	// Generate a valid token, then sign it with a different key to simulate tampering
	claims := AccessTokenClaims{
		UserID:     1,
		Username:   "john.admin",
		Role:       "ADMIN",
		IsKaryawan: true,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		},
	}

	// Sign with a different key
	tamperedKey := []byte("different-secret-key-32-bytes!!!!")
	tokenStr := signTokenWithKey(t, claims, tamperedKey)

	_, err := svc.ValidateAccessToken(tokenStr)
	if !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("expected ErrTokenInvalid for tampered token, got: %v", err)
	}
}

func TestTokenService_ValidateRefreshToken_BlacklistedJTI(t *testing.T) {
	bl := newMockBlacklist()
	cfg := testTokenConfig()
	svc := NewTokenService(cfg, bl)

	// Generate a valid refresh token
	identity := testIdentity()
	_, refreshToken, err := svc.GenerateTokenPair(identity)
	if err != nil {
		t.Fatalf("expected no error generating token pair, got: %v", err)
	}

	// Extract JTI from the refresh token
	refreshClaims, err := svc.ValidateRefreshToken(context.Background(), refreshToken)
	if err != nil {
		t.Fatalf("expected valid refresh token before blacklisting, got: %v", err)
	}

	// Blacklist the JTI
	bl.blacklisted[refreshClaims.ID] = true

	// Validate should now fail
	_, err = svc.ValidateRefreshToken(context.Background(), refreshToken)
	if !errors.Is(err, ErrTokenExpired) {
		t.Errorf("expected ErrTokenExpired for blacklisted JTI, got: %v", err)
	}
}

func TestTokenService_ValidateAccessToken_MissingClaims(t *testing.T) {
	bl := newMockBlacklist()
	cfg := testTokenConfig()
	svc := NewTokenService(cfg, bl)

	tests := []struct {
		name   string
		claims AccessTokenClaims
	}{
		{
			name: "missing UserID",
			claims: AccessTokenClaims{
				UserID:   0, // zero value = missing
				Username: "john.admin",
				Role:     "ADMIN",
				RegisteredClaims: jwt.RegisteredClaims{
					IssuedAt:  jwt.NewNumericDate(time.Now()),
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
				},
			},
		},
		{
			name: "missing Username",
			claims: AccessTokenClaims{
				UserID:   1,
				Username: "", // empty = missing
				Role:     "ADMIN",
				RegisteredClaims: jwt.RegisteredClaims{
					IssuedAt:  jwt.NewNumericDate(time.Now()),
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
				},
			},
		},
		{
			name: "missing Role",
			claims: AccessTokenClaims{
				UserID:   1,
				Username: "john.admin",
				Role:     "", // empty = missing
				RegisteredClaims: jwt.RegisteredClaims{
					IssuedAt:  jwt.NewNumericDate(time.Now()),
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenStr := signTokenWithKey(t, tt.claims, cfg.SecretKey)

			_, err := svc.ValidateAccessToken(tokenStr)
			if !errors.Is(err, ErrTokenInvalid) {
				t.Errorf("expected ErrTokenInvalid for %s, got: %v", tt.name, err)
			}
		})
	}
}

func TestTokenService_ValidateAccessToken_WrongSigningKey(t *testing.T) {
	bl := newMockBlacklist()
	cfg := testTokenConfig()
	svc := NewTokenService(cfg, bl)

	// Create valid claims but sign with a completely different key
	claims := AccessTokenClaims{
		UserID:     1,
		Username:   "john.admin",
		Role:       "ADMIN",
		IsKaryawan: true,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		},
	}

	wrongKey := []byte("completely-wrong-key-32-bytes!!!!")
	tokenStr := signTokenWithKey(t, claims, wrongKey)

	_, err := svc.ValidateAccessToken(tokenStr)
	if !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("expected ErrTokenInvalid for wrong signing key, got: %v", err)
	}
}
