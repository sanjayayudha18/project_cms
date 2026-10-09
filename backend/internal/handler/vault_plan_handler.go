package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/service"
)

// VaultPlanServicer is the subset of service.VaultPlanService the handler needs.
type VaultPlanServicer interface {
	List(ctx context.Context, actor service.Actor, f service.VaultPlanListFilter) ([]db.ListVaultPlansRow, error)
	Get(ctx context.Context, actor service.Actor, id int64) (*service.VaultPlanDetail, error)
	Candidates(ctx context.Context, actor service.Actor, id int64, terminalID string, urgent bool) ([]service.VaultCandidate, error)
	SaveAssignments(ctx context.Context, actor service.Actor, id int64, in []service.VaultAssignmentInput) (*service.VaultPlanDetail, error)
	Submit(ctx context.Context, actor service.Actor, id int64) (*service.VaultPlanDetail, error)
	Approve(ctx context.Context, actor service.Actor, id int64) (*service.VaultPlanDetail, error)
	Reject(ctx context.Context, actor service.Actor, id int64, reason string) (*service.VaultPlanDetail, error)
	Audit(ctx context.Context, actor service.Actor, id int64) ([]service.AuditEntry, error)
}

// VaultPlanHandler serves CIT -> Penetapan Vault (cit-acm-plan FR4/FR6). Mount at
// /api/v1/vault-plans behind RequireAuth + RequireRoles("ACM-USER","ACM-SPV","ADMIN");
// area membership and the per-action role are checked in the service.
type VaultPlanHandler struct {
	svc VaultPlanServicer
}

// NewVaultPlanHandler creates a VaultPlanHandler.
func NewVaultPlanHandler(svc VaultPlanServicer) *VaultPlanHandler {
	return &VaultPlanHandler{svc: svc}
}

// Routes returns the vault plan router.
func (h *VaultPlanHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Get("/{id}", h.Get)
	r.Get("/{id}/candidates", h.Candidates)
	r.Get("/{id}/audit", h.Audit)
	r.Put("/{id}/assignments", h.SaveAssignments)
	r.Post("/{id}/submit", h.transition(func(ctx context.Context, a service.Actor, id int64, _ *http.Request) (*service.VaultPlanDetail, error) {
		return h.svc.Submit(ctx, a, id)
	}))
	r.Post("/{id}/approve", h.transition(func(ctx context.Context, a service.Actor, id int64, _ *http.Request) (*service.VaultPlanDetail, error) {
		return h.svc.Approve(ctx, a, id)
	}))
	r.Post("/{id}/reject", h.transition(func(ctx context.Context, a service.Actor, id int64, r *http.Request) (*service.VaultPlanDetail, error) {
		var body struct {
			Reason string `json:"rejection_reason"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body) // empty/invalid body -> empty reason -> 422 from the service
		return h.svc.Reject(ctx, a, id, body.Reason)
	}))
	return r
}

func parseDateParam(v string) (pgtype.Date, error) {
	if v == "" {
		return pgtype.Date{}, nil
	}
	t, err := time.Parse("2006-01-02", v)
	if err != nil {
		return pgtype.Date{}, errors.New("tanggal harus berformat YYYY-MM-DD")
	}
	return pgtype.Date{Time: t, Valid: true}, nil
}

func formatPgDate(d pgtype.Date) *string {
	if !d.Valid {
		return nil
	}
	s := d.Time.Format("2006-01-02")
	return &s
}

// List handles GET /?status=&from=&to=&area_id=.
func (h *VaultPlanHandler) List(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	q := r.URL.Query()
	from, err := parseDateParam(q.Get("from"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	to, err := parseDateParam(q.Get("to"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	areaID, err := parseOptionalIDParam(q, "area_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	rows, err := h.svc.List(r.Context(), actor, service.VaultPlanListFilter{Status: q.Get("status"), AreaID: areaID, DateFrom: from, DateTo: to})
	if err != nil {
		h.handleError(w, err)
		return
	}
	items := make([]map[string]any, len(rows))
	for i, p := range rows {
		items[i] = map[string]any{
			"id": p.ID, "vendor_request_id": p.VendorRequestID, "request_number": p.RequestNumber,
			"replenish_date": formatPgDate(p.ReplenishDate), "request_status": p.RequestStatus,
			"acm_area_id": p.AcmAreaID, "acm_area_name": p.AcmAreaName, "status": p.Status,
			"assigned_count": p.AssignedCount, "warning_count": p.WarningCount, "submitted_at": formatTimestamptz(p.SubmittedAt),
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"plans": items})
}

func vaultPlanJSON(d *service.VaultPlanDetail) map[string]any {
	p := d.Plan
	return map[string]any{
		"id": p.ID, "vendor_request_id": p.VendorRequestID, "request_number": p.RequestNumber,
		"replenish_date": formatPgDate(p.ReplenishDate), "request_status": p.RequestStatus,
		"vault_rejection_reason": p.VaultRejectionReason, "acm_area_id": p.AcmAreaID, "acm_area_name": p.AcmAreaName,
		"status": p.Status, "submitted_by": p.SubmittedBy, "submitted_at": formatTimestamptz(p.SubmittedAt),
		"approved_by": p.ApprovedBy, "approved_at": formatTimestamptz(p.ApprovedAt),
		"rejected_by": p.RejectedBy, "rejected_at": formatTimestamptz(p.RejectedAt), "rejection_reason": p.RejectionReason,
		"rejected_by_vendor": p.RejectedByVendor, "atms": d.Atms,
	}
}

func (h *VaultPlanHandler) actorAndID(w http.ResponseWriter, r *http.Request) (service.Actor, int64, bool) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return actor, 0, false
	}
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return actor, 0, false
	}
	return actor, id, true
}

// Get handles GET /{id}.
func (h *VaultPlanHandler) Get(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := h.actorAndID(w, r)
	if !ok {
		return
	}
	d, err := h.svc.Get(r.Context(), actor, id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, vaultPlanJSON(d))
}

// Candidates handles GET /{id}/candidates?terminal_id=&urgent=true.
func (h *VaultPlanHandler) Candidates(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := h.actorAndID(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	if q.Get("terminal_id") == "" {
		writeValidationError(w, "terminal_id", "wajib diisi")
		return
	}
	c, err := h.svc.Candidates(r.Context(), actor, id, q.Get("terminal_id"), q.Get("urgent") == "true")
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"candidates": c})
}

// SaveAssignments handles PUT /{id}/assignments {assignments: [...]} (replace-all, draft only).
func (h *VaultPlanHandler) SaveAssignments(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := h.actorAndID(w, r)
	if !ok {
		return
	}
	var body struct {
		Assignments []service.VaultAssignmentInput `json:"assignments"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}
	d, err := h.svc.SaveAssignments(r.Context(), actor, id, body.Assignments)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, vaultPlanJSON(d))
}

func (h *VaultPlanHandler) transition(fn func(context.Context, service.Actor, int64, *http.Request) (*service.VaultPlanDetail, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, id, ok := h.actorAndID(w, r)
		if !ok {
			return
		}
		d, err := fn(r.Context(), actor, id, r)
		if err != nil {
			h.handleError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, vaultPlanJSON(d))
	}
}

// Audit handles GET /{id}/audit.
func (h *VaultPlanHandler) Audit(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := h.actorAndID(w, r)
	if !ok {
		return
	}
	entries, err := h.svc.Audit(r.Context(), actor, id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toAuditLogResponse(entries))
}

func (h *VaultPlanHandler) handleError(w http.ResponseWriter, err error) {
	var validationErr *service.ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeValidationError(w, validationErr.Field, validationErr.Message)
	case errors.Is(err, service.ErrVaultPlanNotFound):
		writeError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, service.ErrInvalidTransition):
		writeError(w, http.StatusConflict, "conflict", "Status saat ini tidak mengizinkan aksi ini")
	case errors.Is(err, service.ErrSelfApproval):
		writeForbidden(w, "Penyetuju tidak boleh sama dengan pengaju (four-eyes)")
	case errors.Is(err, service.ErrNotAuthorized):
		writeForbidden(w, "Anda tidak berhak melakukan aksi ini")
	case errors.Is(err, service.ErrRejectReasonEmpty):
		writeValidationError(w, "rejection_reason", "wajib diisi")
	default:
		writeUnexpectedError(w, err)
	}
}
