package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

func TestIsBodyLimitExempt(t *testing.T) {
	tests := []struct {
		method, path string
		want         bool
	}{
		{http.MethodPost, "/api/v1/dsr/uploads", true},
		{http.MethodPost, "/api/v1/dsr/uploads/confirm", false},
		{http.MethodGet, "/api/v1/dsr/uploads", false},
		{http.MethodPost, "/api/v1/admin/master-data/import/atms", true},
		{http.MethodPost, "/api/v1/admin/master-data/import/atms/confirm", true},
		{http.MethodPost, "/api/v1/admin/holidays/import", false},
		{http.MethodPut, "/api/v1/admin/roles/7/permissions", false},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			if got := IsBodyLimitExempt(httptest.NewRequest(tt.method, tt.path, nil)); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// TestBodyLimit_Router mirrors the main.go wiring: a JSON route rejects a body
// one byte over the cap, while the exempt DSR upload still takes a > 1 MiB file.
func TestBodyLimit_Router(t *testing.T) {
	limit := custommw.BodyLimit(JSONBodyLimit, IsBodyLimitExempt)

	rolesRouter, rolesTokens := mountRoleMgmtHandler(&fakeRoleMgmtServicer{updateResult: []int64{1}})
	roles := chi.NewRouter()
	roles.Use(limit)
	roles.Mount("/", rolesRouter)

	// The padding sits inside the JSON value: json.Decoder stops after the first
	// value, so bytes trailing it are never read and never hit the cap.
	jsonOfSize := func(n int) string {
		head, tail := `{"menu_feature_ids":[1],"pad":"`, `"}`
		return head + strings.Repeat("a", n-len(head)-len(tail)) + tail
	}
	body := jsonOfSize(JSONBodyLimit + 1)
	rec := doRequest(roles, http.MethodPut, "/api/v1/admin/roles/7/permissions", tokenForRole(t, rolesTokens, 1, "ADMIN"), body)
	if rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("JSON body 1 MiB + 1 = %d, want 4xx: %s", rec.Code, rec.Body.String())
	}
	rec = doRequest(roles, http.MethodPut, "/api/v1/admin/roles/7/permissions", tokenForRole(t, rolesTokens, 1, "ADMIN"), jsonOfSize(JSONBodyLimit))
	if rec.Code != http.StatusOK {
		t.Fatalf("JSON body at 1 MiB = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	dsrRouter, dsrTokens, _ := mountDsrDetailHandler(t, "http://127.0.0.1:0")
	dsr := chi.NewRouter()
	dsr.Use(limit)
	dsr.Mount("/", dsrRouter)

	xlsx := append([]byte("PK\x03\x04"), make([]byte, 2<<20)...)
	rec = httptest.NewRecorder()
	dsr.ServeHTTP(rec, dsrUploadRequest(t, tokenForVendor(t, dsrTokens, 1, 7), "dsr.xlsx", xlsx))
	// Past the body cap and validation, the dry-run fails on the unreachable ETL.
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("2 MiB DSR upload = %d, want 502 (reached dry-run): %s", rec.Code, rec.Body.String())
	}
}
