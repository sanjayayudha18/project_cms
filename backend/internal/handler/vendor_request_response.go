package handler

import (
	"time"

	"github.com/cimb-niaga/cms/backend/internal/service"
)

// -- shared shapes ----------------------------------------------------------

type vendorRequestUserRef struct {
	ID       int64  `json:"id"`
	FullName string `json:"full_name"`
}

func toUserRefResponse(u *service.UserRef) *vendorRequestUserRef {
	if u == nil {
		return nil
	}
	return &vendorRequestUserRef{ID: u.ID, FullName: u.FullName}
}

type vendorRequestPagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalCount int64 `json:"total_count"`
	TotalPages int   `json:"total_pages"`
}

func formatDate(t time.Time) string { return t.Format("2006-01-02") }

func formatTimestamp(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func formatTimestampPtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := formatTimestamp(*t)
	return &s
}

// -- GET /forecast ------------------------------------------------------

type forecastRowResponse struct {
	TerminalID      string `json:"terminal_id"`
	PeriodePred     string `json:"periode_pred"`
	Denom           int32  `json:"denom"`
	AmountReplenish int64  `json:"amount_replenish"`
	AmountRefund    int64  `json:"amount_refund"`
	DmaaFileID      int64  `json:"dmaa_file_id"`
	LokasiATM       string `json:"lokasi_atm"`
	Brand           string `json:"brand"`
	FLMVendor       string `json:"flm_vendor"`
	FLMVendorRegion string `json:"flm_vendor_region"`
	// replenishment-request-enhancements (Req 5.6, 6.1): additive fields,
	// appended behind the pre-existing ones — no renames/removals, and
	// amount_refund above stays on the wire (Req 5.1 drops only the rendered
	// column). Escrow is a decimal string (never a JSON number, Req 5.10);
	// null when the terminal has no itm_replenish row (Req 5.11 renders "-").
	PriorityClass string  `json:"priority_class"`
	Paket         string  `json:"paket"`
	Escrow        *string `json:"escrow"`
	// forecast-browser-summary (FR3): additive.
	IsRequested bool `json:"is_requested"`
	// atm-visit-quota (FR5): sisa kunjungan, null = belum ada baris kuota.
	VisitRemaining  *int32 `json:"visit_remaining"`
	VisitQuotaTotal *int32 `json:"visit_quota_total"`
}

type forecastResponse struct {
	Data       []forecastRowResponse   `json:"data"`
	Pagination vendorRequestPagination `json:"pagination"`
}

func toForecastResponse(result *service.BrowseForecastResult) forecastResponse {
	data := make([]forecastRowResponse, len(result.Data))
	for i, row := range result.Data {
		data[i] = forecastRowResponse{
			TerminalID:      row.TerminalID,
			PeriodePred:     formatDate(row.PeriodePred),
			Denom:           row.Denom,
			AmountReplenish: row.AmountReplenish,
			AmountRefund:    row.AmountRefund,
			DmaaFileID:      row.DmaaFileID,
			LokasiATM:       row.LokasiATM,
			Brand:           row.Brand,
			FLMVendor:       row.FLMVendor,
			FLMVendorRegion: row.FLMVendorRegion,
			PriorityClass:   row.PriorityClass,
			Paket:           row.Paket,
			Escrow:          row.Escrow,
			IsRequested:     row.IsRequested,
			VisitRemaining:  row.VisitRemaining,
			VisitQuotaTotal: row.VisitQuotaTotal,
		}
	}
	return forecastResponse{
		Data: data,
		Pagination: vendorRequestPagination{
			Page: result.Page, PageSize: result.PageSize,
			TotalCount: result.Total, TotalPages: result.TotalPages,
		},
	}
}

// -- GET /forecast/summary -------------------------------------------------

type forecastSummaryTotalsResponse struct {
	ATMCount                   int64 `json:"atm_count"`
	RequestedATMCount          int64 `json:"requested_atm_count"`
	UnrequestedATMCount        int64 `json:"unrequested_atm_count"`
	UnassignedATMCount         int64 `json:"unassigned_atm_count"`
	AmountReplenish            int64 `json:"amount_replenish"`
	UnrequestedAmountReplenish int64 `json:"unrequested_amount_replenish"`
}

type forecastSummaryGroupResponse struct {
	VendorID                   int64  `json:"vendor_id"` // 0 = tanpa vendor aktif (atm-visit-quota FR6.6)
	FLMVendor                  string `json:"flm_vendor"`
	FLMVendorRegion            string `json:"flm_vendor_region"`
	ATMCount                   int64  `json:"atm_count"`
	RequestedATMCount          int64  `json:"requested_atm_count"`
	UnrequestedATMCount        int64  `json:"unrequested_atm_count"`
	AmountReplenish            int64  `json:"amount_replenish"`
	UnrequestedAmountReplenish int64  `json:"unrequested_amount_replenish"`
}

type forecastSummaryResponse struct {
	ForecastDate string                         `json:"forecast_date"`
	Totals       forecastSummaryTotalsResponse  `json:"totals"`
	Groups       []forecastSummaryGroupResponse `json:"groups"`
}

func toForecastSummaryResponse(result *service.ForecastSummaryResult) forecastSummaryResponse {
	groups := make([]forecastSummaryGroupResponse, len(result.Groups))
	for i, g := range result.Groups {
		groups[i] = forecastSummaryGroupResponse(g)
	}
	return forecastSummaryResponse{
		ForecastDate: formatDate(result.ForecastDate),
		Totals:       forecastSummaryTotalsResponse(result.Totals),
		Groups:       groups,
	}
}

// -- GET /vendors -----------------------------------------------------------

type vendorOptionResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type vendorOptionsResponse struct {
	Vendors []vendorOptionResponse `json:"vendors"`
	Regions []string               `json:"regions"`
}

func toVendorOptionsResponse(result *service.VendorOptionsResult) vendorOptionsResponse {
	vendors := make([]vendorOptionResponse, len(result.Vendors))
	for i, v := range result.Vendors {
		vendors[i] = vendorOptionResponse{ID: v.ID, Name: v.Name}
	}
	return vendorOptionsResponse{Vendors: vendors, Regions: result.Regions}
}

// -- vendor request detail (POST /, PUT items, transitions, GET /{id}) ----

type vendorRequestItemResponse struct {
	ID              int64  `json:"id"`
	TerminalID      string `json:"terminal_id"`
	PeriodePred     string `json:"periode_pred"`
	Denom           int32  `json:"denom"`
	AmountReplenish int64  `json:"amount_replenish"`
	AmountRefund    int64  `json:"amount_refund"`
	TicketNumber    string `json:"ticket_number"`
}

type vendorRequestDetailResponse struct {
	ID                 int64                       `json:"id"`
	RequestNumber      string                      `json:"request_number"`
	ForecastDate       string                      `json:"forecast_date"`
	Status             string                      `json:"status"`
	Notes              string                      `json:"notes"`
	CreatedBy          vendorRequestUserRef        `json:"created_by"`
	ApprovedBy         *vendorRequestUserRef       `json:"approved_by"`
	RejectedBy         *vendorRequestUserRef       `json:"rejected_by"`
	RejectionReason    string                      `json:"rejection_reason"`
	CreatedAt          string                      `json:"created_at"`
	UpdatedAt          string                      `json:"updated_at"`
	SubmittedAt        *string                     `json:"submitted_at"`
	ApprovedAt         *string                     `json:"approved_at"`
	RejectedAt         *string                     `json:"rejected_at"`
	Items              []vendorRequestItemResponse `json:"items"`
	TotalAmount        int64                       `json:"total_amount"`
	ReplenishDate      *string                     `json:"replenish_date"`
	RequestCategory    *string                     `json:"request_category"`
	IsCanceled         bool                        `json:"is_canceled"`
	IsManual           bool                        `json:"is_manual"`
	CancellationReason *string                     `json:"cancellation_reason"`
	RegionCode         *string                     `json:"region_code"`
	// atm-visit-quota (FR5): laporan selesai + kuota per ATM.
	CompletionSubmittedBy     *vendorRequestUserRef  `json:"completion_submitted_by"`
	CompletionSubmittedAt     *string                `json:"completion_submitted_at"`
	CompletionApprovedBy      *vendorRequestUserRef  `json:"completion_approved_by"`
	CompletionApprovedAt      *string                `json:"completion_approved_at"`
	CompletionRejectedBy      *vendorRequestUserRef  `json:"completion_rejected_by"`
	CompletionRejectedAt      *string                `json:"completion_rejected_at"`
	CompletionRejectionReason *string                `json:"completion_rejection_reason"`
	Atms                      []requestAtmStatusResp `json:"atms"`
	// Only on POST /{id}/complete/approve: ATMs that went over quota.
	OverQuotaTerminals []string `json:"over_quota_terminals,omitempty"`
}

type requestAtmStatusResp struct {
	TerminalID       string  `json:"terminal_id"`
	TicketNumber     string  `json:"ticket_number"`
	CompletionResult *string `json:"completion_result"`
	VisitRemaining   *int32  `json:"visit_remaining"`
	VisitQuotaTotal  *int32  `json:"visit_quota_total"`
	IsOverQuota      bool    `json:"is_over_quota"`
}

func toDetailResponse(d *service.VendorRequestDetail) vendorRequestDetailResponse {
	items := make([]vendorRequestItemResponse, len(d.Items))
	for i, it := range d.Items {
		items[i] = vendorRequestItemResponse{
			ID: it.ID, TerminalID: it.TerminalID, PeriodePred: formatDate(it.PeriodePred),
			Denom: it.Denom, AmountReplenish: it.AmountReplenish, AmountRefund: it.AmountRefund,
			TicketNumber: it.TicketNumber,
		}
	}
	return vendorRequestDetailResponse{
		ID: d.ID, RequestNumber: d.RequestNumber, ForecastDate: formatDate(d.ForecastDate),
		Status: d.Status, Notes: d.Notes,
		CreatedBy:          vendorRequestUserRef{ID: d.CreatedBy.ID, FullName: d.CreatedBy.FullName},
		ApprovedBy:         toUserRefResponse(d.ApprovedBy),
		RejectedBy:         toUserRefResponse(d.RejectedBy),
		RejectionReason:    d.RejectionReason,
		CreatedAt:          formatTimestamp(d.CreatedAt),
		UpdatedAt:          formatTimestamp(d.UpdatedAt),
		SubmittedAt:        formatTimestampPtr(d.SubmittedAt),
		ApprovedAt:         formatTimestampPtr(d.ApprovedAt),
		RejectedAt:         formatTimestampPtr(d.RejectedAt),
		Items:              items,
		TotalAmount:        d.TotalAmount,
		ReplenishDate:      formatDatePtr(d.ReplenishDate),
		RequestCategory:    d.RequestCategory,
		IsCanceled:         d.IsCanceled,
		IsManual:           d.IsManual,
		CancellationReason: d.CancellationReason,
		RegionCode:         d.RegionCode,

		CompletionSubmittedBy:     toUserRefResponse(d.CompletionSubmittedBy),
		CompletionSubmittedAt:     formatTimestampPtr(d.CompletionSubmittedAt),
		CompletionApprovedBy:      toUserRefResponse(d.CompletionApprovedBy),
		CompletionApprovedAt:      formatTimestampPtr(d.CompletionApprovedAt),
		CompletionRejectedBy:      toUserRefResponse(d.CompletionRejectedBy),
		CompletionRejectedAt:      formatTimestampPtr(d.CompletionRejectedAt),
		CompletionRejectionReason: d.CompletionRejectionReason,
		Atms:                      toRequestAtmStatusResp(d.Atms),
	}
}

func toRequestAtmStatusResp(in []service.RequestAtmStatus) []requestAtmStatusResp {
	out := make([]requestAtmStatusResp, len(in))
	for i, a := range in {
		out[i] = requestAtmStatusResp(a)
	}
	return out
}

// -- GET / (list) ---------------------------------------------------------

type vendorRequestSummaryResponse struct {
	ID                 int64                 `json:"id"`
	RequestNumber      string                `json:"request_number"`
	ForecastDate       string                `json:"forecast_date"`
	Status             string                `json:"status"`
	Notes              string                `json:"notes"`
	ItemCount          int64                 `json:"item_count"`
	TotalAmount        int64                 `json:"total_amount"`
	CreatedBy          vendorRequestUserRef  `json:"created_by"`
	ApprovedBy         *vendorRequestUserRef `json:"approved_by"`
	CreatedAt          string                `json:"created_at"`
	SubmittedAt        *string               `json:"submitted_at"`
	ApprovedAt         *string               `json:"approved_at"`
	RejectedAt         *string               `json:"rejected_at"`
	ReplenishDate      *string               `json:"replenish_date"`
	RequestCategory    *string               `json:"request_category"`
	IsCanceled         bool                  `json:"is_canceled"`
	IsManual           bool                  `json:"is_manual"`
	CancellationReason *string               `json:"cancellation_reason"`
}

type vendorRequestListResponse struct {
	Data       []vendorRequestSummaryResponse `json:"data"`
	Pagination vendorRequestPagination        `json:"pagination"`
}

func toListResponse(result *service.ListVendorRequestResult) vendorRequestListResponse {
	data := make([]vendorRequestSummaryResponse, len(result.Data))
	for i, r := range result.Data {
		data[i] = vendorRequestSummaryResponse{
			ID: r.ID, RequestNumber: r.RequestNumber, ForecastDate: formatDate(r.ForecastDate),
			Status: r.Status, Notes: r.Notes, ItemCount: r.ItemCount, TotalAmount: r.TotalAmount,
			CreatedBy:          vendorRequestUserRef{ID: r.CreatedBy.ID, FullName: r.CreatedBy.FullName},
			ApprovedBy:         toUserRefResponse(r.ApprovedBy),
			CreatedAt:          formatTimestamp(r.CreatedAt),
			SubmittedAt:        formatTimestampPtr(r.SubmittedAt),
			ApprovedAt:         formatTimestampPtr(r.ApprovedAt),
			RejectedAt:         formatTimestampPtr(r.RejectedAt),
			ReplenishDate:      formatDatePtr(r.ReplenishDate),
			RequestCategory:    r.RequestCategory,
			IsCanceled:         r.IsCanceled,
			IsManual:           r.IsManual,
			CancellationReason: r.CancellationReason,
		}
	}
	return vendorRequestListResponse{
		Data: data,
		Pagination: vendorRequestPagination{
			Page: result.Page, PageSize: result.PageSize,
			TotalCount: result.Total, TotalPages: result.TotalPages,
		},
	}
}

// -- GET /{id}/audit-log ----------------------------------------------------

type auditLogEntryResponse struct {
	ID            int64          `json:"id"`
	EntityType    string         `json:"entity_type"`
	EntityID      int64          `json:"entity_id"`
	Action        string         `json:"action"`
	PerformedBy   int64          `json:"performed_by"`
	PerformedAt   string         `json:"performed_at"`
	PreviousState *string        `json:"previous_state"`
	NewState      *string        `json:"new_state"`
	Metadata      map[string]any `json:"metadata"`
}

type auditLogResponse struct {
	Data []auditLogEntryResponse `json:"data"`
}

func toAuditLogResponse(entries []service.AuditEntry) auditLogResponse {
	data := make([]auditLogEntryResponse, len(entries))
	for i, e := range entries {
		data[i] = auditLogEntryResponse{
			ID: e.ID, EntityType: e.EntityType, EntityID: e.EntityID, Action: e.Action,
			PerformedBy: e.PerformedBy, PerformedAt: formatTimestamp(e.PerformedAt),
			PreviousState: e.PreviousState, NewState: e.NewState, Metadata: e.Metadata,
		}
	}
	return auditLogResponse{Data: data}
}
