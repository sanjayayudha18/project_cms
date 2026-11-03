// Package health serves the readiness probe (/ready): unlike /health
// (liveness, always 200), it pings the dependencies a backend needs to serve.
package health

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// Ready runs every check in parallel, each bounded by timeout, and answers
// 200 {"status":"ok","checks":{name:"ok"}} when all pass, else 503 with the
// failing names marked "fail". Error details are logged, never sent to the
// client (they can carry hosts or credentials from DSNs).
func Ready(timeout time.Duration, checks map[string]func(context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		var (
			mu      sync.Mutex
			wg      sync.WaitGroup
			results = make(map[string]string, len(checks))
			ok      = true
		)
		for name, check := range checks {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := check(ctx)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					ok = false
					results[name] = "fail"
					slog.Warn("readiness check failed", "check", name, "error", err)
					return
				}
				results[name] = "ok"
			}()
		}
		wg.Wait()

		status, code := "ok", http.StatusOK
		if !ok {
			status, code = "unavailable", http.StatusServiceUnavailable
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": status, "checks": results})
	}
}
