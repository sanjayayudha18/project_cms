package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/approval"
	"github.com/cimb-niaga/cms/backend/internal/db"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

type fakeMasterDataDetail struct {
	head  db.MasterDataChangeRequest
	batch []db.MasterDataChangeRequest
	err   error
}

func (f *fakeMasterDataDetail) GetByID(context.Context, int64) (db.MasterDataChangeRequest, error) {
	return f.head, f.err
}

func (f *fakeMasterDataDetail) ListByBatch(context.Context, int64) ([]db.MasterDataChangeRequest, error) {
	return f.batch, f.err
}

func mountMasterDataDetail(reader ApprovalReader, detail MasterDataApprovalDetailReader) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	}, noopBlacklist{})
	h := NewApprovalHandler(&fakeApprovalOrchestrator{}, reader).WithMasterDataDetail(detail)
	r := chi.NewRouter()
	r.With(custommw.RequireAuth(tokenSvc)).Mount("/api/v1/approvals", h.Routes())
	return r, tokenSvc
}

func mdRequest(makerID int64) db.ApprovalRequest {
	return db.ApprovalRequest{ID: 7, MakerID: makerID, DocumentType: "master_data", DocumentID: 40, Status: "pending"}
}

func TestApprovalMasterDataDetail_MakerAndPendingApproverMayRead_OthersMayNot(t *testing.T) {
	head := db.MasterDataChangeRequest{ID: 40, EntityType: "vendor", Op: "update", Payload: []byte(`{"name":"Baru"}`), Before: []byte(`{"name":"Lama"}`), Status: "pending"}
	inbox := []approval.InboxItem{{RequestID: 7, DocumentType: "master_data", DocumentID: 40}}
	cases := []struct {
		name  string
		user  int64
		inbox []approval.InboxItem
		want  int
	}{
		{"maker", 5, nil, http.StatusOK},
		{"approver with a pending step", 9, inbox, http.StatusOK},
		{"unrelated user", 11, nil, http.StatusForbidden},
		{"approver of a different request", 12, []approval.InboxItem{{RequestID: 99}}, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, ts := mountMasterDataDetail(&fakeApprovalReader{getResult: mdRequest(5), inboxResult: tc.inbox}, &fakeMasterDataDetail{head: head})

			rec := doRequest(router, http.MethodGet, "/api/v1/approvals/7/master-data", tokenFor(t, ts, tc.user), "")

			if rec.Code != tc.want {
				t.Fatalf("expected %d, got %d: %s", tc.want, rec.Code, rec.Body.String())
			}
			if tc.want == http.StatusForbidden {
				var body map[string]any
				_ = json.Unmarshal(rec.Body.Bytes(), &body)
				if _, leaked := body["changes"]; leaked {
					t.Errorf("a forbidden response must not carry the change payload: %s", rec.Body.String())
				}
			}
		})
	}
}

func TestApprovalMasterDataDetail_SingleChangeCarriesPayloadAndBefore(t *testing.T) {
	head := db.MasterDataChangeRequest{ID: 40, EntityType: "vendor", Op: "update", Payload: []byte(`{"name":"Baru"}`), Before: []byte(`{"name":"Lama"}`), Status: "pending"}
	router, ts := mountMasterDataDetail(&fakeApprovalReader{getResult: mdRequest(5)}, &fakeMasterDataDetail{head: head})

	rec := doRequest(router, http.MethodGet, "/api/v1/approvals/7/master-data", tokenFor(t, ts, 5), "")

	var body struct {
		Total   int            `json:"total"`
		Counts  map[string]int `json:"counts"`
		Changes []struct {
			Op      string         `json:"op"`
			Payload map[string]any `json:"payload"`
			Before  map[string]any `json:"before"`
		} `json:"changes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v: %s", err, rec.Body.String())
	}
	if body.Total != 1 || body.Counts["update"] != 1 || len(body.Changes) != 1 ||
		body.Changes[0].Payload["name"] != "Baru" || body.Changes[0].Before["name"] != "Lama" {
		t.Errorf("body = %s", rec.Body.String())
	}
}

func TestApprovalMasterDataDetail_BatchReturnsCountsAndCapsRows(t *testing.T) {
	batchID := int64(3)
	head := db.MasterDataChangeRequest{ID: 40, EntityType: "vendor", Op: "create", BatchID: &batchID, Payload: []byte(`{}`)}
	var rows []db.MasterDataChangeRequest
	disables := 0
	for i := 0; i < approvalMasterDataDetailLimit+30; i++ {
		op := "create"
		if i%10 == 0 {
			op = "disable"
			disables++
		}
		rows = append(rows, db.MasterDataChangeRequest{ID: int64(40 + i), EntityType: "vendor", Op: op, BatchID: &batchID, Payload: []byte(`{}`)})
	}
	router, ts := mountMasterDataDetail(&fakeApprovalReader{getResult: mdRequest(5)}, &fakeMasterDataDetail{head: head, batch: rows})

	rec := doRequest(router, http.MethodGet, "/api/v1/approvals/7/master-data", tokenFor(t, ts, 5), "")

	var body struct {
		Total     int              `json:"total"`
		Truncated bool             `json:"truncated"`
		Counts    map[string]int   `json:"counts"`
		Changes   []map[string]any `json:"changes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Total != approvalMasterDataDetailLimit+30 || !body.Truncated || len(body.Changes) != approvalMasterDataDetailLimit {
		t.Errorf("total=%d truncated=%v shown=%d", body.Total, body.Truncated, len(body.Changes))
	}
	if body.Counts["create"]+body.Counts["disable"] != body.Total || body.Counts["disable"] != disables {
		t.Errorf("counts must cover the WHOLE batch, got %v", body.Counts)
	}
}

func TestApprovalMasterDataDetail_NotMasterDataOrUnwired_404_Anonymous_401(t *testing.T) {
	other := db.ApprovalRequest{ID: 7, MakerID: 5, DocumentType: "invoice", DocumentID: 1}
	router, ts := mountMasterDataDetail(&fakeApprovalReader{getResult: other}, &fakeMasterDataDetail{})
	if rec := doRequest(router, http.MethodGet, "/api/v1/approvals/7/master-data", tokenFor(t, ts, 5), ""); rec.Code != http.StatusNotFound {
		t.Errorf("non master_data request: expected 404, got %d", rec.Code)
	}
	if rec := doRequest(router, http.MethodGet, "/api/v1/approvals/7/master-data", "", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("anonymous: expected 401, got %d", rec.Code)
	}

	// Endpoint not enabled (no detail reader) answers 404, never a nil dereference.
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{SecretKey: []byte("test-secret-minimum-32-bytes-long!!"), AccessTokenExpiry: time.Minute, RefreshTokenExpiry: time.Hour}, noopBlacklist{})
	r := chi.NewRouter()
	r.With(custommw.RequireAuth(tokenSvc)).Mount("/api/v1/approvals", NewApprovalHandler(&fakeApprovalOrchestrator{}, &fakeApprovalReader{getResult: mdRequest(5)}).Routes())
	if rec := doRequest(r, http.MethodGet, "/api/v1/approvals/7/master-data", tokenFor(t, tokenSvc, 5), ""); rec.Code != http.StatusNotFound {
		t.Errorf("detail reader not wired: expected 404, got %d", rec.Code)
	}
}
