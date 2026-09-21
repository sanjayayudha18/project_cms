package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/cimb-niaga/cms/pkg/middleware"
)

// importRateLimit caps dry-run + confirm calls per user per hour: each one
// parses up to MasterDataImportMaxRows rows and reads current state per row.
const importRateLimit = 20

type importRateLimiter struct {
	rdb redis.Cmdable
	max int64
}

// WithRateLimit enables the per-user hourly cap on /import (Redis INCR+EXPIRE,
// same shape as DsrService.CheckUploadRate).
func (h *AdminMasterDataImportHandler) WithRateLimit(rdb redis.Cmdable) *AdminMasterDataImportHandler {
	h.limiter = &importRateLimiter{rdb: rdb, max: importRateLimit}
	return h
}

func (h *AdminMasterDataImportHandler) rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := middleware.GetAuthContext(r.Context())
		if h.limiter == nil || !ok {
			next.ServeHTTP(w, r)
			return
		}
		key := fmt.Sprintf("mdimport:%d:%s", authCtx.UserID, time.Now().UTC().Format("2006010215"))
		n, err := h.limiter.rdb.Incr(r.Context(), key).Result()
		if err != nil {
			// ponytail: fail open -- Redis outage must not block admins; the 2000-row cap still bounds each call.
			slog.Warn("import rate limit unavailable", "error", err)
			next.ServeHTTP(w, r)
			return
		}
		if n == 1 {
			h.limiter.rdb.Expire(r.Context(), key, time.Hour)
		}
		if n > h.limiter.max {
			w.Header().Set("Retry-After", "3600")
			writeError(w, http.StatusTooManyRequests, "rate_limited", "Terlalu banyak impor, coba lagi dalam satu jam")
			return
		}
		next.ServeHTTP(w, r)
	})
}
