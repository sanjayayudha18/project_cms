package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds application configuration loaded from environment variables,
// shared between the ATM (backend) and CIT (backend-cit) services.
type Config struct {
	// JWT
	JWTSecret          []byte
	AccessTokenExpiry  time.Duration
	RefreshTokenExpiry time.Duration

	// Database
	DatabaseURL        string
	DatabaseReplicaURL string

	// Redis
	RedisURL string

	// Rate Limiting
	RateLimitUsername int
	RateLimitIP       int
	RateLimitWindow   time.Duration

	// Server
	Port string

	// LDAP (optional, used by ATM backend)
	LDAPURL          string
	LDAPBaseDN       string
	LDAPBindDN       string
	LDAPBindPassword string
}

// Load reads configuration from environment variables with sensible defaults.
// defaultPort is used when the PORT environment variable is unset (e.g. "8080" for
// the ATM backend, "8081" for the CIT backend).
// Returns an error if required variables are missing or values are invalid.
func Load(defaultPort string) (*Config, error) {
	// JWT_SECRET — required, min 32 bytes
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, errors.New("JWT_SECRET environment variable is required")
	}
	if len(secret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 bytes, got %d", len(secret))
	}

	// DATABASE_URL — required
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, errors.New("DATABASE_URL environment variable is required")
	}

	// REDIS_URL — required
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		return nil, errors.New("REDIS_URL environment variable is required")
	}

	// PORT — optional, defaults to the caller-supplied defaultPort
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	cfg := &Config{
		JWTSecret:          []byte(secret),
		AccessTokenExpiry:  parseDuration("ACCESS_TOKEN_EXPIRY", 15*time.Minute),
		RefreshTokenExpiry: parseDuration("REFRESH_TOKEN_EXPIRY", 7*24*time.Hour),
		DatabaseURL:        dbURL,
		DatabaseReplicaURL: os.Getenv("DATABASE_REPLICA_URL"),
		RedisURL:           redisURL,
		RateLimitUsername:  parseInt("RATE_LIMIT_USERNAME", 5),
		RateLimitIP:        parseInt("RATE_LIMIT_IP", 20),
		RateLimitWindow:    parseDuration("RATE_LIMIT_WINDOW", 15*time.Minute),
		Port:               port,
		LDAPURL:            os.Getenv("LDAP_URL"),
		LDAPBaseDN:         os.Getenv("LDAP_BASE_DN"),
		LDAPBindDN:         os.Getenv("LDAP_BIND_DN"),
		LDAPBindPassword:   os.Getenv("LDAP_BIND_PASSWORD"),
	}

	return cfg, nil
}

// parseDuration reads a duration from an environment variable.
// Accepts Go duration strings (e.g. "15m", "24h", "30s").
// Returns the default value if the variable is empty or unparseable.
func parseDuration(key string, defaultVal time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		return defaultVal
	}
	return d
}

// parseInt reads an integer from an environment variable.
// Returns the default value if the variable is empty or unparseable.
func parseInt(key string, defaultVal int) int {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return n
}
