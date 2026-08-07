package auth

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"pgregory.net/rapid"
)

// Feature: user-login, Property 11: Blacklist Prevents Token Reuse
// Feature: user-login, Property 12: Idempotent Logout

// newTestTokenServiceWithRedis creates a TokenService backed by a real Redis (miniredis) blacklist.
func newTestTokenServiceWithRedis(secret []byte, redisClient *redis.Client) *TokenService {
	return NewTokenService(TokenConfig{
		SecretKey:          secret,
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	}, NewRedisTokenBlacklist(redisClient))
}

// **Validates: Requirements 8.1, 8.3, 8.4**
func TestProperty_Session_BlacklistPreventsTokenReuse(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer redisClient.Close()

	rapid.Check(t, func(t *rapid.T) {
		secret := genSecretKey(t)
		identity := genAuthIdentity(t)
		ts := newTestTokenServiceWithRedis(secret, redisClient)
		ctx := context.Background()

		// Generate a valid token pair
		_, refreshToken, err := ts.GenerateTokenPair(identity)
		if err != nil {
			t.Fatalf("GenerateTokenPair failed: %v", err)
		}

		// Verify the refresh token is valid before blacklisting
		_, err = ts.ValidateRefreshToken(ctx, refreshToken)
		if err != nil {
			t.Fatalf("ValidateRefreshToken should succeed before blacklist: %v", err)
		}

		// Blacklist the refresh token
		err = ts.BlacklistRefreshToken(ctx, refreshToken)
		if err != nil {
			t.Fatalf("BlacklistRefreshToken failed: %v", err)
		}

		// After blacklisting, validation must fail with ErrTokenExpired
		_, err = ts.ValidateRefreshToken(ctx, refreshToken)
		if err == nil {
			t.Fatal("ValidateRefreshToken should fail after blacklisting, but succeeded")
		}
		if err != ErrTokenExpired {
			t.Fatalf("expected ErrTokenExpired after blacklisting, got: %v", err)
		}
	})
}

// **Validates: Requirements 8.2**
func TestProperty_Session_IdempotentLogout(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer redisClient.Close()

	rapid.Check(t, func(t *rapid.T) {
		secret := genSecretKey(t)
		identity := genAuthIdentity(t)
		ts := newTestTokenServiceWithRedis(secret, redisClient)
		ctx := context.Background()

		// Choose a token type to test: valid, expired, malformed, or empty
		tokenType := rapid.SampledFrom([]string{"valid", "expired", "malformed", "empty"}).Draw(t, "tokenType")

		var tokenStr string
		switch tokenType {
		case "valid":
			// Generate a valid refresh token
			_, refreshToken, err := ts.GenerateTokenPair(identity)
			if err != nil {
				t.Fatalf("GenerateTokenPair failed: %v", err)
			}
			tokenStr = refreshToken

		case "expired":
			// Create a token service with 0 expiry to generate an already-expired token
			expiredTS := NewTokenService(TokenConfig{
				SecretKey:          secret,
				AccessTokenExpiry:  15 * time.Minute,
				RefreshTokenExpiry: -1 * time.Second, // already expired
			}, NewRedisTokenBlacklist(redisClient))
			_, refreshToken, err := expiredTS.GenerateTokenPair(identity)
			if err != nil {
				t.Fatalf("GenerateTokenPair (expired) failed: %v", err)
			}
			tokenStr = refreshToken

		case "malformed":
			// Generate garbage string as a malformed token
			tokenStr = rapid.StringMatching(`[a-zA-Z0-9._\-]{10,100}`).Draw(t, "garbageToken")

		case "empty":
			tokenStr = ""
		}

		// BlacklistRefreshToken must NOT return an error for any of these cases
		err := ts.BlacklistRefreshToken(ctx, tokenStr)
		if err != nil {
			t.Fatalf("BlacklistRefreshToken should not return error for %s token, got: %v", tokenType, err)
		}
	})
}
