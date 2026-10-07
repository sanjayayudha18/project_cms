package handler

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/approval"
)

func TestWriteUnexpectedError(t *testing.T) {
	cases := map[string]struct {
		err  error
		want int
	}{
		"chain incomplete": {fmt.Errorf("submit: %w", approval.ErrChainIncomplete), http.StatusUnprocessableEntity},
		"other":            {errors.New("db down"), http.StatusInternalServerError},
	}
	for name, tc := range cases {
		rec := httptest.NewRecorder()
		writeUnexpectedError(rec, tc.err)
		if rec.Code != tc.want {
			t.Errorf("%s: status %d, want %d", name, rec.Code, tc.want)
		}
	}
}
