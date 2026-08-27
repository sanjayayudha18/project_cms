package auth

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"pgregory.net/rapid"
)

// Feature: user-login, Property 3: Access Token Claims Completeness
// Feature: user-login, Property 4: Refresh Token Uniqueness and Rotation
// Feature: user-login, Property 5: Token Signature Integrity

// mockBlacklist is a mock TokenBlacklist that never blacklists anything.
type mockBlacklist struct{}

func (m *mockBlacklist) Add(_ context.Context, _ string, _ time.Duration) error {
	return nil
}

func (m *mockBlacklist) IsBlacklisted(_ context.Context, _ string) (bool, error) {
	return false, nil
}

// validRoles is the list of 9 valid DB roles.
var validRoles = []string{
	"ADMIN", "ADMIN_PARAM", "ATM-USER", "ATM-SPV",
	"BRANCH-USER", "BRANCH-SPV", "BRANCH-ATM-USER",
	"BRANCH-ATM-SPV", "VENDOR-USER",
}

// genAuthIdentity generates a random AuthIdentity using rapid.
func genAuthIdentity(t *rapid.T) *AuthIdentity {
	userID := rapid.Int64Range(1, 1_000_000).Draw(t, "userID")
	username := rapid.StringMatching(`[a-z][a-z0-9._]{2,30}`).Draw(t, "username")
	role := rapid.SampledFrom(validRoles).Draw(t, "role")
	isKaryawan := rapid.Bool().Draw(t, "isKaryawan")

	var vendorID *int64
	if !isKaryawan {
		vid := rapid.Int64Range(1, 10000).Draw(t, "vendorID")
		vendorID = &vid
	}

	return &AuthIdentity{
		UserID:     userID,
		Username:   username,
		Role:       role,
		IsKaryawan: isKaryawan,
		VendorID:   vendorID,
	}
}

// genSecretKey generates a random 32-byte secret key.
func genSecretKey(t *rapid.T) []byte {
	return rapid.SliceOfN(rapid.Byte(), 32, 64).Draw(t, "secretKey")
}

// newTestTokenService creates a TokenService with given secret.
func newTestTokenService(secret []byte) *TokenService {
	return NewTokenService(TokenConfig{
		SecretKey:          secret,
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	}, &mockBlacklist{})
}

// **Validates: Requirements 4.1, 4.2, 4.7**
func TestProperty_Token_AccessTokenClaimsCompleteness(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		secret := genSecretKey(t)
		identity := genAuthIdentity(t)
		ts := newTestTokenService(secret)

		accessToken, _, err := ts.GenerateTokenPair(identity)
		if err != nil {
			t.Fatalf("GenerateTokenPair failed: %v", err)
		}

		// Decode the token without validation to inspect raw claims
		claims := &AccessTokenClaims{}
		token, err := jwt.ParseWithClaims(accessToken, claims, func(token *jwt.Token) (any, error) {
			return secret, nil
		})
		if err != nil {
			t.Fatalf("ParseWithClaims failed: %v", err)
		}
		if !token.Valid {
			t.Fatal("token is not valid")
		}

		// Verify all identity fields are present in claims
		if claims.UserID != identity.UserID {
			t.Fatalf("UserID mismatch: got %d, want %d", claims.UserID, identity.UserID)
		}
		if claims.Username != identity.Username {
			t.Fatalf("Username mismatch: got %q, want %q", claims.Username, identity.Username)
		}
		if claims.Role != identity.Role {
			t.Fatalf("Role mismatch: got %q, want %q", claims.Role, identity.Role)
		}
		if claims.IsKaryawan != identity.IsKaryawan {
			t.Fatalf("IsKaryawan mismatch: got %v, want %v", claims.IsKaryawan, identity.IsKaryawan)
		}
		if identity.VendorID == nil && claims.VendorID != nil {
			t.Fatalf("VendorID should be nil, got %v", claims.VendorID)
		}
		if identity.VendorID != nil {
			if claims.VendorID == nil {
				t.Fatal("VendorID should not be nil")
			}
			if *claims.VendorID != *identity.VendorID {
				t.Fatalf("VendorID mismatch: got %d, want %d", *claims.VendorID, *identity.VendorID)
			}
		}

		// Verify iat and exp are present and exp - iat = 900 seconds (15 min)
		if claims.IssuedAt == nil {
			t.Fatal("IssuedAt (iat) is nil")
		}
		if claims.ExpiresAt == nil {
			t.Fatal("ExpiresAt (exp) is nil")
		}

		diff := claims.ExpiresAt.Time.Sub(claims.IssuedAt.Time)
		if diff != 900*time.Second {
			t.Fatalf("exp - iat = %v, want 900s (15min)", diff)
		}
	})
}

// **Validates: Requirements 4.3, 4.4**
func TestProperty_Token_RefreshTokenUniquenessAndRotation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		secret := genSecretKey(t)
		identity := genAuthIdentity(t)
		ts := newTestTokenService(secret)

		// Generate N refresh tokens (N >= 2) and ensure all JTIs are unique
		n := rapid.IntRange(2, 20).Draw(t, "n")
		jtis := make(map[string]bool, n)

		for i := 0; i < n; i++ {
			_, refreshToken, err := ts.GenerateTokenPair(identity)
			if err != nil {
				t.Fatalf("GenerateTokenPair iteration %d failed: %v", i, err)
			}

			// Parse refresh token to extract JTI
			claims := &RefreshTokenClaims{}
			token, err := jwt.ParseWithClaims(refreshToken, claims, func(token *jwt.Token) (any, error) {
				return secret, nil
			})
			if err != nil {
				t.Fatalf("ParseWithClaims iteration %d failed: %v", i, err)
			}
			if !token.Valid {
				t.Fatalf("refresh token iteration %d is not valid", i)
			}

			// Check JTI uniqueness
			if claims.ID == "" {
				t.Fatalf("JTI is empty at iteration %d", i)
			}
			if jtis[claims.ID] {
				t.Fatalf("duplicate JTI found at iteration %d: %s", i, claims.ID)
			}
			jtis[claims.ID] = true

			// Verify refresh expiry is 7 days from issuance
			if claims.IssuedAt == nil {
				t.Fatalf("refresh token iat is nil at iteration %d", i)
			}
			if claims.ExpiresAt == nil {
				t.Fatalf("refresh token exp is nil at iteration %d", i)
			}

			expectedExpiry := 7 * 24 * time.Hour
			diff := claims.ExpiresAt.Time.Sub(claims.IssuedAt.Time)
			if diff != expectedExpiry {
				t.Fatalf("refresh exp - iat = %v, want %v (7 days) at iteration %d", diff, expectedExpiry, i)
			}
		}
	})
}

// **Validates: Requirements 4.5, 4.6, 5.1**
func TestProperty_Token_SignatureIntegrity(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate two different secret keys
		keyA := genSecretKey(t)
		keyB := rapid.SliceOfN(rapid.Byte(), 32, 64).Draw(t, "keyB")

		// Ensure keys are different
		if string(keyA) == string(keyB) {
			// Flip a byte to guarantee difference
			keyB[0] = keyA[0] ^ 0xFF
		}

		identity := genAuthIdentity(t)
		tsA := newTestTokenService(keyA)
		tsB := newTestTokenService(keyB)

		// Generate token with key A
		accessToken, refreshToken, err := tsA.GenerateTokenPair(identity)
		if err != nil {
			t.Fatalf("GenerateTokenPair with keyA failed: %v", err)
		}

		// Validate with key A should succeed
		_, err = tsA.ValidateAccessToken(accessToken)
		if err != nil {
			t.Fatalf("ValidateAccessToken with keyA should succeed: %v", err)
		}

		ctx := context.Background()
		_, err = tsA.ValidateRefreshToken(ctx, refreshToken)
		if err != nil {
			t.Fatalf("ValidateRefreshToken with keyA should succeed: %v", err)
		}

		// Validate with key B should fail
		_, err = tsB.ValidateAccessToken(accessToken)
		if err == nil {
			t.Fatal("ValidateAccessToken with keyB should fail, but succeeded")
		}

		_, err = tsB.ValidateRefreshToken(ctx, refreshToken)
		if err == nil {
			t.Fatal("ValidateRefreshToken with keyB should fail, but succeeded")
		}

		// Modify access token payload and verify validation fails with key A
		// Tamper by appending a character to the token
		tamperedAccess := accessToken + "x"
		_, err = tsA.ValidateAccessToken(tamperedAccess)
		if err == nil {
			t.Fatal("ValidateAccessToken with tampered token should fail, but succeeded")
		}

		// Tamper by changing a character in the middle of the token
		if len(accessToken) > 50 {
			runes := []byte(accessToken)
			runes[len(runes)/2] ^= 0x01
			tamperedAccess2 := string(runes)
			_, err = tsA.ValidateAccessToken(tamperedAccess2)
			if err == nil {
				t.Fatal("ValidateAccessToken with bit-flipped token should fail, but succeeded")
			}
		}

		// Tamper refresh token
		tamperedRefresh := refreshToken + "x"
		_, err = tsA.ValidateRefreshToken(ctx, tamperedRefresh)
		if err == nil {
			t.Fatal("ValidateRefreshToken with tampered token should fail, but succeeded")
		}
	})
}
