package extauth

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

const (
	rlClient  = "crown_aaaaaaaaaaaaaaaaaaaaaaaaaa"
	rlClient2 = "crown_bbbbbbbbbbbbbbbbbbbbbbbbbb"
	rlIP      = "10.0.0.1"
	rlWindow  = 15 * time.Minute
)

func newMiniRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, rdb
}

func recordN(t *testing.T, f *TokenFailures, n int, clientID, ip string) {
	t.Helper()
	for i := 0; i < n; i++ {
		if err := f.RecordFailure(context.Background(), clientID, ip); err != nil {
			t.Fatalf("RecordFailure #%d: %v", i+1, err)
		}
	}
}

func mustCheck(t *testing.T, f *TokenFailures, clientID, ip string) time.Duration {
	t.Helper()
	d, err := f.Check(context.Background(), clientID, ip)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	return d
}

func TestTokenFailures_Limits(t *testing.T) {
	tests := []struct {
		name      string
		record    func(t *testing.T, f *TokenFailures)
		clientID  string
		ip        string
		wantBlock bool
	}{
		{"no failures", func(*testing.T, *TokenFailures) {}, rlClient, rlIP, false},
		{"below limit", func(t *testing.T, f *TokenFailures) { recordN(t, f, 2, rlClient, rlIP) }, rlClient, rlIP, false},
		{"client limit reached from another ip", func(t *testing.T, f *TokenFailures) { recordN(t, f, 3, rlClient, rlIP) }, rlClient, "10.0.0.9", true},
		{"ip limit reached across clients", func(t *testing.T, f *TokenFailures) {
			recordN(t, f, 2, rlClient, rlIP)
			recordN(t, f, 1, rlClient2, rlIP)
		}, "crown_cccccccccccccccccccccccccc", rlIP, true},
		{"other client and ip unaffected", func(t *testing.T, f *TokenFailures) { recordN(t, f, 3, rlClient, rlIP) }, rlClient2, "10.0.0.9", false},
		{"invalid client_id not counted per client", func(t *testing.T, f *TokenFailures) { recordN(t, f, 3, "bogus", rlIP) }, "bogus", "10.0.0.9", false},
		{"invalid client_id still counted per ip", func(t *testing.T, f *TokenFailures) { recordN(t, f, 3, "bogus", rlIP) }, rlClient, rlIP, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, rdb := newMiniRedis(t)
			f := NewTokenFailures(rdb, 3, rlWindow)
			tt.record(t, f)
			d := mustCheck(t, f, tt.clientID, tt.ip)
			if (d > 0) != tt.wantBlock {
				t.Fatalf("retryAfter = %v, want blocked = %v", d, tt.wantBlock)
			}
			if d > rlWindow {
				t.Fatalf("retryAfter %v exceeds window", d)
			}
		})
	}
}

func TestTokenFailures_InvalidClientIDCreatesNoClientKey(t *testing.T) {
	mr, rdb := newMiniRedis(t)
	f := NewTokenFailures(rdb, 3, rlWindow)
	recordN(t, f, 1, "crown_UPPER*:x", rlIP)
	keys := mr.Keys()
	if len(keys) != 1 || keys[0] != "rate:ext:token:ip:"+rlIP {
		t.Fatalf("keys = %v, want only the ip key", keys)
	}
}

func TestTokenFailures_TTLAndWindowEnd(t *testing.T) {
	mr, rdb := newMiniRedis(t)
	f := NewTokenFailures(rdb, 3, rlWindow)
	recordN(t, f, 1, rlClient, rlIP)
	mr.FastForward(5 * time.Minute)
	recordN(t, f, 2, rlClient, rlIP) // must not extend the window (EXPIRE NX)

	for _, k := range []string{"rate:ext:token:client:" + rlClient, "rate:ext:token:ip:" + rlIP} {
		if got := mr.TTL(k); got != 10*time.Minute {
			t.Fatalf("%s TTL = %v, want 10m (window not extended)", k, got)
		}
	}
	if d := mustCheck(t, f, rlClient, rlIP); d != 10*time.Minute {
		t.Fatalf("retryAfter = %v, want 10m", d)
	}
	mr.FastForward(10 * time.Minute)
	if d := mustCheck(t, f, rlClient, rlIP); d != 0 {
		t.Fatalf("after window retryAfter = %v, want 0", d)
	}
}

func TestTokenFailures_KeyWithoutTTLReportsFullWindow(t *testing.T) {
	mr, rdb := newMiniRedis(t)
	f := NewTokenFailures(rdb, 3, rlWindow)
	if err := mr.Set("rate:ext:token:ip:"+rlIP, "5"); err != nil {
		t.Fatal(err)
	}
	if d := mustCheck(t, f, rlClient, rlIP); d != rlWindow {
		t.Fatalf("retryAfter = %v, want %v", d, rlWindow)
	}
}

func TestTokenFailures_ResetClient(t *testing.T) {
	_, rdb := newMiniRedis(t)
	f := NewTokenFailures(rdb, 3, rlWindow)
	ctx := context.Background()
	recordN(t, f, 3, rlClient, rlIP)
	if err := f.ResetClient(ctx, rlClient); err != nil {
		t.Fatal(err)
	}
	if d := mustCheck(t, f, rlClient, "10.0.0.9"); d != 0 {
		t.Fatalf("client still blocked after reset: %v", d)
	}
	if d := mustCheck(t, f, rlClient2, rlIP); d == 0 {
		t.Fatal("ip counter must survive ResetClient")
	}
	if err := f.ResetClient(ctx, "bogus"); err != nil {
		t.Fatalf("ResetClient invalid id: %v", err)
	}
}

func TestAPIRate_LimitAndNextMinute(t *testing.T) {
	mr, rdb := newMiniRedis(t)
	a := NewAPIRate(rdb, 2)
	now := time.Unix(1_800_000_000/60*60+45, 0) // 45 s into a minute
	a.now = func() time.Time { return now }
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		if d, err := a.Allow(ctx, rlClient); err != nil || d != 0 {
			t.Fatalf("request %d: retryAfter=%v err=%v", i+1, d, err)
		}
	}
	if d, err := a.Allow(ctx, rlClient); err != nil || d != 15*time.Second {
		t.Fatalf("over limit: retryAfter=%v err=%v, want 15s", d, err)
	}
	if d, err := a.Allow(ctx, rlClient2); err != nil || d != 0 {
		t.Fatalf("other client: retryAfter=%v err=%v", d, err)
	}
	key := "rate:ext:api:" + rlClient + ":" + strconv.FormatInt(now.Unix()/60, 10)
	if got := mr.TTL(key); got != apiKeyTTL {
		t.Fatalf("api key TTL = %v, want %v", got, apiKeyTTL)
	}

	now = now.Add(15 * time.Second) // next minute starts a fresh counter
	if d, err := a.Allow(ctx, rlClient); err != nil || d != 0 {
		t.Fatalf("next minute: retryAfter=%v err=%v", d, err)
	}
	mr.FastForward(apiKeyTTL)
	if mr.Exists(key) {
		t.Fatal("old minute key not expired")
	}
}

func TestRateLimit_FailClosed(t *testing.T) {
	mr, rdb := newMiniRedis(t)
	f := NewTokenFailures(rdb, 3, rlWindow)
	a := NewAPIRate(rdb, 60)
	mr.Close()
	ctx := context.Background()

	d1, errCheck := f.Check(ctx, rlClient, rlIP)
	d2, errAllow := a.Allow(ctx, rlClient)
	if d1 != 0 || d2 != 0 {
		t.Errorf("retryAfter on failure = %v, %v; want 0", d1, d2)
	}
	for name, err := range map[string]error{
		"Check":         errCheck,
		"RecordFailure": f.RecordFailure(ctx, rlClient, rlIP),
		"ResetClient":   f.ResetClient(ctx, rlClient),
		"Allow":         errAllow,
	} {
		if !errors.Is(err, ErrUnavailable) {
			t.Errorf("%s: err = %v, want ErrUnavailable", name, err)
		}
	}
}
