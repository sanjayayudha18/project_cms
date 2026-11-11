package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

func newLoggedRouter(t *testing.T) (*chi.Mux, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(RequestLogger(logger))
	r.Use(chimw.Recoverer)
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	r.Get("/ready", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) })
	r.Route("/api/v1/items", func(r chi.Router) {
		r.Get("/{id}", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("hello")) })
		r.Post("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadRequest) })
		r.Get("/boom", func(http.ResponseWriter, *http.Request) { panic("boom") })
	})
	return r, &buf
}

func lastHTTPLine(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	for _, l := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var m map[string]any
		if json.Unmarshal([]byte(l), &m) == nil && m["msg"] == "http_request" {
			return m
		}
	}
	t.Fatalf("no http_request line in %q", buf.String())
	return nil
}

func TestRequestLoggerClientID(t *testing.T) {
	serve := func(h http.HandlerFunc, hdr map[string]string) map[string]any {
		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, nil))
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		RequestLogger(logger)(h).ServeHTTP(httptest.NewRecorder(), req)
		return lastHTTPLine(t, &buf)
	}

	t.Run("holder filled adds client_id", func(t *testing.T) {
		line := serve(func(_ http.ResponseWriter, r *http.Request) {
			LogFieldsFrom(r.Context()).ClientID = "idm-prod"
		}, nil)
		if line["client_id"] != "idm-prod" {
			t.Errorf("client_id = %v", line["client_id"])
		}
	})
	t.Run("holder empty keeps the original attribute set", func(t *testing.T) {
		line := serve(func(http.ResponseWriter, *http.Request) {}, nil)
		// time, level, msg + method, route, path, status, duration_ms, bytes, request_id, remote_ip
		if len(line) != 11 {
			t.Errorf("attribute set changed: %v", line)
		}
		if _, ok := line["client_id"]; ok {
			t.Errorf("unexpected client_id: %v", line)
		}
	})
	t.Run("never read from request headers", func(t *testing.T) {
		line := serve(func(http.ResponseWriter, *http.Request) {},
			map[string]string{"X-Client-Id": "evil", "Client-Id": "evil"})
		if _, ok := line["client_id"]; ok {
			t.Errorf("client_id taken from header: %v", line)
		}
	})
	t.Run("LogFieldsFrom without logger is nil", func(t *testing.T) {
		if LogFieldsFrom(httptest.NewRequest(http.MethodGet, "/", nil).Context()) != nil {
			t.Error("want nil")
		}
	})
}

func TestRequestLogger(t *testing.T) {
	tests := []struct {
		name      string
		method    string
		target    string
		wantLog   bool
		wantLevel string
		wantRoute string
		status    float64
		bytes     float64
	}{
		{"2xx info with route pattern", http.MethodGet, "/api/v1/items/42?token=secret", true, "INFO", "/api/v1/items/{id}", 200, 5},
		{"4xx warn", http.MethodPost, "/api/v1/items/", true, "WARN", "/api/v1/items", 400, 0},
		{"panic logged as 5xx error", http.MethodGet, "/api/v1/items/boom", true, "ERROR", "/api/v1/items/boom", 500, 0},
		{"unmatched route", http.MethodGet, "/nope", true, "WARN", "unmatched", 404, 19},
		{"health 2xx skipped", http.MethodGet, "/health", false, "", "", 0, 0},
		{"ready non-2xx still logged", http.MethodGet, "/ready", true, "ERROR", "/ready", 503, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, buf := newLoggedRouter(t)
			req := httptest.NewRequest(tt.method, tt.target, nil)
			req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiJ9.secret")
			r.ServeHTTP(httptest.NewRecorder(), req)

			out := buf.String()
			if strings.Contains(out, "eyJ") || strings.Contains(out, "token=secret") || strings.Contains(out, "Bearer") {
				t.Fatalf("log leaks auth header or query: %s", out)
			}
			var line map[string]any
			for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
				var m map[string]any
				if json.Unmarshal([]byte(l), &m) == nil && m["msg"] == "http_request" {
					line = m
				}
			}
			if !tt.wantLog {
				if line != nil {
					t.Fatalf("expected no http_request line, got %v", line)
				}
				return
			}
			if line == nil {
				t.Fatalf("no http_request line in %q", out)
			}
			if line["level"] != tt.wantLevel || line["route"] != tt.wantRoute || line["status"] != tt.status || line["bytes"] != tt.bytes {
				t.Errorf("got level=%v route=%v status=%v bytes=%v", line["level"], line["route"], line["status"], line["bytes"])
			}
			if line["method"] != tt.method || line["request_id"] == "" || line["request_id"] == nil {
				t.Errorf("missing method/request_id: %v", line)
			}
			if line["remote_ip"] != "192.0.2.1" { // httptest RemoteAddr 192.0.2.1:1234, port dropped
				t.Errorf("remote_ip = %v", line["remote_ip"])
			}
			if _, ok := line["duration_ms"].(float64); !ok {
				t.Errorf("duration_ms not numeric: %v", line["duration_ms"])
			}
		})
	}
}
