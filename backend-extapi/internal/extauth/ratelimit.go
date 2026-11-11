package extauth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrUnavailable means the rate-limit store (Redis) could not be reached.
// Callers fail closed and answer 503 (Req 2.9, 4.5).
var ErrUnavailable = errors.New("rate limit store unavailable")

// apiKeyTTL keeps a per-minute API counter a little past its own minute
// (design: TTL 2 menit).
const apiKeyTTL = 2 * time.Minute

func unavailable(err error) error { return fmt.Errorf("%w: %w", ErrUnavailable, err) }

// TokenFailures counts failed token requests per client_id and per source IP
// in a fixed window (keys rate:ext:token:client:{id}, rate:ext:token:ip:{ip}).
// A client_id that fails ValidClientIDFormat is counted per IP only, so a
// caller cannot create arbitrary Redis keys.
type TokenFailures struct {
	rdb    *redis.Client
	limit  int
	window time.Duration
}

// NewTokenFailures returns a limiter allowing limit failures per window.
func NewTokenFailures(rdb *redis.Client, limit int, window time.Duration) *TokenFailures {
	return &TokenFailures{rdb: rdb, limit: limit, window: window}
}

func (f *TokenFailures) keys(clientID, ip string) []string {
	keys := make([]string, 0, 2)
	if ValidClientIDFormat(clientID) {
		keys = append(keys, "rate:ext:token:client:"+clientID)
	}
	return append(keys, "rate:ext:token:ip:"+ip)
}

// Check reports whether a token request may proceed. retryAfter > 0 means a
// limit is reached (429 + Retry-After); err wraps ErrUnavailable on Redis
// failure.
func (f *TokenFailures) Check(ctx context.Context, clientID, ip string) (time.Duration, error) {
	keys := f.keys(clientID, ip)
	pipe := f.rdb.Pipeline()
	gets := make([]*redis.StringCmd, len(keys))
	ttls := make([]*redis.DurationCmd, len(keys))
	for i, k := range keys {
		gets[i] = pipe.Get(ctx, k)
		ttls[i] = pipe.TTL(ctx, k)
	}
	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return 0, unavailable(err)
	}
	var retryAfter time.Duration
	for i := range keys {
		n, err := gets[i].Int()
		if errors.Is(err, redis.Nil) {
			continue
		}
		if err != nil {
			return 0, unavailable(err)
		}
		if n < f.limit {
			continue
		}
		ttl := ttls[i].Val()
		if ttl <= 0 { // key without expiry: report a full window, never 0
			ttl = f.window
		}
		retryAfter = max(retryAfter, ttl)
	}
	return retryAfter, nil
}

// RecordFailure counts one failed token request (INCR + EXPIRE NX per key in
// one pipeline: the window starts at the first failure and is never extended).
func (f *TokenFailures) RecordFailure(ctx context.Context, clientID, ip string) error {
	pipe := f.rdb.Pipeline()
	for _, k := range f.keys(clientID, ip) {
		pipe.Incr(ctx, k)
		pipe.ExpireNX(ctx, k, f.window)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return unavailable(err)
	}
	return nil
}

// ResetClient clears the per-client counter after a successful token request.
// The IP counter is left as is (same as login).
func (f *TokenFailures) ResetClient(ctx context.Context, clientID string) error {
	if !ValidClientIDFormat(clientID) {
		return nil
	}
	if err := f.rdb.Del(ctx, "rate:ext:token:client:"+clientID).Err(); err != nil {
		return unavailable(err)
	}
	return nil
}

// APIRate limits API requests per client per calendar minute
// (key rate:ext:api:{client_id}:{unix_minute}).
type APIRate struct {
	rdb       *redis.Client
	perMinute int
	now       func() time.Time
}

// NewAPIRate returns a limiter allowing perMinute requests per client.
func NewAPIRate(rdb *redis.Client, perMinute int) *APIRate {
	return &APIRate{rdb: rdb, perMinute: perMinute, now: time.Now}
}

// Allow counts one request. retryAfter > 0 means the limit is exceeded and is
// the time left in the current minute; err wraps ErrUnavailable on Redis failure.
func (a *APIRate) Allow(ctx context.Context, clientID string) (time.Duration, error) {
	now := a.now()
	minute := now.Unix() / 60
	key := fmt.Sprintf("rate:ext:api:%s:%d", clientID, minute)
	pipe := a.rdb.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.ExpireNX(ctx, key, apiKeyTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, unavailable(err)
	}
	if incr.Val() <= int64(a.perMinute) {
		return 0, nil
	}
	return time.Unix((minute+1)*60, 0).Sub(now), nil
}
