package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// TokenBlacklist manages revoked refresh token JTIs.
type TokenBlacklist interface {
	// Add adds a JTI to the blacklist with the given TTL.
	Add(ctx context.Context, jti string, ttl time.Duration) error
	// IsBlacklisted checks if a JTI is in the blacklist.
	IsBlacklisted(ctx context.Context, jti string) (bool, error)
}

// RedisTokenBlacklist implements TokenBlacklist using Redis.
type RedisTokenBlacklist struct {
	client *redis.Client
}

// NewRedisTokenBlacklist creates a new RedisTokenBlacklist.
func NewRedisTokenBlacklist(client *redis.Client) *RedisTokenBlacklist {
	return &RedisTokenBlacklist{client: client}
}

// blacklistKey returns the Redis key for a given JTI.
func blacklistKey(jti string) string {
	return fmt.Sprintf("blacklist:jti:%s", jti)
}

// Add adds a JTI to the blacklist with the given TTL.
// The TTL should be the remaining time until the token's natural expiry.
func (b *RedisTokenBlacklist) Add(ctx context.Context, jti string, ttl time.Duration) error {
	return b.client.Set(ctx, blacklistKey(jti), "1", ttl).Err()
}

// IsBlacklisted checks if a JTI is in the blacklist.
// Returns true if the key exists, false otherwise.
// If Redis is unavailable, returns an error (fail-closed behavior).
func (b *RedisTokenBlacklist) IsBlacklisted(ctx context.Context, jti string) (bool, error) {
	result, err := b.client.Exists(ctx, blacklistKey(jti)).Result()
	if err != nil {
		return false, err
	}
	return result > 0, nil
}
