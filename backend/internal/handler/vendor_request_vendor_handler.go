package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/cimb-niaga/cms/backend/internal/service"
)

// VendorRequestPartyReader is the slice of service.VendorOrderService behind
// the internal "Status Vendor" panel (cit-send-vendor FR6.6).
type VendorRequestPartyReader interface {
	RequestParties(ctx context.Context, requestID int64) (*service.RequestVendorParties, error)
}

// VendorRequestReturner is the slice of service.VendorRequestService behind
// "Kembalikan ke pembuat" (FR6.1).
type VendorRequestReturner interface {
	VendorReturn(ctx context.Context, actor service.Actor, id int64, reason string) (*service.VendorRequestDetail, error)
}

type vendorSide struct {
	parties  VendorRequestPartyReader
	returner VendorRequestReturner
}

// WithVendorSide mounts /{id}/vendor-parties and /{id}/vendor-return.
func (h *VendorRequestHandler) WithVendorSide(p VendorRequestPartyReader, r VendorRequestReturner) *VendorRequestHandler {
	h.vendor = &vendorSide{parties: p, returner: r}
	return h
}

// VendorParties handles GET /{id}/vendor-parties.
func (h *VendorRequestHandler) VendorParties(w http.ResponseWriter, r *http.Request) {
	id, err := parseVendorRequestID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	out, err := h.vendor.parties.RequestParties(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// VendorReturn handles POST /{id}/vendor-return {rejection_reason}: vendor_rejected -> rejected.
func (h *VendorRequestHandler) VendorReturn(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	id, err := parseVendorRequestID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	var body rejectBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "body tidak valid")
		return
	}
	result, err := h.vendor.returner.VendorReturn(r.Context(), actor, id, body.RejectionReason)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toDetailResponse(result))
}
