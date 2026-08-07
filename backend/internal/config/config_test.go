package config

import (
	"testing"
	"time"
)

// setEnvForTest sets required env vars for a valid config and returns a cleanup function.
func setEnvForTest(t *testing.T) {
	t.Helper()
	t.Setenv("JWT_SECRET", "this-is-a-secret-key-that-is-at-least-32-bytes!")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/cms?sslmode=disable")
}

func TestLoad_Success_WithDefaults(t *testing.T) {
	setEnvForTest(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(cfg.JWTSecret) < 32 {
		t.Errorf("JWTSecret should be at least 32 bytes, got %d", len(cfg.JWTSecret))
	}
	if cfg.AccessTokenExpiry != 15*time.Minute {
		t.Errorf("AccessTokenExpiry = %v, want 15m", cfg.AccessTokenExpiry)
	}
	if cfg.RefreshTokenExpiry != 7*24*time.Hour {
		t.Errorf("RefreshTokenExpiry = %v, want 7 days", cfg.RefreshTokenExpiry)
	}
	if cfg.RedisURL != "redis://localhost:6379" {
		t.Errorf("RedisURL = %q, want redis://localhost:6379", cfg.RedisURL)
	}
	if cfg.DatabaseURL != "postgres://user:pass@localhost:5432/cms?sslmode=disable" {
		t.Errorf("DatabaseURL = %q, want postgres connection string", cfg.DatabaseURL)
	}
	if cfg.RateLimitUsername != 5 {
		t.Errorf("RateLimitUsername = %d, want 5", cfg.RateLimitUsername)
	}
	if cfg.RateLimitIP != 20 {
		t.Errorf("RateLimitIP = %d, want 20", cfg.RateLimitIP)
	}
	if cfg.RateLimitWindow != 15*time.Minute {
		t.Errorf("RateLimitWindow = %v, want 15m", cfg.RateLimitWindow)
	}
}

func TestLoad_Success_WithCustomValues(t *testing.T) {
	setEnvForTest(t)
	t.Setenv("ACCESS_TOKEN_EXPIRY", "30m")
	t.Setenv("REFRESH_TOKEN_EXPIRY", "24h")
	t.Setenv("RATE_LIMIT_USERNAME", "10")
	t.Setenv("RATE_LIMIT_IP", "50")
	t.Setenv("RATE_LIMIT_WINDOW", "30m")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.AccessTokenExpiry != 30*time.Minute {
		t.Errorf("AccessTokenExpiry = %v, want 30m", cfg.AccessTokenExpiry)
	}
	if cfg.RefreshTokenExpiry != 24*time.Hour {
		t.Errorf("RefreshTokenExpiry = %v, want 24h", cfg.RefreshTokenExpiry)
	}
	if cfg.RateLimitUsername != 10 {
		t.Errorf("RateLimitUsername = %d, want 10", cfg.RateLimitUsername)
	}
	if cfg.RateLimitIP != 50 {
		t.Errorf("RateLimitIP = %d, want 50", cfg.RateLimitIP)
	}
	if cfg.RateLimitWindow != 30*time.Minute {
		t.Errorf("RateLimitWindow = %v, want 30m", cfg.RateLimitWindow)
	}
}

func TestLoad_Error_MissingJWTSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("DATABASE_URL", "postgres://localhost/cms")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing JWT_SECRET")
	}
	if err.Error() != "JWT_SECRET environment variable is required" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLoad_Error_JWTSecretTooShort(t *testing.T) {
	t.Setenv("JWT_SECRET", "short")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("DATABASE_URL", "postgres://localhost/cms")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for short JWT_SECRET")
	}
	if err.Error() != "JWT_SECRET must be at least 32 bytes, got 5" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLoad_Error_MissingRedisURL(t *testing.T) {
	t.Setenv("JWT_SECRET", "this-is-a-secret-key-that-is-at-least-32-bytes!")
	t.Setenv("REDIS_URL", "")
	t.Setenv("DATABASE_URL", "postgres://localhost/cms")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing REDIS_URL")
	}
	if err.Error() != "REDIS_URL environment variable is required" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLoad_Error_MissingDatabaseURL(t *testing.T) {
	t.Setenv("JWT_SECRET", "this-is-a-secret-key-that-is-at-least-32-bytes!")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("DATABASE_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing DATABASE_URL")
	}
	if err.Error() != "DATABASE_URL environment variable is required" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLoad_InvalidDuration_FallsBackToDefault(t *testing.T) {
	setEnvForTest(t)
	t.Setenv("ACCESS_TOKEN_EXPIRY", "not-a-duration")
	t.Setenv("RATE_LIMIT_WINDOW", "invalid")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.AccessTokenExpiry != 15*time.Minute {
		t.Errorf("AccessTokenExpiry = %v, want 15m (default on invalid input)", cfg.AccessTokenExpiry)
	}
	if cfg.RateLimitWindow != 15*time.Minute {
		t.Errorf("RateLimitWindow = %v, want 15m (default on invalid input)", cfg.RateLimitWindow)
	}
}

func TestLoad_InvalidInt_FallsBackToDefault(t *testing.T) {
	setEnvForTest(t)
	t.Setenv("RATE_LIMIT_USERNAME", "abc")
	t.Setenv("RATE_LIMIT_IP", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.RateLimitUsername != 5 {
		t.Errorf("RateLimitUsername = %d, want 5 (default on invalid input)", cfg.RateLimitUsername)
	}
	if cfg.RateLimitIP != 20 {
		t.Errorf("RateLimitIP = %d, want 20 (default on empty)", cfg.RateLimitIP)
	}
}
