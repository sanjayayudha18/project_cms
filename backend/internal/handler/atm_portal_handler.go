package handler

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/service"
)

// Defaults from the ATM Portal API contract (design.md). Applied here, not
// in the service layer, since they're an HTTP-parsing concern — the
// service just validates whatever concrete params it's given.
const (
	defaultPage             = 1
	defaultPageSize         = 25
	defaultStatus           = "all"
	defaultSortBy           = "terminal_id"
	defaultSortOrder        = "asc"
	defaultCashposSortBy    = "cashpos_date"
	defaultCashposSortOrder = "desc"
)

// AtmPortalHandler handles ATM Portal HTTP endpoints.
type AtmPortalHandler struct {
	service service.AtmPortalServicer
}

// NewAtmPortalHandler creates a new AtmPortalHandler with the given service.
func NewAtmPortalHandler(svc service.AtmPortalServicer) *AtmPortalHandler {
	return &AtmPortalHandler{service: svc}
}

// Routes returns a chi.Router with all ATM Portal endpoints mounted.
func (h *AtmPortalHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/atms", h.ListATMs)
	r.Get("/cashpos", h.ListCashpos)
	return r
}

// ListATMs handles GET /atms — paginated, filtered, sorted ATM list with
// global summary and data-freshness timestamp.
func (h *AtmPortalHandler) ListATMs(w http.ResponseWriter, r *http.Request) {
	params, err := parseListATMsParams(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	result, err := h.service.ListATMs(r.Context(), params)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toListATMsResponse(result))
}

// ListCashpos handles GET /cashpos — paginated raw itm_cashpos rows.
// Monetary fields are decimal strings (no float).
func (h *AtmPortalHandler) ListCashpos(w http.ResponseWriter, r *http.Request) {
	params, err := parseListCashposParams(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	result, err := h.service.ListCashpos(r.Context(), params)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toListCashposResponse(result))
}

// parseListCashposParams parses cashpos query params with API defaults.
func parseListCashposParams(q url.Values) (service.ListCashposParams, error) {
	page, err := parseIntParam(q, "page", defaultPage)
	if err != nil {
		return service.ListCashposParams{}, fmt.Errorf("page harus berupa angka")
	}
	pageSize, err := parseIntParam(q, "page_size", defaultPageSize)
	if err != nil {
		return service.ListCashposParams{}, fmt.Errorf("page_size harus berupa angka")
	}

	return service.ListCashposParams{
		Page:      page,
		PageSize:  pageSize,
		Search:    q.Get("search"),
		DateFrom:  q.Get("date_from"),
		DateTo:    q.Get("date_to"),
		SortBy:    queryOrDefault(q, "sort_by", defaultCashposSortBy),
		SortOrder: queryOrDefault(q, "sort_order", defaultCashposSortOrder),
	}, nil
}

// parseListATMsParams parses query params into service.ListATMsParams,
// applying API-contract defaults for any parameter that's absent.
func parseListATMsParams(q url.Values) (service.ListATMsParams, error) {
	page, err := parseIntParam(q, "page", defaultPage)
	if err != nil {
		return service.ListATMsParams{}, fmt.Errorf("page harus berupa angka")
	}
	pageSize, err := parseIntParam(q, "page_size", defaultPageSize)
	if err != nil {
		return service.ListATMsParams{}, fmt.Errorf("page_size harus berupa angka")
	}

	return service.ListATMsParams{
		Page:           page,
		PageSize:       pageSize,
		Search:         q.Get("search"),
		Status:         queryOrDefault(q, "status", defaultStatus),
		MachineType:    q.Get("machine_type"),
		Brand:          q.Get("brand"),
		DeploymentType: q.Get("deployment_type"),
		Region:         q.Get("region"),
		DateFrom:       q.Get("date_from"),
		DateTo:         q.Get("date_to"),
		SortBy:         queryOrDefault(q, "sort_by", defaultSortBy),
		SortOrder:      queryOrDefault(q, "sort_order", defaultSortOrder),
	}, nil
}

// parseIntParam parses an integer query param, returning def when the key
// is absent or empty.
func parseIntParam(q url.Values, key string, def int) (int, error) {
	raw := q.Get(key)
	if raw == "" {
		return def, nil
	}
	return strconv.Atoi(raw)
}

// queryOrDefault returns the query param value, or def when absent/empty.
func queryOrDefault(q url.Values, key, def string) string {
	if v := q.Get(key); v != "" {
		return v
	}
	return def
}

// handleServiceError maps service package errors to HTTP responses.
func (h *AtmPortalHandler) handleServiceError(w http.ResponseWriter, err error) {
	var validationErr *service.ValidationError
	if errors.As(err, &validationErr) {
		// design.md's 400 Bad Request contract uses a single combined
		// message ("page_size harus antara 1 dan 100"), not the
		// structured {error, details[]} envelope writeValidationError
		// produces (422, used by the auth domain) — so writeError is used
		// directly here instead.
		writeError(w, http.StatusBadRequest, "bad_request",
			fmt.Sprintf("%s %s", validationErr.Field, validationErr.Message))
		return
	}

	writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
}

// atmPortalRow is a single ATM row in the API response, matching design.md's
// API Contract JSON field names exactly (low_threshold/critical_threshold,
// not the low_threshold_amount/critical_threshold_amount DB column names).
type atmPortalRow struct {
	TerminalID        string   `json:"terminal_id"`
	LocationName      string   `json:"location_name"`
	Address           string   `json:"address"`
	MachineType       string   `json:"machine_type"`
	Brand             string   `json:"brand"`
	DeploymentType    string   `json:"deployment_type"`
	LowThreshold      *float64 `json:"low_threshold"`
	CriticalThreshold *float64 `json:"critical_threshold"`
	LastReplenishDate *string  `json:"last_replenish_date"`
	LastReplenishTime *string  `json:"last_replenish_time"`
	RefundTotal       *float64 `json:"refund_total"`
	ReplenishTotal    *float64 `json:"replenish_total"`
	Escrow            *float64 `json:"escrow"`
	Status            string   `json:"status"`
}

// atmSummaryResponse mirrors service.ATMSummary for JSON output.
type atmSummaryResponse struct {
	Total        int64 `json:"total"`
	Critical     int64 `json:"critical"`
	Low          int64 `json:"low"`
	Normal       int64 `json:"normal"`
	Unconfigured int64 `json:"unconfigured"`
	NoData       int64 `json:"no_data"`
}

// listATMsResponse is the full JSON response for GET /atms.
type listATMsResponse struct {
	Data        []atmPortalRow     `json:"data"`
	Summary     atmSummaryResponse `json:"summary"`
	Total       int64              `json:"total"`
	Page        int                `json:"page"`
	PageSize    int                `json:"page_size"`
	LastUpdated *string            `json:"last_updated"`
}

// toListATMsResponse converts a service.ListATMsResult into the wire format.
func toListATMsResponse(result *service.ListATMsResult) listATMsResponse {
	data := make([]atmPortalRow, len(result.Data))
	for i, atm := range result.Data {
		data[i] = atmPortalRow{
			TerminalID:        atm.TerminalID,
			LocationName:      atm.LocationName,
			Address:           atm.Address,
			MachineType:       atm.MachineType,
			Brand:             atm.Brand,
			DeploymentType:    atm.DeploymentType,
			LowThreshold:      atm.LowThresholdAmount,
			CriticalThreshold: atm.CriticalThresholdAmount,
			LastReplenishDate: formatDatePtr(atm.LastReplenishDate),
			LastReplenishTime: atm.LastReplenishTime,
			RefundTotal:       atm.RefundTotal,
			ReplenishTotal:    atm.ReplenishTotal,
			Escrow:            atm.Escrow,
			Status:            atm.Status,
		}
	}

	return listATMsResponse{
		Data: data,
		Summary: atmSummaryResponse{
			Total:        result.Summary.Total,
			Critical:     result.Summary.Critical,
			Low:          result.Summary.Low,
			Normal:       result.Summary.Normal,
			Unconfigured: result.Summary.Unconfigured,
			NoData:       result.Summary.NoData,
		},
		Total:       result.Total,
		Page:        result.Page,
		PageSize:    result.PageSize,
		LastUpdated: formatTimePtr(result.LastUpdated),
	}
}

// formatDatePtr formats a *time.Time as "YYYY-MM-DD", or nil.
func formatDatePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format("2006-01-02")
	return &s
}

// formatTimePtr formats a *time.Time as RFC3339, or nil.
func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}

// cashposRow is a single itm_cashpos row in the API response. All 16
// denomination amounts are decimal strings to avoid float precision loss.
type cashposRow struct {
	ID               int64  `json:"id"`
	FileID           int64  `json:"file_id"`
	CashposDate      string `json:"cashpos_date"`
	TerminalID       string `json:"terminal_id"`
	MachineType      string `json:"machine_type"`
	TellerID         string `json:"teller_id"`
	BranchCode       string `json:"branch_code"`
	StartingCash10k  string `json:"starting_cash_10k"`
	CashIn10k        string `json:"cash_in_10k"`
	CashOut10k       string `json:"cash_out_10k"`
	CashPosition10k  string `json:"cash_position_10k"`
	StartingCash20k  string `json:"starting_cash_20k"`
	CashIn20k        string `json:"cash_in_20k"`
	CashOut20k       string `json:"cash_out_20k"`
	CashPosition20k  string `json:"cash_position_20k"`
	StartingCash50k  string `json:"starting_cash_50k"`
	CashIn50k        string `json:"cash_in_50k"`
	CashOut50k       string `json:"cash_out_50k"`
	CashPosition50k  string `json:"cash_position_50k"`
	StartingCash100k string `json:"starting_cash_100k"`
	CashIn100k       string `json:"cash_in_100k"`
	CashOut100k      string `json:"cash_out_100k"`
	CashPosition100k string `json:"cash_position_100k"`
	PositionSource   string `json:"position_source"`
	CreatedAt        string `json:"created_at"`
}

// listCashposResponse is the JSON body for GET /cashpos.
type listCashposResponse struct {
	Data     []cashposRow `json:"data"`
	Total    int64        `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
}

func toListCashposResponse(result *service.ListCashposResult) listCashposResponse {
	data := make([]cashposRow, len(result.Data))
	for i, row := range result.Data {
		data[i] = cashposRow{
			ID:               row.ID,
			FileID:           row.FileID,
			CashposDate:      row.CashposDate.Format("2006-01-02"),
			TerminalID:       row.TerminalID,
			MachineType:      row.MachineType,
			TellerID:         row.TellerID,
			BranchCode:       row.BranchCode,
			StartingCash10k:  row.StartingCash10k,
			CashIn10k:        row.CashIn10k,
			CashOut10k:       row.CashOut10k,
			CashPosition10k:  row.CashPosition10k,
			StartingCash20k:  row.StartingCash20k,
			CashIn20k:        row.CashIn20k,
			CashOut20k:       row.CashOut20k,
			CashPosition20k:  row.CashPosition20k,
			StartingCash50k:  row.StartingCash50k,
			CashIn50k:        row.CashIn50k,
			CashOut50k:       row.CashOut50k,
			CashPosition50k:  row.CashPosition50k,
			StartingCash100k: row.StartingCash100k,
			CashIn100k:       row.CashIn100k,
			CashOut100k:      row.CashOut100k,
			CashPosition100k: row.CashPosition100k,
			PositionSource:   row.PositionSource,
			CreatedAt:        row.CreatedAt.UTC().Format(time.RFC3339),
		}
	}
	return listCashposResponse{
		Data:     data,
		Total:    result.Total,
		Page:     result.Page,
		PageSize: result.PageSize,
	}
}
