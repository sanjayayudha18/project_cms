package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenConfig holds JWT signing and expiry configuration.
type TokenConfig struct {
	SecretKey          []byte        // min 32 bytes, from env
	AccessTokenExpiry  time.Duration // 15 minutes
	RefreshTokenExpiry time.Duration // 7 days
}

// AccessTokenClaims represents the claims embedded in an access token.
type AccessTokenClaims struct {
	UserID     int64  `json:"id"`
	Username   string `json:"username"`
	Role       string `json:"role"`
	IsKaryawan bool   `json:"is_karyawan"`
	VendorID   *int64 `json:"vendor_id,omitempty"`
	jwt.RegisteredClaims
}

// RefreshTokenClaims represents the claims embedded in a refresh token.
type RefreshTokenClaims struct {
	UserID int64 `json:"id"`
	jwt.RegisteredClaims
}

// TokenService handles JWT token generation, validation, and lifecycle management.
type TokenService struct {
	config    TokenConfig
	blacklist TokenBlacklist
}

// NewTokenService creates a new TokenService with the given configuration and blacklist store.
func NewTokenService(config TokenConfig, blacklist TokenBlacklist) *TokenService {
	return &TokenService{
		config:    config,
		blacklist: blacklist,
	}
}

// GenerateTokenPair creates both an access token and a refresh token for the given identity.
// The access token contains user claims and expires in 15 minutes.
// The refresh token contains the user ID and a unique JTI, expiring in 7 days.
func (ts *TokenService) GenerateTokenPair(identity *AuthIdentity) (accessToken, refreshToken string, err error) {
	if identity == nil {
		return "", "", fmt.Errorf("identity cannot be nil")
	}

	now := time.Now()

	// Generate access token
	accessToken, err = ts.generateAccessToken(identity, now)
	if err != nil {
		return "", "", fmt.Errorf("generating access token: %w", err)
	}

	// Generate refresh token
	refreshToken, err = ts.generateRefreshToken(identity, now)
	if err != nil {
		return "", "", fmt.Errorf("generating refresh token: %w", err)
	}

	return accessToken, refreshToken, nil
}

// generateAccessToken creates a signed access token with user identity claims.
func (ts *TokenService) generateAccessToken(identity *AuthIdentity, now time.Time) (string, error) {
	claims := AccessTokenClaims{
		UserID:     identity.UserID,
		Username:   identity.Username,
		Role:       identity.Role,
		IsKaryawan: identity.IsKaryawan,
		VendorID:   identity.VendorID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ts.config.AccessTokenExpiry)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(ts.config.SecretKey)
	if err != nil {
		return "", err
	}

	return signed, nil
}

// generateRefreshToken creates a signed refresh token with a unique JTI for revocation tracking.
func (ts *TokenService) generateRefreshToken(identity *AuthIdentity, now time.Time) (string, error) {
	jti := uuid.New().String()

	claims := RefreshTokenClaims{
		UserID: identity.UserID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ts.config.RefreshTokenExpiry)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(ts.config.SecretKey)
	if err != nil {
		return "", err
	}

	return signed, nil
}

// ValidateAccessToken verifies signature, expiry, and required claims.
// Returns the claims if valid, or an error if token is invalid/expired/tampered.
func (ts *TokenService) ValidateAccessToken(tokenStr string) (*AccessTokenClaims, error) {
	claims := &AccessTokenClaims{}

	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return ts.config.SecretKey, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}

	if !token.Valid {
		return nil, ErrTokenInvalid
	}

	// Verify required claims are present
	if claims.UserID == 0 {
		return nil, ErrTokenInvalid
	}
	if claims.Username == "" {
		return nil, ErrTokenInvalid
	}
	if claims.Role == "" {
		return nil, ErrTokenInvalid
	}

	return claims, nil
}

// ValidateRefreshToken validates a refresh token and checks it's not blacklisted.
// Returns the claims if valid.
func (ts *TokenService) ValidateRefreshToken(ctx context.Context, tokenStr string) (*RefreshTokenClaims, error) {
	claims := &RefreshTokenClaims{}

	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return ts.config.SecretKey, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}

	if !token.Valid {
		return nil, ErrTokenInvalid
	}

	// Verify required claims
	if claims.UserID == 0 {
		return nil, ErrTokenInvalid
	}
	if claims.ID == "" {
		return nil, ErrTokenInvalid
	}

	// Check if JTI is blacklisted
	blacklisted, err := ts.blacklist.IsBlacklisted(ctx, claims.ID)
	if err != nil {
		// Fail-closed: if Redis is unavailable, reject the token
		return nil, ErrServiceUnavailable
	}
	if blacklisted {
		return nil, ErrTokenExpired
	}

	return claims, nil
}

// BlacklistRefreshToken parses a refresh token to extract its JTI and remaining TTL,
// then adds the JTI to the Redis blacklist.
func (ts *TokenService) BlacklistRefreshToken(ctx context.Context, refreshTokenStr string) error {
	claims := &RefreshTokenClaims{}

	_, err := jwt.ParseWithClaims(refreshTokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return ts.config.SecretKey, nil
	})
	if err != nil {
		// If token is expired or invalid, nothing to blacklist
		return nil
	}

	if claims.ID == "" {
		return nil
	}

	// Calculate remaining TTL
	if claims.ExpiresAt == nil {
		return nil
	}
	remaining := time.Until(claims.ExpiresAt.Time)
	if remaining <= 0 {
		// Token already expired, no need to blacklist
		return nil
	}

	return ts.blacklist.Add(ctx, claims.ID, remaining)
}
