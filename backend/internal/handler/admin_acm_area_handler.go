package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/service"
	"github.com/cimb-niaga/cms/pkg/middleware"
)

// AcmAreaAdminServicer is the subset of service.AcmAreaAdminService the handler needs.
type AcmAreaAdminServicer interface {
	List(ctx context.Context, status string) ([]db.ListAcmAreasAdminRow, error)
	Get(ctx context.Context, id int64) (*service.AcmAreaDetail, error)
	EligibleUsers(ctx context.Context) ([]db.ListAcmEligibleUsersRow, error)
	BranchOptions(ctx context.Context) ([]db.ListAcmBranchOptionsRow, error)
	RegionOptions(ctx context.Context) ([]db.ListAcmRegionOptionsRow, error)
	Warnings(ctx context.Context) ([]db.ListVaultPlanRegionWarningsRow, error)
	Create(ctx context.Context, actorID int64, actorRole, name, ip string) (db.AcmArea, error)
	Rename(ctx context.Context, actorID int64, actorRole string, id int64, name, ip string) (db.AcmArea, error)
	SetActive(ctx context.Context, actorID int64, actorRole string, id int64, active bool, ip string) (db.AcmArea, error)
	SetBranches(ctx context.Context, actorID int64, actorRole string, id int64, branchIDs []int64, ip string) (*service.AcmAreaDetail, error)
	SetRegions(ctx context.Context, actorID int64, actorRole string, id int64, regionCodes []string, ip string) (*service.AcmAreaDetail, error)
	SetMembers(ctx context.Context, actorID int64, actorRole string, id int64, userIDs []int64, ip string) (*service.AcmAreaDetail, error)
}

// AdminAcmAreaHandler serves Pengaturan -> Area ACM (cit-acm-plan FR7). Mount at
// /api/v1/admin/acm-areas behind RequireAuth + RequireRoles("ADMIN"); the
// service re-checks the role. Mutations apply immediately (200/201, not 202).
type AdminAcmAreaHandler struct {
	svc AcmAreaAdminServicer
}

// NewAdminAcmAreaHandler creates an AdminAcmAreaHandler.
func NewAdminAcmAreaHandler(svc AcmAreaAdminServicer) *AdminAcmAreaHandler {
	return &AdminAcmAreaHandler{svc: svc}
}

// Routes returns the Area ACM router.
func (h *AdminAcmAreaHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Get("/warnings", h.Warnings)
	r.Get("/eligible-users", h.EligibleUsers)
	r.Get("/branch-options", h.BranchOptions)
	r.Get("/region-options", h.RegionOptions)
	r.Get("/{id}", h.Get)
	r.Put("/{id}", h.Rename)
	r.Post("/{id}/disable", h.setActive(false))
	r.Post("/{id}/enable", h.setActive(true))
	r.Get("/{id}/regions", h.Regions)
	r.Put("/{id}/regions", h.SetRegions)
	r.Put("/{id}/branches", h.SetBranches)
	r.Put("/{id}/members", h.SetMembers)
	return r
}

func acmAreaJSON(a db.AcmArea) map[string]any {
	return map[string]any{"id": a.ID, "name": a.Name, "is_active": a.IsActive, "deleted_at": formatTimestamptz(a.DeletedAt)}
}

func acmAreaDetailJSON(d *service.AcmAreaDetail) map[string]any {
	out := acmAreaJSON(d.Area)
	out["region_codes"] = d.Regions
	out["branches"] = d.Branches
	out["members"] = d.Members
	return out
}

// List handles GET /?status=active|disabled|all.
func (h *AdminAcmAreaHandler) List(w http.ResponseWriter, r *http.Request) {
	status, err := parseStatusParam(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	rows, err := h.svc.List(r.Context(), status)
	if err != nil {
		writeUnexpectedError(w, err)
		return
	}
	items := make([]map[string]any, len(rows))
	for i, a := range rows {
		items[i] = map[string]any{"id": a.ID, "name": a.Name, "is_active": a.IsActive, "deleted_at": formatTimestamptz(a.DeletedAt),
			"branch_count": a.BranchCount, "member_count": a.MemberCount, "region_count": a.RegionCount}
	}
	writeJSON(w, http.StatusOK, map[string]any{"areas": items})
}

// Get handles GET /{id} -- area with branches and members.
func (h *AdminAcmAreaHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	d, err := h.svc.Get(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, acmAreaDetailJSON(d))
}

// EligibleUsers handles GET /eligible-users -- member picker source.
func (h *AdminAcmAreaHandler) EligibleUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.EligibleUsers(r.Context())
	if err != nil {
		writeUnexpectedError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": rows})
}

// BranchOptions handles GET /branch-options -- branch picker source.
func (h *AdminAcmAreaHandler) BranchOptions(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.BranchOptions(r.Context())
	if err != nil {
		writeUnexpectedError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"branches": rows})
}

// RegionOptions handles GET /region-options -- region picker source (cit-vault-select FR1.1).
func (h *AdminAcmAreaHandler) RegionOptions(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.RegionOptions(r.Context())
	if err != nil {
		writeUnexpectedError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"regions": rows})
}

// Warnings handles GET /warnings -- requests without a plan: region without
// an area, or no region code (cit-vault-select FR1.4).
func (h *AdminAcmAreaHandler) Warnings(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.Warnings(r.Context())
	if err != nil {
		writeUnexpectedError(w, err)
		return
	}
	items := make([]map[string]any, len(rows))
	for i, x := range rows {
		items[i] = map[string]any{"vendor_request_id": x.VendorRequestID, "request_number": x.RequestNumber,
			"region_code": x.RegionCode, "replenish_date": formatPgDate(x.ReplenishDate)}
	}
	writeJSON(w, http.StatusOK, map[string]any{"requests_without_area": items})
}

type acmAreaNameBody struct {
	Name string `json:"name"`
}

// Create handles POST / {name} -> 201.
func (h *AdminAcmAreaHandler) Create(w http.ResponseWriter, r *http.Request) {
	auth, ok := h.auth(w, r)
	if !ok {
		return
	}
	var body acmAreaNameBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}
	a, err := h.svc.Create(r.Context(), auth.UserID, auth.Role, body.Name, extractClientIP(r))
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, acmAreaJSON(a))
}

// Rename handles PUT /{id} {name}.
func (h *AdminAcmAreaHandler) Rename(w http.ResponseWriter, r *http.Request) {
	auth, id, ok := h.authAndID(w, r)
	if !ok {
		return
	}
	var body acmAreaNameBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}
	a, err := h.svc.Rename(r.Context(), auth.UserID, auth.Role, id, body.Name, extractClientIP(r))
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, acmAreaJSON(a))
}

func (h *AdminAcmAreaHandler) setActive(active bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth, id, ok := h.authAndID(w, r)
		if !ok {
			return
		}
		a, err := h.svc.SetActive(r.Context(), auth.UserID, auth.Role, id, active, extractClientIP(r))
		if err != nil {
			h.handleError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, acmAreaJSON(a))
	}
}

// SetBranches handles PUT /{id}/branches {vendor_branch_ids: []} (replace-all).
func (h *AdminAcmAreaHandler) SetBranches(w http.ResponseWriter, r *http.Request) {
	h.setLinks(w, r, "vendor_branch_ids", h.svc.SetBranches)
}

// Regions handles GET /{id}/regions.
func (h *AdminAcmAreaHandler) Regions(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	d, err := h.svc.Get(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"region_codes": d.Regions})
}

// SetRegions handles PUT /{id}/regions {region_codes: []} (replace-all).
func (h *AdminAcmAreaHandler) SetRegions(w http.ResponseWriter, r *http.Request) {
	auth, id, ok := h.authAndID(w, r)
	if !ok {
		return
	}
	var body struct {
		RegionCodes *[]string `json:"region_codes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}
	if body.RegionCodes == nil {
		writeValidationError(w, "region_codes", "wajib diisi (boleh array kosong)")
		return
	}
	d, err := h.svc.SetRegions(r.Context(), auth.UserID, auth.Role, id, *body.RegionCodes, extractClientIP(r))
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, acmAreaDetailJSON(d))
}

// SetMembers handles PUT /{id}/members {user_ids: []} (replace-all).
func (h *AdminAcmAreaHandler) SetMembers(w http.ResponseWriter, r *http.Request) {
	h.setLinks(w, r, "user_ids", h.svc.SetMembers)
}

func (h *AdminAcmAreaHandler) setLinks(w http.ResponseWriter, r *http.Request, field string,
	fn func(context.Context, int64, string, int64, []int64, string) (*service.AcmAreaDetail, error)) {
	auth, id, ok := h.authAndID(w, r)
	if !ok {
		return
	}
	var body map[string][]int64
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}
	ids, present := body[field]
	if !present {
		writeValidationError(w, field, "wajib diisi (boleh array kosong)")
		return
	}
	d, err := fn(r.Context(), auth.UserID, auth.Role, id, ids, extractClientIP(r))
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, acmAreaDetailJSON(d))
}

func (h *AdminAcmAreaHandler) auth(w http.ResponseWriter, r *http.Request) (*middleware.AuthContext, bool) {
	a, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
	}
	return a, ok
}

func (h *AdminAcmAreaHandler) authAndID(w http.ResponseWriter, r *http.Request) (*middleware.AuthContext, int64, bool) {
	a, ok := h.auth(w, r)
	if !ok {
		return nil, 0, false
	}
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return nil, 0, false
	}
	return a, id, true
}

func (h *AdminAcmAreaHandler) handleError(w http.ResponseWriter, err error) {
	var validationErr *service.ValidationError
	var conflictErr *service.AcmAreaBranchConflictError
	var regionConflictErr *service.AcmAreaRegionConflictError
	switch {
	case errors.As(err, &validationErr):
		writeValidationError(w, validationErr.Field, validationErr.Message)
	case errors.As(err, &conflictErr):
		writeJSON(w, http.StatusConflict, map[string]any{"error": "conflict", "message": err.Error(), "conflicts": conflictErr.Conflicts})
	case errors.As(err, &regionConflictErr):
		writeJSON(w, http.StatusConflict, map[string]any{"error": "conflict", "message": err.Error(), "conflicts": regionConflictErr.Conflicts})
	case errors.Is(err, service.ErrNotAuthorized):
		writeForbidden(w, "Anda tidak berhak mengubah area ACM")
	case errors.Is(err, service.ErrAcmAreaNotFound):
		writeError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, service.ErrAcmAreaNameConflict), errors.Is(err, service.ErrAcmAreaInactive), errors.Is(err, service.ErrAcmAreaStatusUnchanged):
		writeError(w, http.StatusConflict, "conflict", err.Error())
	default:
		writeUnexpectedError(w, err)
	}
}
