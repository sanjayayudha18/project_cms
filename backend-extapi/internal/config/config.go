// Package config loads backend-extapi settings from the environment.
//
// pkg/config.Load is not used because it requires JWT_SECRET, which this
// service must never hold (Req 3.4). Errors name the env var, never its value.
package config

import (
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"time"

	pkgconfig "github.com/cimb-niaga/cms/pkg/config"
)

// MinSecretBytes is the minimum length of EXT_JWT_SECRET.
const MinSecretBytes = 32

// Config is the validated runtime configuration.
type Config struct {
	DatabaseURL    string
	RedisURL       string
	Port           string
	TrustedProxies []netip.Prefix
	ExtJWTSecret   string
	TokenIssuer    string
	TokenTTL       time.Duration
	TokenFailLimit int
	TokenFailWin   time.Duration
	APIRatePerMin  int
}

// CLIConfig is what the extclient CLI needs.
type CLIConfig struct {
	DatabaseURL string
}

// Load reads and validates the service configuration. getenv is os.Getenv in
// production.
func Load(getenv func(string) string) (Config, error) {
	// ponytail: getenv cannot tell unset from empty, so an empty JWT_SECRET passes.
	if getenv("JWT_SECRET") != "" {
		return Config{}, errors.New("JWT_SECRET must not be set for backend-extapi")
	}

	var c Config
	var err error
	if c.DatabaseURL, err = required(getenv, "DATABASE_URL"); err != nil {
		return Config{}, err
	}
	if c.RedisURL, err = required(getenv, "REDIS_URL"); err != nil {
		return Config{}, err
	}
	if c.ExtJWTSecret, err = required(getenv, "EXT_JWT_SECRET"); err != nil {
		return Config{}, err
	}
	if len(c.ExtJWTSecret) < MinSecretBytes {
		return Config{}, fmt.Errorf("EXT_JWT_SECRET must be at least %d bytes", MinSecretBytes)
	}

	c.Port = withDefault(getenv, "PORT", "8082")
	c.TokenIssuer = withDefault(getenv, "EXT_TOKEN_ISSUER", "crown")

	if c.TrustedProxies, err = pkgconfig.ParseTrustedProxies(getenv("TRUSTED_PROXIES")); err != nil {
		return Config{}, errors.New("TRUSTED_PROXIES is invalid")
	}
	if c.TokenTTL, err = durationIn(getenv, "EXT_TOKEN_TTL", "15m", time.Minute, time.Hour); err != nil {
		return Config{}, err
	}
	if c.TokenFailWin, err = durationIn(getenv, "EXT_TOKEN_FAIL_WINDOW", "15m", time.Minute, 24*time.Hour); err != nil {
		return Config{}, err
	}
	if c.TokenFailLimit, err = intIn(getenv, "EXT_TOKEN_FAIL_LIMIT", "10", 1, 1000); err != nil {
		return Config{}, err
	}
	if c.APIRatePerMin, err = intIn(getenv, "EXT_API_RATE_PER_MINUTE", "60", 1, 10000); err != nil {
		return Config{}, err
	}
	return c, nil
}

// LoadCLI reads only DATABASE_URL, for the extclient tool.
func LoadCLI(getenv func(string) string) (CLIConfig, error) {
	u, err := required(getenv, "DATABASE_URL")
	if err != nil {
		return CLIConfig{}, err
	}
	return CLIConfig{DatabaseURL: u}, nil
}

func required(getenv func(string) string, name string) (string, error) {
	v := strings.TrimSpace(getenv(name))
	if v == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return v, nil
}

func withDefault(getenv func(string) string, name, def string) string {
	if v := strings.TrimSpace(getenv(name)); v != "" {
		return v
	}
	return def
}

func durationIn(getenv func(string) string, name, def string, lo, hi time.Duration) (time.Duration, error) {
	d, err := time.ParseDuration(withDefault(getenv, name, def))
	if err != nil {
		return 0, fmt.Errorf("%s is not a valid duration", name)
	}
	if d < lo || d > hi {
		return 0, fmt.Errorf("%s must be between %s and %s", name, lo, hi)
	}
	return d, nil
}

func intIn(getenv func(string) string, name, def string, lo, hi int) (int, error) {
	n, err := strconv.Atoi(withDefault(getenv, name, def))
	if err != nil {
		return 0, fmt.Errorf("%s is not a valid integer", name)
	}
	if n < lo || n > hi {
		return 0, fmt.Errorf("%s must be between %d and %d", name, lo, hi)
	}
	return n, nil
}
