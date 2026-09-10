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

// -- vendor request detail (POST /, PUT items, transitions, GET /{id}) ----

type vendorRequestItemResponse struct {
	ID              int64  `json:"id"`
	TerminalID      string `json:"terminal_id"`
	PeriodePred     string `json:"periode_pred"`
	Denom           int32  `json:"denom"`
	AmountReplenish int64  `json:"amount_replenish"`
	AmountRefund    int64  `json:"amount_refund"`
}

type vendorRequestDetailResponse struct {
	ID              int64                       `json:"id"`
	RequestNumber   string                      `json:"request_number"`
	ForecastDate    string                      `json:"forecast_date"`
	Status          string                      `json:"status"`
	Notes           string                      `json:"notes"`
	CreatedBy       vendorRequestUserRef        `json:"created_by"`
	ApprovedBy      *vendorRequestUserRef       `json:"approved_by"`
	RejectedBy      *vendorRequestUserRef       `json:"rejected_by"`
	RejectionReason string                      `json:"rejection_reason"`
	CreatedAt       string                      `json:"created_at"`
	UpdatedAt       string                      `json:"updated_at"`
	SubmittedAt     *string                     `json:"submitted_at"`
	ApprovedAt      *string                     `json:"approved_at"`
	RejectedAt      *string                     `json:"rejected_at"`
	Items           []vendorRequestItemResponse `json:"items"`
	TotalAmount     int64                       `json:"total_amount"`
}

func toDetailResponse(d *service.VendorRequestDetail) vendorRequestDetailResponse {
	items := make([]vendorRequestItemResponse, len(d.Items))
	for i, it := range d.Items {
		items[i] = vendorRequestItemResponse{
			ID: it.ID, TerminalID: it.TerminalID, PeriodePred: formatDate(it.PeriodePred),
			Denom: it.Denom, AmountReplenish: it.AmountReplenish, AmountRefund: it.AmountRefund,
		}
	}
	return vendorRequestDetailResponse{
		ID: d.ID, RequestNumber: d.RequestNumber, ForecastDate: formatDate(d.ForecastDate),
		Status: d.Status, Notes: d.Notes,
		CreatedBy:       vendorRequestUserRef{ID: d.CreatedBy.ID, FullName: d.CreatedBy.FullName},
		ApprovedBy:      toUserRefResponse(d.ApprovedBy),
		RejectedBy:      toUserRefResponse(d.RejectedBy),
		RejectionReason: d.RejectionReason,
		CreatedAt:       formatTimestamp(d.CreatedAt),
		UpdatedAt:       formatTimestamp(d.UpdatedAt),
		SubmittedAt:     formatTimestampPtr(d.SubmittedAt),
		ApprovedAt:      formatTimestampPtr(d.ApprovedAt),
		RejectedAt:      formatTimestampPtr(d.RejectedAt),
		Items:           items,
		TotalAmount:     d.TotalAmount,
	}
}

// -- GET / (list) ---------------------------------------------------------

type vendorRequestSummaryResponse struct {
	ID            int64                 `json:"id"`
	RequestNumber string                `json:"request_number"`
	ForecastDate  string                `json:"forecast_date"`
	Status        string                `json:"status"`
	Notes         string                `json:"notes"`
	ItemCount     int64                 `json:"item_count"`
	TotalAmount   int64                 `json:"total_amount"`
	CreatedBy     vendorRequestUserRef  `json:"created_by"`
	ApprovedBy    *vendorRequestUserRef `json:"approved_by"`
	CreatedAt     string                `json:"created_at"`
	SubmittedAt   *string               `json:"submitted_at"`
	ApprovedAt    *string               `json:"approved_at"`
	RejectedAt    *string               `json:"rejected_at"`
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
			CreatedBy:   vendorRequestUserRef{ID: r.CreatedBy.ID, FullName: r.CreatedBy.FullName},
			ApprovedBy:  toUserRefResponse(r.ApprovedBy),
			CreatedAt:   formatTimestamp(r.CreatedAt),
			SubmittedAt: formatTimestampPtr(r.SubmittedAt),
			ApprovedAt:  formatTimestampPtr(r.ApprovedAt),
			RejectedAt:  formatTimestampPtr(r.RejectedAt),
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
