package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// RequestLogger writes one structured line per request (msg "http_request")
// through logger: method, route (chi pattern, low cardinality for log-based
// metrics), path, status, duration_ms, bytes, request_id, remote_ip. Level is
// Error for 5xx, Warn for 4xx, Info otherwise. Successful /health and /ready
// probes are skipped (Docker hits them every 10 s).
//
// Never logs headers, query strings or bodies: they can carry JWTs or secrets
// (CLAUDE.md Sec 9). Place it after RequestID and TrustedRealIP and before
// Recoverer, so recovered panics are logged as 500.
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			status := ww.Status()
			if status == 0 { // handler wrote nothing: net/http sends 200
				status = http.StatusOK
			}
			if status < 300 && (r.URL.Path == "/health" || r.URL.Path == "/ready") {
				return
			}

			route := "unmatched"
			if rc := chi.RouteContext(r.Context()); rc != nil && rc.RoutePattern() != "" {
				route = rc.RoutePattern()
			}

			remoteIP := r.RemoteAddr
			if ip, ok := parseRemoteAddr(r.RemoteAddr); ok {
				remoteIP = ip.String() // drop the port
			}

			level := slog.LevelInfo
			switch {
			case status >= 500:
				level = slog.LevelError
			case status >= 400:
				level = slog.LevelWarn
			}

			// Background ctx: the request ctx may already be cancelled (Timeout,
			// client gone) and the line must still be written.
			logger.LogAttrs(context.Background(), level, "http_request",
				slog.String("method", r.Method),
				slog.String("route", route),
				slog.String("path", r.URL.Path),
				slog.Int("status", status),
				slog.Float64("duration_ms", float64(time.Since(start).Microseconds())/1000),
				slog.Int("bytes", ww.BytesWritten()),
				slog.String("request_id", chimw.GetReqID(r.Context())),
				slog.String("remote_ip", remoteIP),
			)
		})
	}
}
