package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestReady(t *testing.T) {
	pass := func(context.Context) error { return nil }
	failSecret := func(context.Context) error { return errors.New("dial postgres://u:secretpw@db:5432") }
	hang := func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }

	tests := []struct {
		name       string
		checks     map[string]func(context.Context) error
		wantCode   int
		wantStatus string
		wantChecks map[string]string
	}{
		{"all ok", map[string]func(context.Context) error{"db": pass, "redis": pass}, 200, "ok", map[string]string{"db": "ok", "redis": "ok"}},
		{"one fails", map[string]func(context.Context) error{"db": failSecret, "redis": pass}, 503, "unavailable", map[string]string{"db": "fail", "redis": "ok"}},
		{"timeout fails", map[string]func(context.Context) error{"redis": hang}, 503, "unavailable", map[string]string{"redis": "fail"}},
		{"no checks", nil, 200, "ok", map[string]string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			start := time.Now()
			Ready(50*time.Millisecond, tt.checks)(w, httptest.NewRequest(http.MethodGet, "/ready", nil))
			if time.Since(start) > time.Second {
				t.Fatalf("timeout not enforced")
			}
			if w.Code != tt.wantCode {
				t.Fatalf("code = %d, want %d", w.Code, tt.wantCode)
			}
			if strings.Contains(w.Body.String(), "secretpw") {
				t.Fatalf("error detail leaked to client: %s", w.Body.String())
			}
			var body struct {
				Status string            `json:"status"`
				Checks map[string]string `json:"checks"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if body.Status != tt.wantStatus || len(body.Checks) != len(tt.wantChecks) {
				t.Fatalf("got %+v", body)
			}
			for k, v := range tt.wantChecks {
				if body.Checks[k] != v {
					t.Errorf("check %s = %q, want %q", k, body.Checks[k], v)
				}
			}
		})
	}
}
