package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/service"
	"github.com/cimb-niaga/cms/pkg/middleware"
)

// CashCountCategoryParamsServicer is the subset of service.CashCountCategoryParamsAdminService the handler needs.
type CashCountCategoryParamsServicer interface {
	List(ctx context.Context) ([]db.CashCountCategoryParam, error)
	Create(ctx context.Context, makerID int64, req service.CashCountCategoryParamsPayload, actorIP string) (db.MasterDataChangeRequest, error)
}

// CashCountCategoryParamsHandler serves "Parameter Kategori Vault" (ScDD 5.1,
// AC-5.1-01/04). Mount at /api/v1/cash-count/category-params; the maker role
// (CC-CWM) is re-checked in MasterDataChangeService.Submit.
type CashCountCategoryParamsHandler struct {
	svc CashCountCategoryParamsServicer
}

// NewCashCountCategoryParamsHandler creates a CashCountCategoryParamsHandler.
func NewCashCountCategoryParamsHandler(svc CashCountCategoryParamsServicer) *CashCountCategoryParamsHandler {
	return &CashCountCategoryParamsHandler{svc: svc}
}

// Routes returns the router: GET / (active set + history), POST / (propose a new set).
func (h *CashCountCategoryParamsHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	return r
}

// List handles GET / -> {"active": set|null, "history": [superseded sets]}.
func (h *CashCountCategoryParamsHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.List(r.Context())
	if err != nil {
		writeUnexpectedError(w, err)
		return
	}
	var active any
	history := make([]map[string]any, 0, len(rows))
	for _, p := range rows {
		item := cashCountCategoryParamsJSON(p)
		if !p.SupersededAt.Valid && active == nil {
			active = item
			continue
		}
		history = append(history, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"active": active, "history": history})
}

// Create handles POST / {medium_min_amount, medium_max_amount, currency_code,
// onsite_high, onsite_medium, onsite_low} -- 202 pending approval.
func (h *CashCountCategoryParamsHandler) Create(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	var body service.CashCountCategoryParamsPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}
	change, err := h.svc.Create(r.Context(), authCtx.UserID, body, extractClientIP(r))
	var validationErr *service.ValidationError
	switch {
	case err == nil:
		writeJSON(w, http.StatusAccepted, changeRequestAcceptedResponse(change))
	case errors.As(err, &validationErr):
		writeValidationError(w, validationErr.Field, validationErr.Message)
	case errors.Is(err, service.ErrCashCountCategoryParamsMissing), errors.Is(err, service.ErrMasterDataChangePending):
		writeError(w, http.StatusConflict, "conflict", err.Error())
	case errors.Is(err, service.ErrMasterDataForbidden):
		writeForbidden(w, "Anda tidak berhak mengubah parameter cash count")
	default:
		writeUnexpectedError(w, err)
	}
}

func cashCountCategoryParamsJSON(p db.CashCountCategoryParam) map[string]any {
	return map[string]any{
		"id":                p.ID,
		"medium_min_amount": cashCountAmountString(p.MediumMinAmount),
		"medium_max_amount": cashCountAmountString(p.MediumMaxAmount),
		"currency_code":     p.CurrencyCode,
		"onsite_high":       p.OnsiteHigh,
		"onsite_medium":     p.OnsiteMedium,
		"onsite_low":        p.OnsiteLow,
		"period_months":     p.PeriodMonths,
		"created_by":        p.CreatedBy,
		"effective_from":    formatTimestamptz(p.EffectiveFrom),
		"superseded_at":     formatTimestamptz(p.SupersededAt),
	}
}

// cashCountAmountString renders a numeric as exact decimal text ("5000000000.00"), never float.
func cashCountAmountString(n pgtype.Numeric) *string {
	v, err := n.Value()
	if err != nil || v == nil {
		return nil
	}
	s := fmt.Sprint(v)
	return &s
}
