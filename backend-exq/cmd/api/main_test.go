package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend-exq/internal/exportjob"
	"github.com/cimb-niaga/cms/backend-exq/internal/handler"
)

// spySvc records which service method a request reached and the ctx budget it had.
type spySvc struct {
	called string
	budget time.Duration
}

func (s *spySvc) note(ctx context.Context, name string) error {
	s.called = name
	if dl, ok := ctx.Deadline(); ok {
		s.budget = time.Until(dl)
	}
	return errors.New("stop here")
}

func (s *spySvc) Create(ctx context.Context, _ exportjob.Actor, _ exportjob.Params) (exportjob.Job, bool, error) {
	return exportjob.Job{}, false, s.note(ctx, "Create")
}
func (s *spySvc) List(ctx context.Context, _ exportjob.Actor, _ int) ([]exportjob.Job, error) {
	return nil, s.note(ctx, "List")
}
func (s *spySvc) Get(ctx context.Context, _ exportjob.Actor, _ int64) (exportjob.Job, error) {
	return exportjob.Job{}, s.note(ctx, "Get")
}
func (s *spySvc) StartDownload(ctx context.Context, _ exportjob.Actor, _ int64) (exportjob.Job, error) {
	return exportjob.Job{}, s.note(ctx, "StartDownload")
}
func (s *spySvc) OpenChunks(context.Context, int64, int64) exportjob.ChunkIterator { return nil }
func (s *spySvc) CompleteDownload(context.Context, int64) error                    { return nil }
func (s *spySvc) AbortDownload(context.Context, int64) error                       { return nil }

func TestMountExportsRoutesAndTimeouts(t *testing.T) {
	passAuth := func(next http.Handler) http.Handler { return next }
	denyAuth := func(http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) })
	}

	tests := []struct {
		name       string
		path       string
		wantCall   string
		wantBudget time.Duration
	}{
		{"download wins over mounted catch-all, 5 min lease", "/api/v1/exports/jobs/7/file", "StartDownload", exportjob.DownloadLease},
		{"job status uses mounted routes, 30 s", "/api/v1/exports/jobs/7", "Get", 30 * time.Second},
		{"list uses mounted routes, 30 s", "/api/v1/exports/jobs", "List", 30 * time.Second},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spy := &spySvc{}
			r := chi.NewRouter()
			mountExports(r, passAuth, handler.NewExportsHandler(spy, nil))

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))

			if spy.called != tc.wantCall {
				t.Fatalf("service call = %q, want %q (status %d)", spy.called, tc.wantCall, rec.Code)
			}
			if spy.budget <= tc.wantBudget-5*time.Second || spy.budget > tc.wantBudget {
				t.Fatalf("ctx budget = %s, want ~%s", spy.budget, tc.wantBudget)
			}
		})
	}

	t.Run("auth guards the download route too", func(t *testing.T) {
		spy := &spySvc{}
		r := chi.NewRouter()
		mountExports(r, denyAuth, handler.NewExportsHandler(spy, nil))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/exports/jobs/7/file", nil))
		if rec.Code != http.StatusUnauthorized || spy.called != "" {
			t.Fatalf("status = %d, called = %q", rec.Code, spy.called)
		}
	})
}
