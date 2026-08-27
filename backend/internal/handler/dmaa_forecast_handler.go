package handler

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/service"
)

// Defaults from the DMAA Forecast Viewer API contract (design.md). Applied
// at the HTTP parsing layer, matching the ATM Portal convention.
const (
	defaultDmaaSortBy    = "periode_pred"
	defaultDmaaSortOrder = "desc"
)

// DmaaForecastHandler handles DMAA Forecast Viewer HTTP endpoints.
type DmaaForecastHandler struct {
	service service.DmaaForecastServicer
}

// NewDmaaForecastHandler creates a new DmaaForecastHandler with the given service.
func NewDmaaForecastHandler(svc service.DmaaForecastServicer) *DmaaForecastHandler {
	return &DmaaForecastHandler{service: svc}
}

// Routes returns a chi.Router with all DMAA Forecast endpoints mounted.
func (h *DmaaForecastHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.ListDmaaForecast)
	return r
}

// ListDmaaForecast handles GET /api/v1/dmaa-forecast — paginated, filtered,
// sorted dmaa_atm_forecast rows.
func (h *DmaaForecastHandler) ListDmaaForecast(w http.ResponseWriter, r *http.Request) {
	params, err := parseDmaaForecastParams(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	result, err := h.service.ListDmaaForecast(r.Context(), params)
	if err != nil {
		h.handleDmaaServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toDmaaForecastResponse(result))
}

// parseDmaaForecastParams parses query params into
// service.ListDmaaForecastParams, applying API-contract defaults.
func parseDmaaForecastParams(q url.Values) (service.ListDmaaForecastParams, error) {
	page, err := parseIntParam(q, "page", defaultPage)
	if err != nil {
		return service.ListDmaaForecastParams{}, fmt.Errorf("page harus berupa angka")
	}
	pageSize, err := parseIntParam(q, "page_size", defaultPageSize)
	if err != nil {
		return service.ListDmaaForecastParams{}, fmt.Errorf("page_size harus berupa angka")
	}

	return service.ListDmaaForecastParams{
		Page:       page,
		PageSize:   pageSize,
		DateFrom:   q.Get("date_from"),
		DateTo:     q.Get("date_to"),
		TerminalID: q.Get("terminal_id"),
		SortBy:     queryOrDefault(q, "sort_by", defaultDmaaSortBy),
		SortOrder:  queryOrDefault(q, "sort_order", defaultDmaaSortOrder),
	}, nil
}

// handleDmaaServiceError maps service package errors to HTTP responses:
// ValidationError → 400, anything else (incl. DB failures) → 503 per
// design.md's error table (read replica unavailable).
func (h *DmaaForecastHandler) handleDmaaServiceError(w http.ResponseWriter, err error) {
	var validationErr *service.ValidationError
	if errors.As(err, &validationErr) {
		writeError(w, http.StatusBadRequest, "bad_request",
			fmt.Sprintf("%s %s", validationErr.Field, validationErr.Message))
		return
	}

	writeServiceUnavailable(w, "Layanan DMAA Forecast sementara tidak tersedia")
}

// dmaaForecastResponseRow is a single forecast row in the API response.
type dmaaForecastResponseRow struct {
	TerminalID      string `json:"terminal_id"`
	DmaaFileID      int64  `json:"dmaa_file_id"`
	PeriodePred     string `json:"periode_pred"`
	Denom           int32  `json:"denom"`
	AmountReplenish int64  `json:"amount_replenish"`
	AmountRefund    int64  `json:"amount_refund"`
	CreatedAt       string `json:"created_at"`
}

// dmaaForecastPagination is the pagination metadata block.
type dmaaForecastPagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalRows  int64 `json:"total_rows"`
	TotalPages int   `json:"total_pages"`
}

// dmaaForecastResponse is the full JSON body for GET /api/v1/dmaa-forecast.
type dmaaForecastResponse struct {
	Data       []dmaaForecastResponseRow `json:"data"`
	Pagination dmaaForecastPagination    `json:"pagination"`
}

// toDmaaForecastResponse converts a service.ListDmaaForecastResult into the
// wire format.
func toDmaaForecastResponse(result *service.ListDmaaForecastResult) dmaaForecastResponse {
	data := make([]dmaaForecastResponseRow, len(result.Data))
	for i, row := range result.Data {
		data[i] = dmaaForecastResponseRow{
			TerminalID:      row.TerminalID,
			DmaaFileID:      row.DmaaFileID,
			PeriodePred:     row.PeriodePred.Format("2006-01-02"),
			Denom:           row.Denom,
			AmountReplenish: row.AmountReplenish,
			AmountRefund:    row.AmountRefund,
			CreatedAt:       row.CreatedAt.UTC().Format(time.RFC3339),
		}
	}

	return dmaaForecastResponse{
		Data: data,
		Pagination: dmaaForecastPagination{
			Page:       result.Page,
			PageSize:   result.PageSize,
			TotalRows:  result.Total,
			TotalPages: result.TotalPages,
		},
	}
}
