package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// MasterDataChangeReader is the subset of service.MasterDataChangeService
// the handler needs -- read-only (T2.7): list (filter by entity_type/
// status) and get-by-id.
type MasterDataChangeReader interface {
	Get(ctx context.Context, id int64) (db.MasterDataChangeRequest, error)
	List(ctx context.Context, arg db.ListMasterDataChangeRequestsParams) ([]db.MasterDataChangeRequest, error)
	Count(ctx context.Context, arg db.CountMasterDataChangeRequestsParams) (int64, error)
}

// validMasterDataChangeStatuses mirrors the
// master_data_change_requests_status_chk CHECK constraint (T1.5).
var validMasterDataChangeStatuses = map[string]bool{
	"pending": true, "approved": true, "rejected": true, "applied": true, "stale": true,
}

// AdminMasterDataChangeHandler handles ADMIN/ADMIN_PARAM-only read access to
// the master-data maker-checker change log (T2.7): list (filter by
// entity_type/status, paginated) and get-by-id with full payload/before for
// diff display. Mount behind RequireAuth + RequireRoles("ADMIN",
// "ADMIN_PARAM") -- see cmd/api/main.go.
type AdminMasterDataChangeHandler struct {
	svc MasterDataChangeReader
}

// NewAdminMasterDataChangeHandler creates a new AdminMasterDataChangeHandler with the given dependency.
func NewAdminMasterDataChangeHandler(svc MasterDataChangeReader) *AdminMasterDataChangeHandler {
	return &AdminMasterDataChangeHandler{svc: svc}
}

// Routes returns a chi.Router with the read endpoints mounted.
func (h *AdminMasterDataChangeHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Get("/{id}", h.Get)
	return r
}

// List handles GET / -- filterable, paginated change request list.
func (h *AdminMasterDataChangeHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	page, pageSize, err := parsePageParams(q)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	var entityType, status *string
	if v := q.Get("entity_type"); v != "" {
		entityType = &v
	}
	if v := q.Get("status"); v != "" {
		if !validMasterDataChangeStatuses[v] {
			writeError(w, http.StatusBadRequest, "bad_request", "status tidak valid")
			return
		}
		status = &v
	}

	filters := db.ListMasterDataChangeRequestsParams{
		EntityType: entityType,
		Status:     status,
		PageLimit:  int64(pageSize),
		PageOffset: int64((page - 1) * pageSize),
	}
	changes, err := h.svc.List(r.Context(), filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	total, err := h.svc.Count(r.Context(), db.CountMasterDataChangeRequestsParams{EntityType: entityType, Status: status})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}

	items := make([]map[string]any, len(changes))
	for i, c := range changes {
		items[i] = masterDataChangeToResponse(c)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"changes": items, "page": page, "page_size": pageSize, "total": total,
	})
}

// Get handles GET /{id} -- full payload/before for diff display.
func (h *AdminMasterDataChangeHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	change, err := h.svc.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "change request not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	writeJSON(w, http.StatusOK, masterDataChangeToResponse(change))
}

// masterDataChangeToResponse builds the flat-JSON change-request DTO.
// payload/before are passed through as raw JSON (not re-escaped as a
// string) so the frontend can render/diff them directly.
func masterDataChangeToResponse(c db.MasterDataChangeRequest) map[string]any {
	var before any
	if len(c.Before) > 0 {
		before = json.RawMessage(c.Before)
	}
	return map[string]any{
		"id": c.ID, "entity_type": c.EntityType, "entity_id": c.EntityID, "op": c.Op,
		"payload": json.RawMessage(c.Payload), "before": before,
		"status": c.Status, "maker_id": c.MakerID, "approval_request_id": c.ApprovalRequestID,
		"batch_id": c.BatchID, "error": c.Error,
		"created_at": formatTimestamptz(c.CreatedAt), "updated_at": formatTimestamptz(c.UpdatedAt),
	}
}
