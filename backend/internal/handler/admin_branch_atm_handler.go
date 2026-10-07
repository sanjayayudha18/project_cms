package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/service"
)

// BranchATMServicer is the subset of service.BranchATMService the handler
// needs. Read-only -- no create/update/delete, no maker-checker.
type BranchATMServicer interface {
	List(ctx context.Context, vendorID, branchID, pageLimit, pageOffset int64) (service.ListManagedATMsResult, error)
}

// AdminBranchATMHandler handles the read-only "ATM" sub-tab on the vendor
// branch detail page (.kiro/specs/vendor-branch-atms). Mount at
// /api/v1/admin/vendors/{vendorID}/branches/{branchID}/atms behind
// RequireAuth + RequireRoles("ADMIN", "ADMIN_PARAM") -- see cmd/api/main.go.
type AdminBranchATMHandler struct {
	svc BranchATMServicer
}

// NewAdminBranchATMHandler creates a new AdminBranchATMHandler with the given dependency.
func NewAdminBranchATMHandler(svc BranchATMServicer) *AdminBranchATMHandler {
	return &AdminBranchATMHandler{svc: svc}
}

// Routes returns a chi.Router with the (read-only) branch ATM endpoint mounted.
func (h *AdminBranchATMHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	return r
}

func managedATMToResponse(a service.ManagedATM) map[string]any {
	return map[string]any{
		"atm_id":                   a.ATMID,
		"terminal_id":              a.TerminalID,
		"location_name":            a.LocationName,
		"location_city_or_regency": a.LocationCityOrRegency,
		"priority_class":           a.PriorityClass,
		"is_active":                a.IsActive,
		"package_code":             a.PackageCode,
	}
}

// List handles GET / -- paginated list of ATMs managed by the branch.
func (h *AdminBranchATMHandler) List(w http.ResponseWriter, r *http.Request) {
	vendorID, err := parsePathID(r, "vendorID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	branchID, err := parsePathID(r, "branchID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	page, pageSize, err := parsePageParams(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	result, err := h.svc.List(r.Context(), vendorID, branchID, int64(pageSize), int64((page-1)*pageSize))
	if err != nil {
		h.handleError(w, err)
		return
	}

	items := make([]map[string]any, len(result.ATMs))
	for i, a := range result.ATMs {
		items[i] = managedATMToResponse(a)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"atms": items, "page": page, "page_size": pageSize, "total": result.Total,
	})
}

func (h *AdminBranchATMHandler) handleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrBranchNotFound):
		writeError(w, http.StatusNotFound, "not_found", service.ErrBranchNotFound.Error())
	default:
		writeUnexpectedError(w, err)
	}
}
