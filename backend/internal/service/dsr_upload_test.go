package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// fakeDsrRepository implements DsrRepository for unit tests. Every method not
// explicitly stubbed by a test returns pgx.ErrNoRows / empty, matching what a
// fresh, empty DB would return.
type fakeDsrRepository struct {
	upload *db.GetDsrUploadByChecksumRow
}

func (f *fakeDsrRepository) GetVendorByID(ctx context.Context, id int64) (db.GetVendorByIDRow, error) {
	return db.GetVendorByIDRow{ID: id, Code: "BIJAK", Name: "Bijak"}, nil
}

func (f *fakeDsrRepository) GetDsrUploadByChecksum(ctx context.Context, checksum string) (db.GetDsrUploadByChecksumRow, error) {
	if f.upload != nil {
		return *f.upload, nil
	}
	return db.GetDsrUploadByChecksumRow{}, pgx.ErrNoRows
}

func (f *fakeDsrRepository) GetDsrUploadByIDForVendor(ctx context.Context, arg db.GetDsrUploadByIDForVendorParams) (db.GetDsrUploadByIDForVendorRow, error) {
	return db.GetDsrUploadByIDForVendorRow{}, pgx.ErrNoRows
}

func (f *fakeDsrRepository) ListDsrDailyRowErrors(ctx context.Context, uploadID int64) ([]db.ListDsrDailyRowErrorsRow, error) {
	return nil, nil
}

func (f *fakeDsrRepository) ListDsrRencanaIsiRowErrors(ctx context.Context, uploadID int64) ([]db.ListDsrRencanaIsiRowErrorsRow, error) {
	return nil, nil
}

func (f *fakeDsrRepository) ListDsrDailyRows(ctx context.Context, uploadID int64) ([]db.ListDsrDailyRowsRow, error) {
	return nil, nil
}

func (f *fakeDsrRepository) ListDsrRencanaIsiRows(ctx context.Context, uploadID int64) ([]db.ListDsrRencanaIsiRowsRow, error) {
	return nil, nil
}

func (f *fakeDsrRepository) ListDsrUploadsByVendor(ctx context.Context, arg db.ListDsrUploadsByVendorParams) ([]db.ListDsrUploadsByVendorRow, error) {
	return nil, nil
}

func (f *fakeDsrRepository) CountDsrUploadsByVendor(ctx context.Context, arg db.CountDsrUploadsByVendorParams) (int64, error) {
	return 0, nil
}

func TestWriteAndDryRun_WritesFileAndRelaysParsedPayload(t *testing.T) {
	uploadDir := t.TempDir()

	var capturedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-secret" {
			t.Errorf("expected Authorization header to be forwarded, got %q", auth)
		}
		var body struct {
			Filename string `json:"filename"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data": map[string]any{
				"checksum":        "abc123",
				"staged_filename": body.Filename,
				"daily":           map[string]any{"rows": []any{}},
			},
			"error": nil,
		})
	}))
	defer server.Close()

	svc := NewDsrService(&fakeDsrRepository{}, nil, server.Client(), uploadDir, server.URL, "Bearer test-secret")

	result, err := svc.WriteAndDryRun(context.Background(), "BIJAK", 7, "laporan.xlsx", []byte("fake xlsx content"))
	if err != nil {
		t.Fatalf("WriteAndDryRun failed: %v", err)
	}

	if capturedPath != "/process/dsr/dry-run" {
		t.Errorf("expected dry-run endpoint to be called, got %q", capturedPath)
	}
	if result.Checksum != "abc123" {
		t.Errorf("expected checksum abc123, got %q", result.Checksum)
	}
	wantStaged := "BIJAK__7__laporan.xlsx"
	if result.StagedFilename != wantStaged {
		t.Errorf("expected staged filename %q, got %q", wantStaged, result.StagedFilename)
	}

	if _, err := os.Stat(filepath.Join(uploadDir, wantStaged)); err != nil {
		t.Errorf("expected file written to upload dir: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(result.Raw, &raw); err != nil {
		t.Fatalf("Raw is not valid JSON: %v", err)
	}
	if _, ok := raw["daily"]; !ok {
		t.Error("expected Raw to contain the full parsed payload (daily key)")
	}
}

func TestConfirmUpload_ReadsBackAfterCommitSucceeds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process/dsr/commit" {
			t.Errorf("expected commit endpoint, got %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data":   map[string]any{"mode": "commit", "result": "daily=completed rencana_isi=completed"},
			"error":  nil,
		})
	}))
	defer server.Close()

	uploadRow := db.GetDsrUploadByChecksumRow{
		ID: 1, DailyStatus: "completed", DailyRowCount: int32Ptr(17), DailyErrorCount: int32Ptr(2),
		RencanaIsiStatus: "pending",
	}
	repo := &fakeDsrRepository{upload: &uploadRow}
	svc := NewDsrService(repo, nil, server.Client(), t.TempDir(), server.URL, "")

	result, err := svc.ConfirmUpload(context.Background(), "BIJAK__7__laporan.xlsx", "abc123")
	if err != nil {
		t.Fatalf("ConfirmUpload failed: %v", err)
	}
	if result.Daily.Status != "completed" {
		t.Errorf("expected daily status completed, got %q", result.Daily.Status)
	}
	if result.Daily.RowCount != 17 {
		t.Errorf("expected daily row_count 17, got %d", result.Daily.RowCount)
	}
	// success is derived (rows - errors), never read from a stored column.
	if result.Daily.SuccessCount != 15 {
		t.Errorf("expected daily success_count 15 (17 rows - 2 errors), got %d", result.Daily.SuccessCount)
	}
	if result.RencanaIsi.Status != "pending" {
		t.Errorf("expected rencana_isi status pending (sheet not ingested), got %q", result.RencanaIsi.Status)
	}
	// Both sheets report the same upload id -- the API contract kept from the
	// old two-table schema.
	if result.Daily.FileID == nil || result.RencanaIsi.FileID == nil || *result.Daily.FileID != *result.RencanaIsi.FileID {
		t.Error("expected both sheets to carry the same upload file_id")
	}
}

func TestConfirmUpload_PropagatesCommitFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": "error", "data": nil, "error": "staged file not found",
		})
	}))
	defer server.Close()

	svc := NewDsrService(&fakeDsrRepository{}, nil, server.Client(), t.TempDir(), server.URL, "")

	_, err := svc.ConfirmUpload(context.Background(), "BIJAK__7__missing.xlsx", "abc123")
	if err == nil {
		t.Fatal("expected ConfirmUpload to fail when service_dsr_etl reports an error")
	}
}

func int32Ptr(v int32) *int32 { return &v }
