package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

const (
	// DsrMaxUploadBytes caps the vendor DSR workbook upload size (intent.md
	// security constraint: validated before any disk write).
	DsrMaxUploadBytes = 10 * 1024 * 1024

	dsrUploadRateLimit = 10               // uploads/hour/vendor (intent.md)
	dsrProcessTimeout  = 25 * time.Second // Go's blocking budget to service_dsr_etl (URS <=30s NFR)
)

var (
	// ErrDsrVendorNotFound is returned when the JWT's vendor_id doesn't resolve to an
	// active vendor -- should not happen for a validly-issued token, but guarded anyway.
	ErrDsrVendorNotFound = errors.New("vendor not found")
	// ErrDsrFileNotFound is returned by the detail endpoints for an unknown or
	// not-this-vendor's file_id.
	ErrDsrFileNotFound = errors.New("dsr file not found")
	// ErrDsrRateLimited is returned when a vendor exceeds the per-hour upload cap.
	ErrDsrRateLimited = errors.New("dsr upload rate limit exceeded")

	oleMagic = []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1} // legacy .xls (OLE2/BIFF)
	zipMagic = []byte{0x50, 0x4B, 0x03, 0x04}                         // .xlsx ("PK\x03\x04")
)

// DsrRepository is the small, service-defined interface the sqlc-generated
// *db.Queries satisfies (Go convention: interface defined where it's used).
type DsrRepository interface {
	GetVendorByID(ctx context.Context, id int64) (db.GetVendorByIDRow, error)
	GetDsrUploadByChecksum(ctx context.Context, checksum string) (db.GetDsrUploadByChecksumRow, error)
	GetDsrUploadByIDForVendor(ctx context.Context, arg db.GetDsrUploadByIDForVendorParams) (db.GetDsrUploadByIDForVendorRow, error)
	ListDsrDailyRowErrors(ctx context.Context, uploadID int64) ([]db.ListDsrDailyRowErrorsRow, error)
	ListDsrRencanaIsiRowErrors(ctx context.Context, uploadID int64) ([]db.ListDsrRencanaIsiRowErrorsRow, error)
	ListDsrDailyRows(ctx context.Context, uploadID int64) ([]db.ListDsrDailyRowsRow, error)
	ListDsrRencanaIsiRows(ctx context.Context, uploadID int64) ([]db.ListDsrRencanaIsiRowsRow, error)
	ListDsrUploadsByVendor(ctx context.Context, arg db.ListDsrUploadsByVendorParams) ([]db.ListDsrUploadsByVendorRow, error)
	CountDsrUploadsByVendor(ctx context.Context, arg db.CountDsrUploadsByVendorParams) (int64, error)
}

// DsrService implements the vendor DSR upload pipeline: validate -> write ->
// dry-run parse (preview, no DB write) -> vendor confirms -> commit (re-parse
// the same staged file, write to DB). Parsing itself happens entirely in
// backend_python/dsr/dsr_etl.py -- this service never reads the workbook's
// contents; it only relays service_dsr_etl's parsed JSON to the caller.
type DsrService struct {
	repo         DsrRepository
	redis        *redis.Client
	httpClient   *http.Client
	uploadDir    string // FTP_DATA/DSR
	retryBaseURL string // e.g. http://localhost:8090 -- service_dsr_etl's base URL
	processAuth  string // Authorization header value for service_dsr_etl's require_auth
}

// NewDsrService creates a new DsrService.
func NewDsrService(repo DsrRepository, redisClient *redis.Client, httpClient *http.Client, uploadDir, retryBaseURL, processAuth string) *DsrService {
	return &DsrService{
		repo:         repo,
		redis:        redisClient,
		httpClient:   httpClient,
		uploadDir:    uploadDir,
		retryBaseURL: strings.TrimRight(retryBaseURL, "/"),
		processAuth:  processAuth,
	}
}

// DsrRowError is one row-level ingest error (broken denom cell on Daily, or an
// unresolved ATM terminal id on Rencana Isi).
type DsrRowError struct {
	RowNo int32
	Label string
}

// DsrSheetResult is one sheet's ingest outcome. Both sheets live on the same
// dsr_uploads row but keep their own status/counts -- either may succeed alone.
type DsrSheetResult struct {
	Status       string // pending|processing|completed|failed|skipped
	FileID       *int64
	RowCount     int32
	SuccessCount int32
	ErrorCount   int32
	Errors       []DsrRowError
}

// DsrUploadResult is the full response to a vendor upload.
type DsrUploadResult struct {
	Checksum   string
	Daily      DsrSheetResult
	RencanaIsi DsrSheetResult
}

// ValidateUpload checks size + magic bytes and returns a sanitized basename,
// before any disk write (intent.md security constraint).
func (s *DsrService) ValidateUpload(content []byte, originalFilename string) (string, error) {
	if len(content) == 0 {
		return "", &ValidationError{Field: "file", Message: "file kosong"}
	}
	if len(content) > DsrMaxUploadBytes {
		return "", &ValidationError{Field: "file", Message: "ukuran file melebihi 10MB"}
	}
	if !bytes.HasPrefix(content, oleMagic) && !bytes.HasPrefix(content, zipMagic) {
		return "", &ValidationError{Field: "file", Message: "format file tidak valid (harus .xls atau .xlsx)"}
	}
	return sanitizeFilename(originalFilename)
}

// sanitizeFilename reduces a client-supplied filename to a safe basename,
// rejecting path traversal / directory components.
func sanitizeFilename(name string) (string, error) {
	base := filepath.Base(filepath.Clean(name))
	if base == "" || base == "." || base == ".." || base == string(filepath.Separator) {
		return "", &ValidationError{Field: "file", Message: "nama file tidak valid"}
	}
	base = strings.ReplaceAll(base, "/", "_")
	base = strings.ReplaceAll(base, "\\", "_")
	return base, nil
}

// ResolveVendor resolves vendor_code + display name from the JWT's vendor_id
// claim -- never from a client-supplied identifier (intent.md constraint).
func (s *DsrService) ResolveVendor(ctx context.Context, vendorID int64) (code, name string, err error) {
	v, err := s.repo.GetVendorByID(ctx, vendorID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", ErrDsrVendorNotFound
		}
		return "", "", fmt.Errorf("resolving vendor: %w", err)
	}
	return v.Code, v.Name, nil
}

// CheckUploadRate enforces a per-vendor, per-hour upload cap via a plain
// Redis INCR+EXPIRE counter (not pkg/middleware.RateLimiter, whose
// username+IP/increment-on-failure shape is login-attempt-specific).
func (s *DsrService) CheckUploadRate(ctx context.Context, vendorID int64) error {
	bucket := time.Now().UTC().Format("2006010215")
	key := fmt.Sprintf("dsr:upload:%d:%s", vendorID, bucket)

	count, err := s.redis.Incr(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("checking upload rate: %w", err)
	}
	if count == 1 {
		s.redis.Expire(ctx, key, time.Hour)
	}
	if count > dsrUploadRateLimit {
		return ErrDsrRateLimited
	}
	return nil
}

// DsrDryRunResult is a thin wrapper around service_dsr_etl's dry-run response:
// Go only needs Checksum + StagedFilename out of it (to drive the eventual
// confirm call and the post-commit DB read-back) -- everything else (parsed
// header fields, every row of both sheets) is relayed to the browser
// untouched via Raw, since Go never needs to interpret workbook contents.
type DsrDryRunResult struct {
	Checksum       string
	StagedFilename string
	Raw            json.RawMessage
}

// WriteAndDryRun writes the validated file to FTP_DATA/DSR/ and synchronously
// calls service_dsr_etl's POST /process/dsr/dry-run (up to dsrProcessTimeout).
// Unlike the old single-phase flow, a timeout/failure here is a hard error:
// nothing has been parsed or staged yet, so there is nothing to fall back to
// -- the vendor is asked to retry the upload.
func (s *DsrService) WriteAndDryRun(ctx context.Context, vendorCode string, userID int64, sanitizedName string, content []byte) (*DsrDryRunResult, error) {
	// <vendor_code>__<user_id>__<original_filename> -- the filename contract
	// backend_python/dsr/dsr_etl.py's parse_upload_filename expects.
	diskName := fmt.Sprintf("%s__%d__%s", vendorCode, userID, sanitizedName)
	if err := os.MkdirAll(s.uploadDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating upload dir: %w", err)
	}
	diskPath := filepath.Join(s.uploadDir, diskName)
	if err := os.WriteFile(diskPath, content, 0o644); err != nil {
		return nil, fmt.Errorf("writing upload: %w", err)
	}

	data, err := s.callProcess(ctx, "/process/dsr/dry-run", diskName)
	if err != nil {
		return nil, fmt.Errorf("dry-run parse failed, please try uploading again: %w", err)
	}

	var minimal struct {
		Checksum       string `json:"checksum"`
		StagedFilename string `json:"staged_filename"`
	}
	if err := json.Unmarshal(data, &minimal); err != nil {
		return nil, fmt.Errorf("decoding dry-run result: %w", err)
	}
	return &DsrDryRunResult{Checksum: minimal.Checksum, StagedFilename: minimal.StagedFilename, Raw: data}, nil
}

// ConfirmUpload triggers service_dsr_etl's POST /process/dsr/commit against
// the staged file left behind by WriteAndDryRun, re-parsing it from scratch
// server-side -- never trusting row data the browser echoes back in the
// confirm request, which matters for money-adjacent fields. On success it
// reads the now-persisted rows back from the DB (BuildUploadResult).
func (s *DsrService) ConfirmUpload(ctx context.Context, stagedFilename, checksum string) (*DsrUploadResult, error) {
	if _, err := s.callProcess(ctx, "/process/dsr/commit", stagedFilename); err != nil {
		return nil, fmt.Errorf("confirming upload: %w", err)
	}
	return s.BuildUploadResult(ctx, checksum)
}

// callProcess POSTs {"filename": filename} to service_dsr_etl at path and
// returns the envelope's "data" field, or an error built from the envelope's
// "error" field / a connection failure / a non-2xx status.
func (s *DsrService) callProcess(ctx context.Context, path, filename string) (json.RawMessage, error) {
	triggerCtx, cancel := context.WithTimeout(ctx, dsrProcessTimeout)
	defer cancel()

	body, err := json.Marshal(map[string]string{"filename": filename})
	if err != nil {
		return nil, fmt.Errorf("encoding request: %w", err)
	}

	req, err := http.NewRequestWithContext(triggerCtx, http.MethodPost, s.retryBaseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if s.processAuth != "" {
		req.Header.Set("Authorization", s.processAuth)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling service_dsr_etl: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading service_dsr_etl response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("service_dsr_etl returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var envelope struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
		Error  *string         `json:"error"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return nil, fmt.Errorf("decoding service_dsr_etl response: %w", err)
	}
	if envelope.Status != "success" {
		msg := "unknown error"
		if envelope.Error != nil {
			msg = *envelope.Error
		}
		return nil, errors.New(msg)
	}
	return envelope.Data, nil
}

// BuildUploadResult reads the upload row by checksum. Called after
// ConfirmUpload's commit call succeeds, by which point the rows are durably
// persisted (commit is itself synchronous).
func (s *DsrService) BuildUploadResult(ctx context.Context, checksum string) (*DsrUploadResult, error) {
	result := &DsrUploadResult{Checksum: checksum}

	upload, err := s.repo.GetDsrUploadByChecksum(ctx, checksum)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Commit reported success but nothing landed -- report both sheets
			// pending rather than failing the vendor's request.
			result.Daily = DsrSheetResult{Status: "pending"}
			result.RencanaIsi = DsrSheetResult{Status: "pending"}
			return result, nil
		}
		return nil, fmt.Errorf("fetching upload: %w", err)
	}

	dailyErrs, err := s.repo.ListDsrDailyRowErrors(ctx, upload.ID)
	if err != nil {
		return nil, fmt.Errorf("listing daily row errors: %w", err)
	}
	result.Daily = newSheetResult(upload.ID, upload.DailyStatus, upload.DailyRowCount, upload.DailyErrorCount, dailyRowErrorsToDto(dailyErrs))

	rencanaErrs, err := s.repo.ListDsrRencanaIsiRowErrors(ctx, upload.ID)
	if err != nil {
		return nil, fmt.Errorf("listing rencana isi row errors: %w", err)
	}
	result.RencanaIsi = newSheetResult(upload.ID, upload.RencanaIsiStatus, upload.RencanaIsiRowCount, upload.RencanaIsiErrorCount, rencanaRowErrorsToDto(rencanaErrs))

	return result, nil
}

// successCount derives rows-minus-errors: the schema stores only those two, so
// the third is computed rather than trusted from a column the ETL could leave
// inconsistent.
func successCount(rowCount, errorCount *int32) int32 {
	n := int32OrZero(rowCount) - int32OrZero(errorCount)
	if n < 0 {
		return 0
	}
	return n
}

func newSheetResult(uploadID int64, status string, rowCount, errorCount *int32, errs []DsrRowError) DsrSheetResult {
	return DsrSheetResult{
		Status: status, FileID: &uploadID,
		RowCount: int32OrZero(rowCount), SuccessCount: successCount(rowCount, errorCount),
		ErrorCount: int32OrZero(errorCount), Errors: errs,
	}
}

// DsrDailyRow is one row of the Daily sheet's full detail view.
type DsrDailyRow struct {
	RowNo        int32
	Section      string
	Flow         string
	LineLabel    string
	MemoNo       *string
	Denom100k    *float64
	Denom50k     *float64
	Denom20k     *float64
	Denom10k     *float64
	Denom5k      *float64
	Denom2k      *float64
	Denom1k      *float64
	LineTotalIdr *float64
	Remarks      *string
}

// DsrRencanaIsiRow is one row of the Rencana Isi sheet's full detail view.
type DsrRencanaIsiRow struct {
	RowNo                int32
	AtmTerminalID        string
	AtmLocation          *string
	DenomConfig          *string
	Fill100kIdr          *float64
	Fill50kIdr           *float64
	SplankBalance0800Idr *float64
	Remarks              *string
}

// DsrSheetDetail is the full detail view for GET /uploads/{sheet}/{file_id}.
// Only one of DailyRows/RencanaIsiRows is populated, matching which getter
// built it -- the two sheets have unrelated column shapes.
type DsrSheetDetail struct {
	FileID         int64
	Filename       string
	Status         string
	ReportDate     *time.Time
	RowCount       int32
	SuccessCount   int32
	ErrorCount     int32
	Errors         []DsrRowError
	DailyRows      []DsrDailyRow
	RencanaIsiRows []DsrRencanaIsiRow
}

// GetSaldoFile fetches one upload's Daily-sheet detail, scoped to the caller's
// vendor (FR13: a vendor may only see files scoped to their own vendor_id).
func (s *DsrService) GetSaldoFile(ctx context.Context, vendorName string, fileID int64) (*DsrSheetDetail, error) {
	upload, err := s.getUploadForVendor(ctx, vendorName, fileID)
	if err != nil {
		return nil, err
	}
	errs, err := s.repo.ListDsrDailyRowErrors(ctx, upload.ID)
	if err != nil {
		return nil, fmt.Errorf("listing daily row errors: %w", err)
	}
	rows, err := s.repo.ListDsrDailyRows(ctx, upload.ID)
	if err != nil {
		return nil, fmt.Errorf("listing daily rows: %w", err)
	}
	dailyRows, err := dailyRowsToDto(rows)
	if err != nil {
		return nil, fmt.Errorf("converting daily rows: %w", err)
	}
	detail := newSheetDetail(upload, upload.DailyStatus, upload.DailyRowCount, upload.DailyErrorCount, dailyRowErrorsToDto(errs))
	detail.DailyRows = dailyRows
	return detail, nil
}

// GetRencanaIsiFile fetches one upload's Rencana Isi-sheet detail, scoped to
// the caller's vendor.
func (s *DsrService) GetRencanaIsiFile(ctx context.Context, vendorName string, fileID int64) (*DsrSheetDetail, error) {
	upload, err := s.getUploadForVendor(ctx, vendorName, fileID)
	if err != nil {
		return nil, err
	}
	errs, err := s.repo.ListDsrRencanaIsiRowErrors(ctx, upload.ID)
	if err != nil {
		return nil, fmt.Errorf("listing rencana isi row errors: %w", err)
	}
	rows, err := s.repo.ListDsrRencanaIsiRows(ctx, upload.ID)
	if err != nil {
		return nil, fmt.Errorf("listing rencana isi rows: %w", err)
	}
	rencanaRows, err := rencanaIsiRowsToDto(rows)
	if err != nil {
		return nil, fmt.Errorf("converting rencana isi rows: %w", err)
	}
	detail := newSheetDetail(upload, upload.RencanaIsiStatus, upload.RencanaIsiRowCount, upload.RencanaIsiErrorCount, rencanaRowErrorsToDto(errs))
	detail.RencanaIsiRows = rencanaRows
	return detail, nil
}

// getUploadForVendor fetches the upload row with vendor scoping applied in SQL.
func (s *DsrService) getUploadForVendor(ctx context.Context, vendorName string, uploadID int64) (db.GetDsrUploadByIDForVendorRow, error) {
	row, err := s.repo.GetDsrUploadByIDForVendor(ctx, db.GetDsrUploadByIDForVendorParams{ID: uploadID, Vendor: vendorName})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return row, ErrDsrFileNotFound
		}
		return row, fmt.Errorf("fetching upload: %w", err)
	}
	return row, nil
}

func newSheetDetail(upload db.GetDsrUploadByIDForVendorRow, status string, rowCount, errorCount *int32, errs []DsrRowError) *DsrSheetDetail {
	return &DsrSheetDetail{
		FileID: upload.ID, Filename: upload.Filename, Status: status,
		ReportDate:   dateOrNil(upload.ReportDate),
		RowCount:     int32OrZero(rowCount),
		SuccessCount: successCount(rowCount, errorCount),
		ErrorCount:   int32OrZero(errorCount),
		Errors:       errs,
	}
}

// DsrSheetSummary is one sheet's status within a list-uploads row.
type DsrSheetSummary struct {
	FileID int64
	Status string
}

// DsrUploadListItem is one uploaded workbook in the paginated list. Both
// sheets are always present (same upload row) and share a FileID; their Status
// fields say whether each sheet actually ingested.
type DsrUploadListItem struct {
	ReportDate time.Time
	Daily      *DsrSheetSummary
	RencanaIsi *DsrSheetSummary
}

// DsrListUploadsParams holds validated request parameters for ListUploads.
type DsrListUploadsParams struct {
	VendorName string
	Page       int
	PageSize   int
	DateFrom   string
	DateTo     string
}

func (p DsrListUploadsParams) validate() error {
	if p.Page < 1 {
		return &ValidationError{Field: "page", Message: "harus lebih besar atau sama dengan 1"}
	}
	if p.PageSize < 1 || p.PageSize > 100 {
		return &ValidationError{Field: "page_size", Message: "harus antara 1 dan 100"}
	}
	if err := validateDateBound("date_from", p.DateFrom); err != nil {
		return err
	}
	if err := validateDateBound("date_to", p.DateTo); err != nil {
		return err
	}
	if p.DateFrom != "" && p.DateTo != "" && p.DateFrom > p.DateTo {
		return &ValidationError{Field: "date_from", Message: "tidak boleh lebih besar dari date_to"}
	}
	return nil
}

// DsrUploadListResult is the paginated upload list result.
type DsrUploadListResult struct {
	Data     []DsrUploadListItem
	Total    int64
	Page     int
	PageSize int
}

// ListUploads paginates over this vendor's uploaded workbooks, newest first.
// One row per upload carries both sheets' statuses, so the old per-date
// follow-up lookups are gone.
func (s *DsrService) ListUploads(ctx context.Context, params DsrListUploadsParams) (*DsrUploadListResult, error) {
	if err := params.validate(); err != nil {
		return nil, err
	}

	uploads, err := s.repo.ListDsrUploadsByVendor(ctx, db.ListDsrUploadsByVendorParams{
		Vendor: params.VendorName, DateFrom: params.DateFrom, DateTo: params.DateTo,
		Page: int32(params.Page), PageSize: int32(params.PageSize),
	})
	if err != nil {
		return nil, fmt.Errorf("listing uploads: %w", err)
	}
	total, err := s.repo.CountDsrUploadsByVendor(ctx, db.CountDsrUploadsByVendorParams{
		Vendor: params.VendorName, DateFrom: params.DateFrom, DateTo: params.DateTo,
	})
	if err != nil {
		return nil, fmt.Errorf("counting uploads: %w", err)
	}

	items := make([]DsrUploadListItem, len(uploads))
	for i, u := range uploads {
		items[i] = DsrUploadListItem{
			ReportDate: u.ReportDate.Time,
			Daily:      &DsrSheetSummary{FileID: u.ID, Status: u.DailyStatus},
			RencanaIsi: &DsrSheetSummary{FileID: u.ID, Status: u.RencanaIsiStatus},
		}
	}

	return &DsrUploadListResult{Data: items, Total: total, Page: params.Page, PageSize: params.PageSize}, nil
}

func int32OrZero(p *int32) int32 {
	if p == nil {
		return 0
	}
	return *p
}

func dateOrNil(d pgtype.Date) *time.Time {
	if !d.Valid {
		return nil
	}
	return &d.Time
}

func dailyRowErrorsToDto(rows []db.ListDsrDailyRowErrorsRow) []DsrRowError {
	out := make([]DsrRowError, len(rows))
	for i, r := range rows {
		out[i] = DsrRowError{RowNo: r.RowNo, Label: r.LineLabel}
	}
	return out
}

func rencanaRowErrorsToDto(rows []db.ListDsrRencanaIsiRowErrorsRow) []DsrRowError {
	out := make([]DsrRowError, len(rows))
	for i, r := range rows {
		out[i] = DsrRowError{RowNo: r.RowNo, Label: r.AtmTerminalID}
	}
	return out
}

func dailyRowsToDto(rows []db.ListDsrDailyRowsRow) ([]DsrDailyRow, error) {
	out := make([]DsrDailyRow, len(rows))
	for i, r := range rows {
		var err error
		row := DsrDailyRow{RowNo: r.RowNo, Section: r.Section, Flow: r.Flow, LineLabel: r.LineLabel, MemoNo: r.MemoNo, Remarks: r.Remarks}
		if row.Denom100k, err = numericToFloat64Ptr(r.Denom100kIdr); err != nil {
			return nil, err
		}
		if row.Denom50k, err = numericToFloat64Ptr(r.Denom50kIdr); err != nil {
			return nil, err
		}
		if row.Denom20k, err = numericToFloat64Ptr(r.Denom20kIdr); err != nil {
			return nil, err
		}
		if row.Denom10k, err = numericToFloat64Ptr(r.Denom10kIdr); err != nil {
			return nil, err
		}
		if row.Denom5k, err = numericToFloat64Ptr(r.Denom5kIdr); err != nil {
			return nil, err
		}
		if row.Denom2k, err = numericToFloat64Ptr(r.Denom2kIdr); err != nil {
			return nil, err
		}
		if row.Denom1k, err = numericToFloat64Ptr(r.Denom1kIdr); err != nil {
			return nil, err
		}
		if row.LineTotalIdr, err = numericToFloat64Ptr(r.LineTotalIdr); err != nil {
			return nil, err
		}
		out[i] = row
	}
	return out, nil
}

func rencanaIsiRowsToDto(rows []db.ListDsrRencanaIsiRowsRow) ([]DsrRencanaIsiRow, error) {
	out := make([]DsrRencanaIsiRow, len(rows))
	for i, r := range rows {
		var err error
		row := DsrRencanaIsiRow{RowNo: r.RowNo, AtmTerminalID: r.AtmTerminalID, AtmLocation: r.AtmLocation, DenomConfig: r.DenomConfig, Remarks: r.Remarks}
		if row.Fill100kIdr, err = numericToFloat64Ptr(r.Fill100kIdr); err != nil {
			return nil, err
		}
		if row.Fill50kIdr, err = numericToFloat64Ptr(r.Fill50kIdr); err != nil {
			return nil, err
		}
		if row.SplankBalance0800Idr, err = numericToFloat64Ptr(r.SplankBalance0800Idr); err != nil {
			return nil, err
		}
		out[i] = row
	}
	return out, nil
}
