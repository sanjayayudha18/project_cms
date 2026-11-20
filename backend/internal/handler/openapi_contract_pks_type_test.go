package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/service"
)

// Contract fixtures for /admin/pks-types (spec vendor-pks-type task 3.3/4.4).

const contractPTBase = "/api/v1/admin/pks-types"

// contractPksTypeSvc is a configurable PksTypeAdminServicer; a set err fails every call.
type contractPksTypeSvc struct {
	err error
}

func (s *contractPksTypeSvc) List(context.Context) ([]db.ListPksTypesRow, error) {
	if s.err != nil {
		return nil, s.err
	}
	ts := pgtype.Timestamptz{Time: contractTime(), Valid: true}
	return []db.ListPksTypesRow{
		{ID: 1, Code: "FLM", Name: "FLM", IsActive: true, InUse: true, CreatedAt: ts, UpdatedAt: ts},
		{ID: 2, Code: "OLD", Name: "Lama", IsActive: false, CreatedAt: ts},
	}, nil
}

func (s *contractPksTypeSvc) staged(op string) (db.MasterDataChangeRequest, error) {
	if s.err != nil {
		return db.MasterDataChangeRequest{}, s.err
	}
	return db.MasterDataChangeRequest{ID: 7, EntityType: "vendor_pks_type", Op: op, Status: "pending"}, nil
}

func (s *contractPksTypeSvc) Create(context.Context, int64, service.PksTypeCreatePayload, string) (db.MasterDataChangeRequest, error) {
	return s.staged("create")
}

func (s *contractPksTypeSvc) Update(context.Context, int64, int64, service.PksTypeUpdatePayload, string) (db.MasterDataChangeRequest, error) {
	return s.staged("update")
}

func (s *contractPksTypeSvc) Disable(context.Context, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return s.staged("disable")
}

func (s *contractPksTypeSvc) Enable(context.Context, int64, int64, string) (db.MasterDataChangeRequest, error) {
	return s.staged("enable")
}

// contractPksTypeCases covers 200/202/400/401/403/404/409/422/500 of the type operations.
func contractPksTypeCases() []contractCase {
	pt := contractPTBase
	fail := func(err error) *contractPksTypeSvc { return &contractPksTypeSvc{err: err} }
	createBody := `{"code":"CIT","name":"Cash in Transit"}`
	updateBody := `{"name":"Nama baru"}`
	return []contractCase{
		{name: "pks-types list", method: http.MethodGet, path: pt, role: "ADMIN", wantStatus: 200},
		{name: "pks-types list no token", method: http.MethodGet, path: pt, wantStatus: 401},
		{name: "pks-types list unexpected", method: http.MethodGet, path: pt, role: "ADMIN", pksTypes: fail(errors.New("boom")), wantStatus: 500},
		{name: "pks-types create", method: http.MethodPost, path: pt, body: createBody, role: "ADMIN", wantStatus: 202},
		{name: "pks-types create bad json", method: http.MethodPost, path: pt, body: "{", role: "ADMIN", wantStatus: 400},
		{name: "pks-types create forbidden", method: http.MethodPost, path: pt, body: createBody, role: "ADMIN", pksTypes: fail(service.ErrMasterDataForbidden), wantStatus: 403},
		{name: "pks-types create code taken", method: http.MethodPost, path: pt, body: createBody, role: "ADMIN", pksTypes: fail(service.ErrPksTypeConflict), wantStatus: 409},
		{name: "pks-types create pending", method: http.MethodPost, path: pt, body: createBody, role: "ADMIN", pksTypes: fail(service.ErrMasterDataChangePending), wantStatus: 409},
		{name: "pks-types create invalid", method: http.MethodPost, path: pt, body: createBody, role: "ADMIN",
			pksTypes: fail(&service.ValidationError{Field: "code", Message: "format kode tidak valid"}), wantStatus: 422},
		{name: "pks-types update", method: http.MethodPut, path: pt + "/1", body: updateBody, role: "ADMIN", wantStatus: 202},
		{name: "pks-types update bad id", method: http.MethodPut, path: pt + "/abc", body: updateBody, role: "ADMIN", wantStatus: 400},
		{name: "pks-types update not found", method: http.MethodPut, path: pt + "/1", body: updateBody, role: "ADMIN", pksTypes: fail(service.ErrPksTypeNotFound), wantStatus: 404},
		{name: "pks-types disable", method: http.MethodPost, path: pt + "/1/disable", role: "ADMIN", wantStatus: 202},
		{name: "pks-types disable in use", method: http.MethodPost, path: pt + "/1/disable", role: "ADMIN", pksTypes: fail(service.ErrPksTypeInUse), wantStatus: 409},
		{name: "pks-types enable", method: http.MethodPost, path: pt + "/1/enable", role: "ADMIN", wantStatus: 202},
	}
}
