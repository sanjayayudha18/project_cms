package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/service"
	"github.com/cimb-niaga/cms/pkg/middleware"
)

// DsrUploadHandler handles the vendor DSR upload endpoints.
type DsrUploadHandler struct {
	service *service.DsrService
}

// NewDsrUploadHandler creates a new DsrUploadHandler.
func NewDsrUploadHandler(svc *service.DsrService) *DsrUploadHandler {
	return &DsrUploadHandler{service: svc}
}

// Routes returns a chi.Router with all DSR upload endpoints mounted. Every
// route is vendor-scoped (FR13) -- callers with no VendorID claim (internal
// LDAP users) get 403 inside each handler.
func (h *DsrUploadHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/uploads", h.Upload)
	r.Post("/uploads/confirm", h.Confirm)
	r.Get("/uploads", h.ListUploads)
	r.Get("/uploads/daily/{fileId}", h.GetDaily)
	r.Get("/uploads/rencana-isi/{fileId}", h.GetRencanaIsi)
	return r
}

// vendorContext pulls the caller's vendor_id from the auth context and
// resolves it to the vendor's code + display name (code is what
// backend_python/dsr/dsr_etl.py's parse_upload_filename/resolve_vendor_name
// expects in the on-disk filename; name is what's stored in
// dsr_uploads.vendor and used for DB-side scoping). Returns ok=false
// after writing the appropriate error response.
func (h *DsrUploadHandler) vendorContext(w http.ResponseWriter, r *http.Request) (vendorID int64, vendorCode, vendorName string, ok bool) {
	authCtx, found := middleware.GetAuthContext(r.Context())
	if !found {
		writeUnauthorized(w, "Token tidak valid")
		return 0, "", "", false
	}
	if authCtx.VendorID == nil {
		writeForbidden(w, "Endpoint ini khusus untuk vendor")
		return 0, "", "", false
	}

	code, name, err := h.service.ResolveVendor(r.Context(), *authCtx.VendorID)
	if err != nil {
		if errors.Is(err, service.ErrDsrVendorNotFound) {
			writeForbidden(w, "Vendor tidak ditemukan atau tidak aktif")
			return 0, "", "", false
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return 0, "", "", false
	}
	return *authCtx.VendorID, code, name, true
}

// Upload handles POST /uploads (multipart "file"). Returns a DRY-RUN preview
// only -- nothing is written to the DSR tables yet. The vendor reviews the
// parsed data in the FE and calls POST /uploads/confirm to actually persist
// it (see service.DsrService's dry-run/confirm split for why).
func (h *DsrUploadHandler) Upload(w http.ResponseWriter, r *http.Request) {
	vendorID, vendorCode, _, ok := h.vendorContext(w, r)
	if !ok {
		return
	}

	if err := h.service.CheckUploadRate(r.Context(), vendorID); err != nil {
		if errors.Is(err, service.ErrDsrRateLimited) {
			writeTooManyRequests(w, "Batas upload per jam tercapai, coba lagi nanti", 3600)
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}

	// Cap the whole request body slightly above the file cap (multipart
	// framing overhead) -- ValidateUpload re-checks the exact file size.
	r.Body = http.MaxBytesReader(w, r.Body, service.DsrMaxUploadBytes+64*1024)
	if err := r.ParseMultipartForm(service.DsrMaxUploadBytes + 64*1024); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "File terlalu besar atau request tidak valid")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Field 'file' wajib diisi")
		return
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Gagal membaca file")
		return
	}

	sanitizedName, err := h.service.ValidateUpload(content, header.Filename)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	result, err := h.service.WriteAndDryRun(r.Context(), vendorCode, vendorID, sanitizedName, content)
	if err != nil {
		writeError(w, http.StatusBadGateway, "dry_run_failed", err.Error())
		return
	}

	// result.Raw is service_dsr_etl's parsed dry-run payload (header fields +
	// every row of both sheets), relayed to the browser as-is -- Go never
	// needs to interpret workbook contents, only stage it for confirm.
	writeJSON(w, http.StatusOK, result.Raw)
}

// confirmRequest is the body of POST /uploads/confirm.
type confirmRequest struct {
	StagedFilename string `json:"staged_filename"`
	Checksum       string `json:"checksum"`
}

// Confirm handles POST /uploads/confirm. The vendor has reviewed the dry-run
// preview and now asks the server to actually persist it -- this re-parses
// the staged file server-side (never trusting data echoed back by the
// browser) before writing to the DSR tables.
func (h *DsrUploadHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	_, vendorCode, _, ok := h.vendorContext(w, r)
	if !ok {
		return
	}

	var req confirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.StagedFilename == "" || req.Checksum == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "staged_filename dan checksum wajib diisi")
		return
	}

	// Defense in depth: the staged filename embeds <vendor_code>__<user_id>__<name>
	// (dsr_etl.py's contract) -- reject if it doesn't belong to the confirming
	// vendor, so one vendor can never commit another vendor's staged upload even
	// with a guessed/leaked filename (FR13).
	stagedVendorCode, _, ok := strings.Cut(req.StagedFilename, "__")
	if !ok || stagedVendorCode != vendorCode {
		writeForbidden(w, "File ini bukan milik vendor Anda")
		return
	}

	result, err := h.service.ConfirmUpload(r.Context(), req.StagedFilename, req.Checksum)
	if err != nil {
		writeError(w, http.StatusBadGateway, "confirm_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toDsrUploadResponse(result))
}

// GetDaily handles GET /uploads/daily/{fileId}.
func (h *DsrUploadHandler) GetDaily(w http.ResponseWriter, r *http.Request) {
	_, _, vendorName, ok := h.vendorContext(w, r)
	if !ok {
		return
	}
	fileID, err := parseFileID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	detail, err := h.service.GetSaldoFile(r.Context(), vendorName, fileID)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toDsrSheetDetailResponse(detail))
}

// GetRencanaIsi handles GET /uploads/rencana-isi/{fileId}.
func (h *DsrUploadHandler) GetRencanaIsi(w http.ResponseWriter, r *http.Request) {
	_, _, vendorName, ok := h.vendorContext(w, r)
	if !ok {
		return
	}
	fileID, err := parseFileID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	detail, err := h.service.GetRencanaIsiFile(r.Context(), vendorName, fileID)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toDsrSheetDetailResponse(detail))
}

// ListUploads handles GET /uploads (?date_from=&date_to=&page=&page_size=).
func (h *DsrUploadHandler) ListUploads(w http.ResponseWriter, r *http.Request) {
	_, _, vendorName, ok := h.vendorContext(w, r)
	if !ok {
		return
	}

	params, err := parseDsrListUploadsParams(vendorName, r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	result, err := h.service.ListUploads(r.Context(), params)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toDsrUploadListResponse(result))
}

func parseFileID(r *http.Request) (int64, error) {
	raw := chi.URLParam(r, "fileId")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("file id tidak valid")
	}
	return id, nil
}

func parseDsrListUploadsParams(vendorName string, q url.Values) (service.DsrListUploadsParams, error) {
	page, err := parseIntParam(q, "page", defaultPage)
	if err != nil {
		return service.DsrListUploadsParams{}, fmt.Errorf("page harus berupa angka")
	}
	pageSize, err := parseIntParam(q, "page_size", defaultPageSize)
	if err != nil {
		return service.DsrListUploadsParams{}, fmt.Errorf("page_size harus berupa angka")
	}
	return service.DsrListUploadsParams{
		VendorName: vendorName,
		Page:       page,
		PageSize:   pageSize,
		DateFrom:   q.Get("date_from"),
		DateTo:     q.Get("date_to"),
	}, nil
}

// handleServiceError maps service package errors to HTTP responses.
func (h *DsrUploadHandler) handleServiceError(w http.ResponseWriter, err error) {
	if errors.Is(err, service.ErrDsrFileNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "File tidak ditemukan")
		return
	}
	if errors.Is(err, service.ErrDsrVendorNotFound) {
		writeForbidden(w, "Vendor tidak ditemukan atau tidak aktif")
		return
	}

	var validationErr *service.ValidationError
	if errors.As(err, &validationErr) {
		writeError(w, http.StatusBadRequest, "bad_request",
			fmt.Sprintf("%s %s", validationErr.Field, validationErr.Message))
		return
	}

	writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
}

// -- JSON response shapes --------------------------------------------------

type dsrRowErrorResponse struct {
	RowNo int32  `json:"row_no"`
	Label string `json:"label"`
}

type dsrSheetResponse struct {
	FileID       *int64                `json:"file_id"`
	Status       string                `json:"status"`
	RowCount     int32                 `json:"row_count"`
	SuccessCount int32                 `json:"success_count"`
	ErrorCount   int32                 `json:"error_count"`
	Errors       []dsrRowErrorResponse `json:"errors"`
}

type dsrUploadResponse struct {
	Checksum   string           `json:"checksum"`
	Daily      dsrSheetResponse `json:"daily"`
	RencanaIsi dsrSheetResponse `json:"rencana_isi"`
}

func toDsrSheetResponse(r service.DsrSheetResult) dsrSheetResponse {
	errs := make([]dsrRowErrorResponse, len(r.Errors))
	for i, e := range r.Errors {
		errs[i] = dsrRowErrorResponse{RowNo: e.RowNo, Label: e.Label}
	}
	return dsrSheetResponse{
		FileID: r.FileID, Status: r.Status, RowCount: r.RowCount,
		SuccessCount: r.SuccessCount, ErrorCount: r.ErrorCount, Errors: errs,
	}
}

func toDsrUploadResponse(result *service.DsrUploadResult) dsrUploadResponse {
	return dsrUploadResponse{
		Checksum:   result.Checksum,
		Daily:      toDsrSheetResponse(result.Daily),
		RencanaIsi: toDsrSheetResponse(result.RencanaIsi),
	}
}

type dsrDailyRowResponse struct {
	RowNo        int32    `json:"row_no"`
	Section      string   `json:"section"`
	Flow         string   `json:"flow"`
	LineLabel    string   `json:"line_label"`
	MemoNo       *string  `json:"memo_no"`
	Denom100k    *float64 `json:"denom_100k"`
	Denom50k     *float64 `json:"denom_50k"`
	Denom20k     *float64 `json:"denom_20k"`
	Denom10k     *float64 `json:"denom_10k"`
	Denom5k      *float64 `json:"denom_5k"`
	Denom2k      *float64 `json:"denom_2k"`
	Denom1k      *float64 `json:"denom_1k"`
	LineTotalIdr *float64 `json:"line_total_idr"`
	Remarks      *string  `json:"remarks"`
}

type dsrRencanaIsiRowResponse struct {
	RowNo                int32    `json:"row_no"`
	AtmTerminalID        string   `json:"atm_terminal_id"`
	AtmLocation          *string  `json:"atm_location"`
	DenomConfig          *string  `json:"denom_config"`
	Fill100kIdr          *float64 `json:"fill_100k_idr"`
	Fill50kIdr           *float64 `json:"fill_50k_idr"`
	SplankBalance0800Idr *float64 `json:"splank_balance_0800_idr"`
	Remarks              *string  `json:"remarks"`
}

type dsrSheetDetailResponse struct {
	FileID         int64                      `json:"file_id"`
	Filename       string                     `json:"filename"`
	Status         string                     `json:"status"`
	ReportDate     *string                    `json:"report_date"`
	RowCount       int32                      `json:"row_count"`
	SuccessCount   int32                      `json:"success_count"`
	ErrorCount     int32                      `json:"error_count"`
	Errors         []dsrRowErrorResponse      `json:"errors"`
	DailyRows      []dsrDailyRowResponse      `json:"daily_rows,omitempty"`
	RencanaIsiRows []dsrRencanaIsiRowResponse `json:"rencana_isi_rows,omitempty"`
}

func toDsrSheetDetailResponse(d *service.DsrSheetDetail) dsrSheetDetailResponse {
	errs := make([]dsrRowErrorResponse, len(d.Errors))
	for i, e := range d.Errors {
		errs[i] = dsrRowErrorResponse{RowNo: e.RowNo, Label: e.Label}
	}
	dailyRows := make([]dsrDailyRowResponse, len(d.DailyRows))
	for i, r := range d.DailyRows {
		dailyRows[i] = dsrDailyRowResponse{
			RowNo: r.RowNo, Section: r.Section, Flow: r.Flow, LineLabel: r.LineLabel, MemoNo: r.MemoNo,
			Denom100k: r.Denom100k, Denom50k: r.Denom50k, Denom20k: r.Denom20k, Denom10k: r.Denom10k,
			Denom5k: r.Denom5k, Denom2k: r.Denom2k, Denom1k: r.Denom1k, LineTotalIdr: r.LineTotalIdr, Remarks: r.Remarks,
		}
	}
	rencanaRows := make([]dsrRencanaIsiRowResponse, len(d.RencanaIsiRows))
	for i, r := range d.RencanaIsiRows {
		rencanaRows[i] = dsrRencanaIsiRowResponse{
			RowNo: r.RowNo, AtmTerminalID: r.AtmTerminalID, AtmLocation: r.AtmLocation, DenomConfig: r.DenomConfig,
			Fill100kIdr: r.Fill100kIdr, Fill50kIdr: r.Fill50kIdr, SplankBalance0800Idr: r.SplankBalance0800Idr, Remarks: r.Remarks,
		}
	}
	return dsrSheetDetailResponse{
		FileID: d.FileID, Filename: d.Filename, Status: d.Status, ReportDate: formatDatePtr(d.ReportDate),
		RowCount: d.RowCount, SuccessCount: d.SuccessCount, ErrorCount: d.ErrorCount, Errors: errs,
		DailyRows: dailyRows, RencanaIsiRows: rencanaRows,
	}
}

type dsrSheetSummaryResponse struct {
	FileID int64  `json:"file_id"`
	Status string `json:"status"`
}

type dsrUploadListItemResponse struct {
	ReportDate string                   `json:"report_date"`
	Daily      *dsrSheetSummaryResponse `json:"daily"`
	RencanaIsi *dsrSheetSummaryResponse `json:"rencana_isi"`
}

type listDsrUploadsResponse struct {
	Data     []dsrUploadListItemResponse `json:"data"`
	Total    int64                       `json:"total"`
	Page     int                         `json:"page"`
	PageSize int                         `json:"page_size"`
}

func toDsrUploadListResponse(result *service.DsrUploadListResult) listDsrUploadsResponse {
	data := make([]dsrUploadListItemResponse, len(result.Data))
	for i, item := range result.Data {
		row := dsrUploadListItemResponse{ReportDate: item.ReportDate.Format("2006-01-02")}
		if item.Daily != nil {
			row.Daily = &dsrSheetSummaryResponse{FileID: item.Daily.FileID, Status: item.Daily.Status}
		}
		if item.RencanaIsi != nil {
			row.RencanaIsi = &dsrSheetSummaryResponse{FileID: item.RencanaIsi.FileID, Status: item.RencanaIsi.Status}
		}
		data[i] = row
	}
	return listDsrUploadsResponse{Data: data, Total: result.Total, Page: result.Page, PageSize: result.PageSize}
}
