package response

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The envelope is the contract backend-exq clients parse: success flag always
// present, data/error/meta omitted when empty.
func TestEnvelopeWriters(t *testing.T) {
	cases := []struct {
		name  string
		write func(http.ResponseWriter)
		code  int
		body  string
	}{
		{"success", func(w http.ResponseWriter) { WriteSuccess(w, map[string]int{"id": 1}) }, http.StatusOK,
			`{"success":true,"data":{"id":1}}`},
		{"created", func(w http.ResponseWriter) { WriteCreated(w, "ok") }, http.StatusCreated,
			`{"success":true,"data":"ok"}`},
		{"error", func(w http.ResponseWriter) { WriteError(w, http.StatusNotFound, "not_found", "Tidak ditemukan") }, http.StatusNotFound,
			`{"success":false,"error":{"code":"not_found","message":"Tidak ditemukan"}}`},
		{"validation", func(w http.ResponseWriter) {
			WriteValidationError(w, []FieldError{{Field: "amount", Message: "wajib diisi"}})
		}, http.StatusUnprocessableEntity,
			`{"success":false,"error":{"code":"validation_error","message":"Validasi gagal","details":[{"field":"amount","message":"wajib diisi"}]}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tc.write(rec)
			if rec.Code != tc.code || rec.Header().Get("Content-Type") != "application/json" || rec.Body.String() != tc.body+"\n" {
				t.Fatalf("= %d %q %s, want %d application/json %s", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String(), tc.code, tc.body)
			}
		})
	}
}
