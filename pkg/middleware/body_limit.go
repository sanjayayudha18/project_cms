package middleware

import "net/http"

// BodyLimit caps every request body at max bytes via http.MaxBytesReader, so a
// JSON handler's decoder fails (and the handler answers 4xx) instead of reading
// an unbounded body. Requests for which exempt returns true (upload endpoints
// that set their own, larger MaxBytesReader) pass through untouched. A nil
// exempt exempts nothing.
func BodyLimit(max int64, exempt func(*http.Request) bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil && (exempt == nil || !exempt(r)) {
				r.Body = http.MaxBytesReader(w, r.Body, max)
			}
			next.ServeHTTP(w, r)
		})
	}
}
