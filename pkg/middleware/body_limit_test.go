package middleware

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBodyLimit(t *testing.T) {
	const limit = 16
	isUpload := func(r *http.Request) bool { return r.URL.Path == "/upload" }

	tests := []struct {
		name    string
		path    string
		exempt  func(*http.Request) bool
		size    int
		wantErr bool
	}{
		{"at limit is read fully", "/json", isUpload, limit, false},
		{"one byte over is rejected", "/json", isUpload, limit + 1, true},
		{"exempt path is not capped", "/upload", isUpload, limit * 10, false},
		{"nil exempt caps everything", "/upload", nil, limit + 1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var readErr error
			var n int
			h := BodyLimit(limit, tt.exempt)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				b, err := io.ReadAll(r.Body)
				n, readErr = len(b), err
			}))
			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(strings.Repeat("a", tt.size)))
			h.ServeHTTP(httptest.NewRecorder(), req)

			var tooBig *http.MaxBytesError
			if got := errors.As(readErr, &tooBig); got != tt.wantErr {
				t.Fatalf("MaxBytesError = %v (err %v); want %v", got, readErr, tt.wantErr)
			}
			if !tt.wantErr && n != tt.size {
				t.Errorf("read %d bytes; want %d", n, tt.size)
			}
		})
	}
}
