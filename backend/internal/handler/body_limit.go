package handler

import (
	"net/http"
	"strings"
)

// JSONBodyLimit is the global request-body cap (BE-08) set in cmd/api/main.go.
// Upload handlers exempted by IsBodyLimitExempt cap their own bodies higher.
const JSONBodyLimit = 1 << 20 // 1 MiB

// IsBodyLimitExempt reports whether r is a multipart upload whose handler sets
// its own, larger http.MaxBytesReader: the DSR workbook (10 MiB) and the
// master-data CSV import (5 MiB). The holiday CSV import is not listed: its own
// cap is already 1 MiB for the whole body, so the global limit changes nothing.
func IsBodyLimitExempt(r *http.Request) bool {
	if r.Method != http.MethodPost {
		return false
	}
	return r.URL.Path == "/api/v1/dsr/uploads" ||
		strings.HasPrefix(r.URL.Path, "/api/v1/admin/master-data/import/")
}
