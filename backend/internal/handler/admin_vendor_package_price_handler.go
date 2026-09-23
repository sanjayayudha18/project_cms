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

// VendorPackagePriceAdminServicer is the subset of
// service.VendorPackagePriceAdminService the handler needs. Mutations return
// a pending db.MasterDataChangeRequest. There is no Enable (a price row is
// effective-dated history, not a togglable entity).
type VendorPackagePriceAdminServicer interface {
	List(ctx context.Context, arg db.ListVendorPackagePricesAdminParams) ([]service.VendorPackagePrice, error)
	Count(ctx context.Context, arg db.CountVendorPackagePricesAdminParams) (int64, error)
	Get(ctx context.Context, vendorID, id int64) (*service.VendorPackagePrice, error)
	Create(ctx context.Context, makerID, vendorID int64, req service.VendorPackagePriceCreatePayload, actorIP string) (db.MasterDataChangeRequest, error)
	Update(ctx context.Context, makerID, vendorID, id int64, req service.VendorPackagePriceContentPayload, actorIP string) (db.MasterDataChangeRequest, error)
	Disable(ctx context.Context, makerID, vendorID, id int64, actorIP string) (db.MasterDataChangeRequest, error)
}

// AdminVendorPackagePriceHandler handles ADMIN/ADMIN_PARAM-only vendor
// package price management (vendor-pricing plan.md P20). Mount at
// /api/v1/admin/vendors/{vendorID}/package-prices behind RequireAuth +
// RequireRoles("ADMIN", "ADMIN_PARAM") -- see cmd/api/main.go.
type AdminVendorPackagePriceHandler struct {
	svc VendorPackagePriceAdminServicer
}

// NewAdminVendorPackagePriceHandler creates a new AdminVendorPackagePriceHandler with the given dependency.
func NewAdminVendorPackagePriceHandler(svc VendorPackagePriceAdminServicer) *AdminVendorPackagePriceHandler {
	return &AdminVendorPackagePriceHandler{svc: svc}
}

// Routes returns a chi.Router with all admin vendor package price endpoints mounted.
func (h *AdminVendorPackagePriceHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Get("/{id}", h.Get)
	r.Put("/{id}", h.Update)
	r.Post("/{id}/disable", h.Disable)
	return r
}

func packagePriceToResponse(p service.VendorPackagePrice) map[string]any {
	return map[string]any{
		"id": p.ID, "vendor_id": p.VendorID, "package_code": p.PackageCode, "machine_group": p.MachineGroup,
		"price_class": p.PriceClass, "tier_min": p.TierMin, "tier_max": p.TierMax,
		"base_price":       p.BasePrice,
		"vendor_branch_id": p.VendorBranchID, "atm_id": p.AtmID, "sla_note": p.SlaNote, "currency": p.Currency,
		"effective_start_date": p.EffectiveStartDate, "effective_end_date": p.EffectiveEndDate,
	}
}

// List handles GET / -- filterable, paginated package price list for one vendor.
func (h *AdminVendorPackagePriceHandler) List(w http.ResponseWriter, r *http.Request) {
	vendorID, err := parsePathID(r, "vendorID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	q := r.URL.Query()
	page, pageSize, err := parsePageParams(q)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	status, err := parseStatusParam(q)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	var packageCode, machineGroup, priceClass *string
	for _, f := range []struct {
		name string
		dst  **string
	}{{"package_code", &packageCode}, {"machine_group", &machineGroup}, {"price_class", &priceClass}} {
		if v := q.Get(f.name); v != "" {
			*f.dst = &v
		}
	}

	arg := db.ListVendorPackagePricesAdminParams{
		VendorID: vendorID, PackageCode: packageCode, MachineGroup: machineGroup, PriceClass: priceClass, Status: status,
		PageLimit: int64(pageSize), PageOffset: int64((page - 1) * pageSize),
	}
	prices, err := h.svc.List(r.Context(), arg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	total, err := h.svc.Count(r.Context(), db.CountVendorPackagePricesAdminParams{
		VendorID: vendorID, PackageCode: packageCode, MachineGroup: machineGroup, PriceClass: priceClass, Status: status,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}

	items := make([]map[string]any, len(prices))
	for i, p := range prices {
		items[i] = packagePriceToResponse(p)
	}
	writeJSON(w, http.StatusOK, map[string]any{"package_prices": items, "page": page, "page_size": pageSize, "total": total})
}

// Get handles GET /{id}.
func (h *AdminVendorPackagePriceHandler) Get(w http.ResponseWriter, r *http.Request) {
	vendorID, err := parsePathID(r, "vendorID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	p, err := h.svc.Get(r.Context(), vendorID, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	if p == nil {
		writeError(w, http.StatusNotFound, "not_found", service.ErrVendorPackagePriceNotFound.Error())
		return
	}
	writeJSON(w, http.StatusOK, packagePriceToResponse(*p))
}

// Create handles POST / -- stages a new price row for approval (202).
func (h *AdminVendorPackagePriceHandler) Create(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	vendorID, err := parsePathID(r, "vendorID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	var body service.VendorPackagePriceCreatePayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}

	change, err := h.svc.Create(r.Context(), authCtx.UserID, vendorID, body, extractClientIP(r))
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, changeRequestAcceptedResponse(change))
}

// Update handles PUT /{id} -- stages an edit of the content fields for approval (202).
func (h *AdminVendorPackagePriceHandler) Update(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	vendorID, err := parsePathID(r, "vendorID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	var body service.VendorPackagePriceContentPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}

	change, err := h.svc.Update(r.Context(), authCtx.UserID, vendorID, id, body, extractClientIP(r))
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, changeRequestAcceptedResponse(change))
}

// Disable handles POST /{id}/disable -- stages ending the price period for approval (202).
func (h *AdminVendorPackagePriceHandler) Disable(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	vendorID, err := parsePathID(r, "vendorID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	change, err := h.svc.Disable(r.Context(), authCtx.UserID, vendorID, id, extractClientIP(r))
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, changeRequestAcceptedResponse(change))
}

func (h *AdminVendorPackagePriceHandler) handleError(w http.ResponseWriter, err error) {
	var validationErr *service.ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeValidationError(w, validationErr.Field, validationErr.Message)
	case errors.Is(err, service.ErrVendorPackagePriceNotFound):
		writeError(w, http.StatusNotFound, "not_found", service.ErrVendorPackagePriceNotFound.Error())
	case errors.Is(err, service.ErrVendorPackagePriceNoVendor):
		writeError(w, http.StatusNotFound, "not_found", service.ErrVendorPackagePriceNoVendor.Error())
	case errors.Is(err, service.ErrVendorPackagePriceInternal):
		writeError(w, http.StatusConflict, "conflict", service.ErrVendorPackagePriceInternal.Error())
	case errors.Is(err, service.ErrVendorPackagePriceOverlap):
		writeError(w, http.StatusConflict, "conflict", service.ErrVendorPackagePriceOverlap.Error())
	case errors.Is(err, service.ErrMasterDataForbidden):
		writeForbidden(w, "Anda tidak berhak mengubah master data")
	case errors.Is(err, service.ErrMasterDataChangePending):
		writeError(w, http.StatusConflict, "conflict", service.ErrMasterDataChangePending.Error())
	default:
		writeUnexpectedError(w, err)
	}
}
