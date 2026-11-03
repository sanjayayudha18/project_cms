package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

// countingDetail records reads so a 403 can prove no data was touched.
type countingDetail struct {
	fakeMasterDataDetail
	reads int
}

func (c *countingDetail) GetByID(ctx context.Context, id int64) (db.MasterDataChangeRequest, error) {
	c.reads++
	return c.fakeMasterDataDetail.GetByID(ctx, id)
}

func (c *countingDetail) ListByBatch(ctx context.Context, id int64) ([]db.MasterDataChangeRequest, error) {
	c.reads++
	return c.fakeMasterDataDetail.ListByBatch(ctx, id)
}

type fakeBatchReader struct {
	row   db.GetMasterDataImportBatchRow
	err   error
	reads int
}

func (f *fakeBatchReader) GetBatch(context.Context, int64) (db.GetMasterDataImportBatchRow, error) {
	f.reads++
	return f.row, f.err
}

func mountDetailWithBatch(reader ApprovalReader, detail MasterDataApprovalDetailReader, batches MasterDataApprovalBatchReader) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
	}, noopBlacklist{})
	h := NewApprovalHandler(&fakeApprovalOrchestrator{}, reader).WithMasterDataDetail(detail)
	if batches != nil {
		h.WithMasterDataBatches(batches)
	}
	r := chi.NewRouter()
	r.With(custommw.RequireAuth(tokenSvc)).Mount("/api/v1/approvals", h.Routes())
	return r, tokenSvc
}

func batchRows(n int) (db.MasterDataChangeRequest, []db.MasterDataChangeRequest) {
	batchID := int64(3)
	rows := make([]db.MasterDataChangeRequest, n)
	for i := range rows {
		op := "create"
		if i%4 == 0 {
			op = "update"
		}
		rows[i] = db.MasterDataChangeRequest{ID: int64(1000 + i), EntityType: "vendor", Op: op, BatchID: &batchID, Payload: []byte(`{}`)}
	}
	return rows[0], rows
}

type detailBody struct {
	Total     int              `json:"total"`
	Truncated bool             `json:"truncated"`
	Page      int              `json:"page"`
	PageSize  int              `json:"page_size"`
	Counts    map[string]int   `json:"counts"`
	Changes   []map[string]any `json:"changes"`
	Batch     map[string]any   `json:"batch"`
}

func TestApprovalMasterDataDetail_Paging(t *testing.T) {
	head, rows := batchRows(430)
	wantUpdates := (len(rows) + 3) / 4
	cases := []struct {
		name          string
		query         string
		wantLen       int
		wantFirstID   float64
		wantTruncated bool
		wantPage      int
		wantPageSize  int
	}{
		{"no params keeps the legacy 200 cap", "", 200, 1000, true, 1, 200},
		{"page 2 size 50", "?page=2&page_size=50", 50, 1050, true, 2, 50},
		{"last page is not truncated", "?page=3&page_size=200", 30, 1400, false, 3, 200},
		{"page beyond range is an empty array", "?page=99&page_size=50", 0, 0, false, 99, 50},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, ts := mountDetailWithBatch(&fakeApprovalReader{getResult: mdRequest(5)}, &fakeMasterDataDetail{head: head, batch: rows}, nil)

			rec := doRequest(router, http.MethodGet, "/api/v1/approvals/7/master-data"+tc.query, tokenFor(t, ts, 5), "")

			if rec.Code != http.StatusOK {
				t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
			}
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
				t.Fatal(err)
			}
			if string(raw["changes"]) == "null" {
				t.Fatalf("changes must be an array, never null: %s", rec.Body.String())
			}
			var body detailBody
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if len(body.Changes) != tc.wantLen || body.Truncated != tc.wantTruncated ||
				body.Page != tc.wantPage || body.PageSize != tc.wantPageSize {
				t.Errorf("len=%d truncated=%v page=%d size=%d", len(body.Changes), body.Truncated, body.Page, body.PageSize)
			}
			if tc.wantLen > 0 && body.Changes[0]["id"] != tc.wantFirstID {
				t.Errorf("first id = %v, want %v", body.Changes[0]["id"], tc.wantFirstID)
			}
			if body.Total != len(rows) || body.Counts["update"] != wantUpdates || body.Counts["create"]+body.Counts["update"] != len(rows) {
				t.Errorf("total/counts must cover the whole batch: total=%d counts=%v", body.Total, body.Counts)
			}
			if body.Batch != nil {
				t.Errorf("no batch reader wired, yet batch = %v", body.Batch)
			}
		})
	}
}

func TestApprovalMasterDataDetail_InvalidPaging_400(t *testing.T) {
	head, rows := batchRows(5)
	for _, q := range []string{
		"?page_size=201", "?page_size=0", "?page_size=-1", "?page_size=abc",
		"?page=0", "?page=-2", "?page=x", "?page=1000001",
	} {
		t.Run(q, func(t *testing.T) {
			router, ts := mountDetailWithBatch(&fakeApprovalReader{getResult: mdRequest(5)}, &fakeMasterDataDetail{head: head, batch: rows}, nil)

			rec := doRequest(router, http.MethodGet, "/api/v1/approvals/7/master-data"+q, tokenFor(t, ts, 5), "")

			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestApprovalMasterDataDetail_SingleRecordIgnoresPagingAndHasNoBatch(t *testing.T) {
	head := db.MasterDataChangeRequest{ID: 40, EntityType: "vendor", Op: "update", Payload: []byte(`{"name":"Baru"}`), Status: "pending"}
	for _, q := range []string{"?page=1&page_size=50", "?page=7&page_size=50"} {
		t.Run(q, func(t *testing.T) {
			batches := &fakeBatchReader{row: db.GetMasterDataImportBatchRow{ID: 1}}
			router, ts := mountDetailWithBatch(&fakeApprovalReader{getResult: mdRequest(5)}, &fakeMasterDataDetail{head: head}, batches)

			rec := doRequest(router, http.MethodGet, "/api/v1/approvals/7/master-data"+q, tokenFor(t, ts, 5), "")

			var body detailBody
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if rec.Code != http.StatusOK || len(body.Changes) != 1 || body.Total != 1 || body.Truncated || body.Batch != nil {
				t.Errorf("code=%d body=%s", rec.Code, rec.Body.String())
			}
			if batches.reads != 0 {
				t.Errorf("single-record request must not read a batch header")
			}
		})
	}
}

func TestApprovalMasterDataDetail_BatchObject(t *testing.T) {
	head, rows := batchRows(3)
	name := "vendors.csv"
	cases := []struct {
		name     string
		batches  MasterDataApprovalBatchReader
		wantCode int
		wantNil  bool
		wantFile any
	}{
		{"with file name", &fakeBatchReader{row: db.GetMasterDataImportBatchRow{ID: 3, Entity: "vendors", FileName: &name, MakerName: "Budi", RowCount: 3}}, 200, false, "vendors.csv"},
		{"null file name", &fakeBatchReader{row: db.GetMasterDataImportBatchRow{ID: 3, Entity: "vendors", MakerName: "Budi", RowCount: 3}}, 200, false, nil},
		{"no reader wired", nil, 200, true, nil},
		{"batch not found", &fakeBatchReader{err: pgx.ErrNoRows}, 404, true, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, ts := mountDetailWithBatch(&fakeApprovalReader{getResult: mdRequest(5)}, &fakeMasterDataDetail{head: head, batch: rows}, tc.batches)

			rec := doRequest(router, http.MethodGet, "/api/v1/approvals/7/master-data", tokenFor(t, ts, 5), "")

			if rec.Code != tc.wantCode {
				t.Fatalf("expected %d, got %d: %s", tc.wantCode, rec.Code, rec.Body.String())
			}
			if tc.wantCode != http.StatusOK {
				return
			}
			var body detailBody
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if tc.wantNil {
				if body.Batch != nil {
					t.Errorf("unexpected batch %v", body.Batch)
				}
				return
			}
			if body.Batch["id"] != float64(3) || body.Batch["entity"] != "vendors" ||
				body.Batch["maker_name"] != "Budi" || body.Batch["row_count"] != float64(3) {
				t.Errorf("batch = %v", body.Batch)
			}
			if v, present := body.Batch["file_name"]; !present || v != tc.wantFile {
				t.Errorf("file_name = %v (present=%v), want %v", v, present, tc.wantFile)
			}
		})
	}
}

// Property 8 (approval part): a user who is neither maker nor holder of a
// pending step gets 403 before paging is parsed and before any data is read.
func TestApprovalMasterDataDetail_Forbidden_BeforePagingAndWithoutReads(t *testing.T) {
	head, rows := batchRows(10)
	for _, q := range []string{"", "?page=1&page_size=50", "?page=2&page_size=5", "?page_size=999", "?page=abc"} {
		t.Run("q="+q, func(t *testing.T) {
			detail := &countingDetail{fakeMasterDataDetail: fakeMasterDataDetail{head: head, batch: rows}}
			batches := &fakeBatchReader{row: db.GetMasterDataImportBatchRow{ID: 3}}
			router, ts := mountDetailWithBatch(&fakeApprovalReader{getResult: mdRequest(5)}, detail, batches)

			rec := doRequest(router, http.MethodGet, "/api/v1/approvals/7/master-data"+q, tokenFor(t, ts, 11), "")

			if rec.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
			}
			if detail.reads != 0 || batches.reads != 0 {
				t.Errorf("data read despite 403: detail=%d batch=%d", detail.reads, batches.reads)
			}
		})
	}
}
