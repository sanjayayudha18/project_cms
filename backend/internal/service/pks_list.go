package service

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/pkg/reports"
)

const (
	pksListDefaultPageSize = 20
	pksListMaxPageSize     = 100
	pksStatusPending       = "menunggu_persetujuan"
	pksOpCreate            = "create"
)

// PksListRepo is the read surface of PksListService (replica). *repository.PksListRepository satisfies it.
type PksListRepo interface {
	List(ctx context.Context, arg db.ListPksAdminParams) ([]db.ListPksAdminRow, error)
	Count(ctx context.Context, arg db.CountPksAdminParams) (int64, error)
	Pending(ctx context.Context, arg db.ListPendingPksChangesParams) ([]db.ListPendingPksChangesRow, error)
}

// PksTypeFinder checks a PKS type code (any status; history may use a disabled type).
// *repository.PksTypeRepository satisfies it.
type PksTypeFinder interface {
	FindByCode(ctx context.Context, code string) (*db.VendorPksType, error)
}

// VendorCodeFinder checks a vendor code. *repository.VendorAdminRepository satisfies it.
type VendorCodeFinder interface {
	FindByCode(ctx context.Context, code string) (*int64, error)
}

// PksListParams is the request of PksListService.List; Filter comes from reports.ParsePksListFilter.
// Page/PageSize out of range fall back to 1 / 20 (max 100).
type PksListParams struct {
	Status, PksType, Vendor, Q string
	Page, PageSize             int
}

// PksPending marks a pending master-data change for the same (vendor, type).
type PksPending struct {
	Op       string `json:"op"`
	ChangeID int64  `json:"change_id"`
}

// PksListItem is one row of Daftar PKS. ID is nil for a pending create (no vendor_pks row yet).
type PksListItem struct {
	ID            *int64      `json:"id"`
	VendorID      int64       `json:"vendor_id"`
	VendorCode    string      `json:"vendor_code"`
	VendorName    string      `json:"vendor_name"`
	PksType       string      `json:"pks_type"`
	PksTypeName   string      `json:"pks_type_name"`
	WodNumber     string      `json:"wod_number"`
	PksName       string      `json:"pks_name"`
	EffectiveDate string      `json:"effective_date"`
	ExpiryDate    string      `json:"expiry_date"`
	RenewalDays   int         `json:"renewal_days"`
	RemainingDays int         `json:"remaining_days"`
	Status        string      `json:"status"`
	Pending       *PksPending `json:"pending"`
}

// PksListPage is the response of PksListService.List.
type PksListPage struct {
	Items    []PksListItem `json:"items"`
	Total    int64         `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
}

// PksListService backs GET /admin/pks (spec vendor-pks-list Req 4). Read only, replica.
type PksListService struct {
	repo    PksListRepo
	types   PksTypeFinder
	vendors VendorCodeFinder
	now     func() time.Time
}

// NewPksListService creates the service.
func NewPksListService(repo PksListRepo, types PksTypeFinder, vendors VendorCodeFinder) *PksListService {
	return &PksListService{repo: repo, types: types, vendors: vendors, now: time.Now}
}

// pendingCreatePayload is the subset of the vendor_pks create payload the list shows.
type pendingCreatePayload struct {
	PksName       string `json:"pks_name"`
	EffectiveDate string `json:"effective_date"`
	ExpiryDate    string `json:"expiry_date"`
	WodNumber     string `json:"wod_number"`
	RenewalDays   *int   `json:"renewal_days"` // absent on changes made before migration 044 → default
}

type pksKey struct{ vendorID, typeID int64 }

// List returns one page of PKS rows (history included) for role ∈ PksViewerRoles.
func (s *PksListService) List(ctx context.Context, role string, p PksListParams) (*PksListPage, error) {
	if !slices.Contains(reports.PksViewerRoles, role) {
		return nil, ErrMasterDataForbidden
	}
	f, err := reports.ParsePksListFilter(p.Status, p.PksType, p.Vendor, p.Q)
	if err != nil {
		return nil, &ValidationError{Field: "filter", Message: err.Error()}
	}
	if f.PksType != "" {
		t, err := s.types.FindByCode(ctx, f.PksType)
		if err != nil {
			return nil, fmt.Errorf("find pks type: %w", err)
		}
		if t == nil {
			return nil, &ValidationError{Field: "pks_type", Message: "Type PKS tidak ditemukan"}
		}
	}
	if f.Vendor != "" {
		id, err := s.vendors.FindByCode(ctx, f.Vendor)
		if err != nil {
			return nil, fmt.Errorf("find vendor: %w", err)
		}
		if id == nil {
			return nil, &ValidationError{Field: "vendor", Message: "Vendor tidak ditemukan"}
		}
	}

	page, size := p.Page, p.PageSize
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = pksListDefaultPageSize
	}
	size = min(size, pksListMaxPageSize)

	today := reports.TodayWIB(s.now())
	todayDate := pgtype.Date{Time: today, Valid: true}
	typeArg, vendorArg, qArg := strPtr(f.PksType), strPtr(f.Vendor), strPtr(f.Q)

	rows, err := s.repo.List(ctx, db.ListPksAdminParams{
		Today: todayDate, Status: f.Status, PksType: typeArg, Vendor: vendorArg, Q: qArg,
		PageOffset: int64((page - 1) * size), PageLimit: int64(size),
	})
	if err != nil {
		return nil, fmt.Errorf("list pks: %w", err)
	}
	total, err := s.repo.Count(ctx, db.CountPksAdminParams{
		Status: f.Status, Today: todayDate, PksType: typeArg, Vendor: vendorArg, Q: qArg,
	})
	if err != nil {
		return nil, fmt.Errorf("count pks: %w", err)
	}
	pending, err := s.repo.Pending(ctx, db.ListPendingPksChangesParams{PksType: typeArg, Vendor: vendorArg})
	if err != nil {
		return nil, fmt.Errorf("list pending pks: %w", err)
	}

	marks := make(map[pksKey]*PksPending, len(pending))
	for _, c := range pending {
		marks[pksKey{c.VendorID, c.PksTypeID}] = &PksPending{Op: c.Op, ChangeID: c.ID}
	}

	var items []PksListItem
	// Pending creates have no vendor_pks row, so they sit outside DB paging: shown on page 1 and added to total.
	if f.Status == "terkini" || f.Status == "semua" {
		creates := pendingCreateItems(pending, f.Q, today)
		total += int64(len(creates))
		if page == 1 {
			items = append(items, creates...)
		}
	}
	for _, r := range rows {
		items = append(items, PksListItem{
			ID: &r.ID, VendorID: r.VendorID, VendorCode: r.VendorCode, VendorName: r.VendorName,
			PksType: r.PksTypeCode, PksTypeName: r.PksTypeName, WodNumber: r.WodNumber, PksName: r.PksName,
			EffectiveDate: r.EffectiveDate.Time.Format(vendorPksDateLayout),
			ExpiryDate:    r.ExpiryDate.Time.Format(vendorPksDateLayout),
			RenewalDays:   int(r.RenewalDays), RemainingDays: int(r.RemainingDays), Status: r.Status,
			Pending: marks[pksKey{r.VendorID, r.PksTypeID}],
		})
	}
	if items == nil {
		items = []PksListItem{}
	}
	return &PksListPage{Items: items, Total: total, Page: page, PageSize: size}, nil
}

// pendingCreateItems turns pending op=create changes into menunggu_persetujuan rows, applying the q filter
// (the SQL filters vendor/type codes already). An unparsable payload is skipped, not fatal for the screen.
func pendingCreateItems(pending []db.ListPendingPksChangesRow, q string, today time.Time) []PksListItem {
	var out []PksListItem
	lq := strings.ToLower(q)
	for _, c := range pending {
		if c.Op != pksOpCreate {
			continue
		}
		var pl pendingCreatePayload
		if err := json.Unmarshal(c.Payload, &pl); err != nil {
			continue
		}
		if lq != "" && !strings.Contains(strings.ToLower(pl.PksName), lq) &&
			!strings.Contains(strings.ToLower(pl.WodNumber), lq) {
			continue
		}
		renewal := reports.RenewalDaysDefault
		if pl.RenewalDays != nil {
			renewal = *pl.RenewalDays
		}
		remaining := 0
		if exp, err := time.Parse(vendorPksDateLayout, pl.ExpiryDate); err == nil {
			remaining = int(exp.Sub(today).Hours() / 24)
		}
		out = append(out, PksListItem{
			VendorID: c.VendorID, VendorCode: c.VendorCode, VendorName: c.VendorName,
			PksType: c.PksTypeCode, PksTypeName: c.PksTypeName, WodNumber: pl.WodNumber, PksName: pl.PksName,
			EffectiveDate: pl.EffectiveDate, ExpiryDate: pl.ExpiryDate, RenewalDays: renewal,
			RemainingDays: remaining, Status: pksStatusPending,
			Pending: &PksPending{Op: c.Op, ChangeID: c.ID},
		})
	}
	return out
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
