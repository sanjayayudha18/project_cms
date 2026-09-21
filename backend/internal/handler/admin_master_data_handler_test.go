package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
	custommw "github.com/cimb-niaga/cms/pkg/middleware"
)

// fakeMasterDataChangeReader implements MasterDataChangeReader for
// httptest, no real DB needed -- the underlying logic is covered by
// backend/internal/service/masterdata_change_test.go; this only exercises
// HTTP wiring.
type fakeMasterDataChangeReader struct {
	listResult  []db.MasterDataChangeRequest
	listErr     error
	countResult int64
	countErr    error
	getResult   db.MasterDataChangeRequest
	getErr      error
}

func (f *fakeMasterDataChangeReader) Get(context.Context, int64) (db.MasterDataChangeRequest, error) {
	return f.getResult, f.getErr
}

func (f *fakeMasterDataChangeReader) List(context.Context, db.ListMasterDataChangeRequestsParams) ([]db.MasterDataChangeRequest, error) {
	return f.listResult, f.listErr
}

func (f *fakeMasterDataChangeReader) Count(context.Context, db.CountMasterDataChangeRequestsParams) (int64, error) {
	return f.countResult, f.countErr
}

func mountAdminMasterDataChangeHandler(svc MasterDataChangeReader) (http.Handler, *pkgauth.TokenService) {
	tokenSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	}, noopBlacklist{})

	h := NewAdminMasterDataChangeHandler(svc)
	r := chi.NewRouter()
	r.With(
		custommw.RequireAuth(tokenSvc),
		custommw.RequireRoles("ADMIN", "ADMIN_PARAM"),
	).Mount("/api/v1/admin/master-data/changes", h.Routes())
	return r, tokenSvc
}

func TestAdminMasterDataChangeHandler_List_HappyPath(t *testing.T) {
	svc := &fakeMasterDataChangeReader{
		listResult:  []db.MasterDataChangeRequest{{ID: 1, EntityType: "vendor", Op: "create", Status: "pending", MakerID: 7, Payload: []byte(`{"code":"V1"}`)}},
		countResult: 1,
	}
	router, tokenSvc := mountAdminMasterDataChangeHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/master-data/changes?page=1&page_size=10", token, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"total":1`) || !strings.Contains(body, `"page_size":10`) {
		t.Errorf("expected pagination metadata in body, got: %s", body)
	}
	if !strings.Contains(body, `"entity_type":"vendor"`) || !strings.Contains(body, `"payload":{"code":"V1"}`) {
		t.Errorf("expected flat-JSON change row with raw payload in body, got: %s", body)
	}
}

func TestAdminMasterDataChangeHandler_List_EntityTypeAndStatusFilter_PassedThrough(t *testing.T) {
	var captured db.ListMasterDataChangeRequestsParams
	svc := &fakeMasterDataChangeReader{}
	router, tokenSvc := mountAdminMasterDataChangeHandler(&capturingReader{fakeMasterDataChangeReader: svc, onList: func(arg db.ListMasterDataChangeRequestsParams) { captured = arg }})
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/master-data/changes?entity_type=vendor&status=pending", token, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if captured.EntityType == nil || *captured.EntityType != "vendor" {
		t.Errorf("expected entity_type filter passed through, got %+v", captured.EntityType)
	}
	if captured.Status == nil || *captured.Status != "pending" {
		t.Errorf("expected status filter passed through, got %+v", captured.Status)
	}
}

func TestAdminMasterDataChangeHandler_List_InvalidStatus_BadRequest(t *testing.T) {
	router, tokenSvc := mountAdminMasterDataChangeHandler(&fakeMasterDataChangeReader{})
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/master-data/changes?status=bogus", token, "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminMasterDataChangeHandler_List_NoAuth_Unauthorized(t *testing.T) {
	router, _ := mountAdminMasterDataChangeHandler(&fakeMasterDataChangeReader{})

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/master-data/changes", "", "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminMasterDataChangeHandler_List_WrongRole_Forbidden(t *testing.T) {
	router, tokenSvc := mountAdminMasterDataChangeHandler(&fakeMasterDataChangeReader{})
	token := tokenForRole(t, tokenSvc, 1, "ATM-USER")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/master-data/changes", token, "")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminMasterDataChangeHandler_Get_HappyPath_IncludesPayloadAndBefore(t *testing.T) {
	svc := &fakeMasterDataChangeReader{
		getResult: db.MasterDataChangeRequest{
			ID: 5, EntityType: "vendor", Op: "update", Status: "pending", MakerID: 7,
			Payload: []byte(`{"name":"New Name"}`), Before: []byte(`{"name":"Old Name"}`),
		},
	}
	router, tokenSvc := mountAdminMasterDataChangeHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN_PARAM")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/master-data/changes/5", token, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"payload":{"name":"New Name"}`) {
		t.Errorf("expected raw payload in body, got: %s", body)
	}
	if !strings.Contains(body, `"before":{"name":"Old Name"}`) {
		t.Errorf("expected raw before in body, got: %s", body)
	}
}

func TestAdminMasterDataChangeHandler_Get_NotFound(t *testing.T) {
	svc := &fakeMasterDataChangeReader{getErr: pgx.ErrNoRows}
	router, tokenSvc := mountAdminMasterDataChangeHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/master-data/changes/999", token, "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminMasterDataChangeHandler_Get_InvalidID_BadRequest(t *testing.T) {
	router, tokenSvc := mountAdminMasterDataChangeHandler(&fakeMasterDataChangeReader{})
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/master-data/changes/not-a-number", token, "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminMasterDataChangeHandler_Get_InternalError(t *testing.T) {
	svc := &fakeMasterDataChangeReader{getErr: errors.New("db exploded")}
	router, tokenSvc := mountAdminMasterDataChangeHandler(svc)
	token := tokenForRole(t, tokenSvc, 1, "ADMIN")

	rec := doRequest(router, http.MethodGet, "/api/v1/admin/master-data/changes/1", token, "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

// capturingReader wraps a fakeMasterDataChangeReader to record the exact
// filter params List() was called with.
type capturingReader struct {
	*fakeMasterDataChangeReader
	onList func(db.ListMasterDataChangeRequestsParams)
}

func (c *capturingReader) List(ctx context.Context, arg db.ListMasterDataChangeRequestsParams) ([]db.MasterDataChangeRequest, error) {
	c.onList(arg)
	return c.fakeMasterDataChangeReader.List(ctx, arg)
}
