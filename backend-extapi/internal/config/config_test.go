package config

import (
	"strings"
	"testing"
	"time"
)

const goodSecret = "0123456789abcdef0123456789abcdef"

func env(over map[string]string) func(string) string {
	base := map[string]string{
		"DATABASE_URL":   "postgres://u:p@h/db",
		"REDIS_URL":      "redis://r:6379",
		"EXT_JWT_SECRET": goodSecret,
	}
	for k, v := range over {
		base[k] = v
	}
	return func(k string) string { return base[k] }
}

func TestLoadDefaults(t *testing.T) {
	c, err := Load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != "8082" || c.TokenIssuer != "crown" || c.TokenTTL != 15*time.Minute ||
		c.TokenFailLimit != 10 || c.TokenFailWin != 15*time.Minute || c.APIRatePerMin != 60 {
		t.Errorf("unexpected defaults: %+v", c)
	}
}

func TestLoadProxies(t *testing.T) {
	c, err := Load(env(map[string]string{"TRUSTED_PROXIES": "10.0.0.0/8, 192.168.1.1"}))
	if err != nil || len(c.TrustedProxies) != 2 {
		t.Fatalf("got %v, %v", c.TrustedProxies, err)
	}
}

// Property 14: bad configuration is rejected and the message never carries a secret value.
func TestLoadRejects(t *testing.T) {
	const leak = "leaky-jwt-secret-value-should-not-appear-in-errors"
	tests := []struct {
		name string
		over map[string]string
		want string
	}{
		{"JWT_SECRET present", map[string]string{"JWT_SECRET": leak}, "JWT_SECRET"},
		{"no database", map[string]string{"DATABASE_URL": ""}, "DATABASE_URL"},
		{"no redis", map[string]string{"REDIS_URL": ""}, "REDIS_URL"},
		{"no secret", map[string]string{"EXT_JWT_SECRET": ""}, "EXT_JWT_SECRET"},
		{"short secret", map[string]string{"EXT_JWT_SECRET": "short-secret-value"}, "EXT_JWT_SECRET"},
		{"31 byte secret", map[string]string{"EXT_JWT_SECRET": goodSecret[:31]}, "EXT_JWT_SECRET"},
		{"ttl too small", map[string]string{"EXT_TOKEN_TTL": "30s"}, "EXT_TOKEN_TTL"},
		{"ttl too big", map[string]string{"EXT_TOKEN_TTL": "61m"}, "EXT_TOKEN_TTL"},
		{"ttl garbage", map[string]string{"EXT_TOKEN_TTL": "abc"}, "EXT_TOKEN_TTL"},
		{"fail limit 0", map[string]string{"EXT_TOKEN_FAIL_LIMIT": "0"}, "EXT_TOKEN_FAIL_LIMIT"},
		{"fail limit big", map[string]string{"EXT_TOKEN_FAIL_LIMIT": "1001"}, "EXT_TOKEN_FAIL_LIMIT"},
		{"fail window small", map[string]string{"EXT_TOKEN_FAIL_WINDOW": "59s"}, "EXT_TOKEN_FAIL_WINDOW"},
		{"fail window big", map[string]string{"EXT_TOKEN_FAIL_WINDOW": "25h"}, "EXT_TOKEN_FAIL_WINDOW"},
		{"rate 0", map[string]string{"EXT_API_RATE_PER_MINUTE": "0"}, "EXT_API_RATE_PER_MINUTE"},
		{"rate big", map[string]string{"EXT_API_RATE_PER_MINUTE": "10001"}, "EXT_API_RATE_PER_MINUTE"},
		{"rate garbage", map[string]string{"EXT_API_RATE_PER_MINUTE": "x"}, "EXT_API_RATE_PER_MINUTE"},
		{"bad proxies", map[string]string{"TRUSTED_PROXIES": "not-an-ip"}, "TRUSTED_PROXIES"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(env(tt.over))
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not name %s", err, tt.want)
			}
			for _, v := range []string{leak, goodSecret, "short-secret-value", "postgres://"} {
				if strings.Contains(err.Error(), v) {
					t.Errorf("error leaks a value: %q", err)
				}
			}
		})
	}
}

func TestLoadCLI(t *testing.T) {
	c, err := LoadCLI(env(map[string]string{"REDIS_URL": "", "EXT_JWT_SECRET": ""}))
	if err != nil || c.DatabaseURL == "" {
		t.Fatalf("got %+v, %v", c, err)
	}
	if _, err := LoadCLI(env(map[string]string{"DATABASE_URL": ""})); err == nil {
		t.Error("expected error without DATABASE_URL")
	}
}
