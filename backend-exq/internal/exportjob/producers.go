package exportjob

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend-exq/internal/repository"
	"github.com/cimb-niaga/cms/pkg/csvexport"
)

// Querier is the subset of *repository.Queries the producers call.
type Querier interface {
	ReportQuerier
	ExportVendorsBatch(ctx context.Context, arg repository.ExportVendorsBatchParams) ([]repository.ExportVendorsBatchRow, error)
	ExportVendorBranchesBatch(ctx context.Context, arg repository.ExportVendorBranchesBatchParams) ([]repository.ExportVendorBranchesBatchRow, error)
	ExportVendorVaultsBatch(ctx context.Context, arg repository.ExportVendorVaultsBatchParams) ([]repository.ExportVendorVaultsBatchRow, error)
	ExportVendorPicsBatch(ctx context.Context, arg repository.ExportVendorPicsBatchParams) ([]repository.ExportVendorPicsBatchRow, error)
	ExportATMsBatch(ctx context.Context, arg repository.ExportATMsBatchParams) ([]repository.ExportATMsBatchRow, error)
	ExportATMAssignmentsBatch(ctx context.Context, arg repository.ExportATMAssignmentsBatchParams) ([]repository.ExportATMAssignmentsBatchRow, error)
	ExportDsrLocationMapsBatch(ctx context.Context, arg repository.ExportDsrLocationMapsBatchParams) ([]repository.ExportDsrLocationMapsBatchRow, error)
	ExportPackagePricesBatch(ctx context.Context, arg repository.ExportPackagePricesBatchParams) ([]repository.ExportPackagePricesBatchRow, error)
	CountVendors(ctx context.Context, arg repository.CountVendorsParams) (int64, error)
	CountVendorBranches(ctx context.Context, arg repository.CountVendorBranchesParams) (int64, error)
	CountVendorVaults(ctx context.Context, arg repository.CountVendorVaultsParams) (int64, error)
	CountVendorPics(ctx context.Context, arg repository.CountVendorPicsParams) (int64, error)
	CountATMs(ctx context.Context, status string) (int64, error)
	CountATMAssignments(ctx context.Context, status string) (int64, error)
	CountDsrLocationMaps(ctx context.Context, arg repository.CountDsrLocationMapsParams) (int64, error)
	CountPackagePrices(ctx context.Context, arg repository.CountPackagePricesParams) (int64, error)
}

var _ Querier = (*repository.Queries)(nil)

// Producer streams the CSV rows of one entity. Header and cells come only from
// pkg/csvexport so an export reads back through the importer unchanged.
type Producer interface {
	Header() []string
	// Produce emits every matching row in id order, one keyset page at a time.
	// No rows -> emit is never called (the caller writes a header-only file).
	Produce(ctx context.Context, q Querier, p Params, asOf time.Time, emit func([]string) error) error
}

// PreviewProducer is a master-data producer that can also serve one preview page.
type PreviewProducer interface {
	Producer
	// Page returns up to limit rows with id > afterID in id order, raw cells (no CSVSafe).
	Page(ctx context.Context, q Querier, p Params, asOf time.Time, afterID int64, limit int) (rows [][]string, lastID int64, err error)
	// Count returns how many rows Produce would emit for p in the same snapshot.
	Count(ctx context.Context, q Querier, p Params, asOf time.Time) (int64, error)
}

// tableProducer is one keyset-paged master-data table. Produce (file) and Page
// (preview) share fetch and cells, so a preview cannot differ from the file.
type tableProducer[R any] struct {
	header []string
	fetch  func(ctx context.Context, q Querier, p Params, asOf time.Time, afterID int64, limit int32) ([]R, error)
	id     func(R) int64
	cells  func(R) []string
	count  func(ctx context.Context, q Querier, p Params, asOf time.Time) (int64, error)
}

func (t tableProducer[R]) Header() []string { return t.header }

func (t tableProducer[R]) Count(ctx context.Context, q Querier, p Params, asOf time.Time) (int64, error) {
	return t.count(ctx, q, p, asOf)
}

func (t tableProducer[R]) Page(ctx context.Context, q Querier, p Params, asOf time.Time, afterID int64, limit int) ([][]string, int64, error) {
	// The batch queries take an int32 LIMIT; clamp once here so no producer converts.
	lim := int32(max(0, min(limit, math.MaxInt32)))
	rows, err := t.fetch(ctx, q, p, asOf, afterID, lim)
	if err != nil {
		return nil, afterID, fmt.Errorf("export page after id %d: %w", afterID, err)
	}
	out := make([][]string, 0, len(rows))
	last := afterID
	for _, r := range rows {
		out = append(out, t.cells(r))
		last = t.id(r)
	}
	return out, last, nil
}

// Produce loops Page until a page is shorter than PageSize.
func (t tableProducer[R]) Produce(ctx context.Context, q Querier, p Params, asOf time.Time, emit func([]string) error) error {
	var after int64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		rows, last, err := t.Page(ctx, q, p, asOf, after, PageSize)
		if err != nil {
			return err
		}
		for _, r := range rows {
			if err := emit(r); err != nil {
				return err
			}
		}
		after = last
		if len(rows) < PageSize {
			return nil
		}
	}
}

// producer wraps a free-form produce func (report producers).
type producer struct {
	header  []string
	produce func(ctx context.Context, q Querier, p Params, asOf time.Time, emit func([]string) error) error
}

func (x producer) Header() []string { return x.header }

func (x producer) Produce(ctx context.Context, q Querier, p Params, asOf time.Time, emit func([]string) error) error {
	return x.produce(ctx, q, p, asOf, emit)
}

func headerOf(name string, exportOnly bool) []string {
	look := csvexport.Lookup
	if exportOnly {
		look = csvexport.LookupExportOnly
	}
	e, ok := look(name)
	if !ok {
		panic("exportjob: csvexport entity missing: " + name) // programming error, caught by tests
	}
	return e.Header
}

// wibDate is the WIB calendar date of t as a pgtype.Date (Req 5.8).
func wibDate(t time.Time) pgtype.Date {
	y, m, d := t.In(wib).Date()
	return pgtype.Date{Time: time.Date(y, m, d, 0, 0, 0, 0, time.UTC), Valid: true}
}

func boolStr(b bool) string { return strconv.FormatBool(b) }

// scope picks the vendor/branch ids a producer passes on; global kinds pass nil.
func scope(p Params) (vendorID, branchID *int64) { return p.VendorID, p.BranchID }

func vendorsProducer(forceAll bool) Producer {
	status := func(p Params) string {
		if forceAll { // Req 4.6: the vendor profile row is always exported
			return "all"
		}
		return p.StatusFilter
	}
	return tableProducer[repository.ExportVendorsBatchRow]{
		header: headerOf(csvexport.Vendors, false),
		fetch: func(ctx context.Context, q Querier, p Params, _ time.Time, after int64, limit int32) ([]repository.ExportVendorsBatchRow, error) {
			v, _ := scope(p)
			return q.ExportVendorsBatch(ctx, repository.ExportVendorsBatchParams{AfterID: after, VendorID: v, Status: status(p), BatchSize: limit})
		},
		id: func(r repository.ExportVendorsBatchRow) int64 { return r.ID },
		cells: func(r repository.ExportVendorsBatchRow) []string {
			return csvexport.VendorCells(csvexport.VendorRow(r))
		},
		count: func(ctx context.Context, q Querier, p Params, _ time.Time) (int64, error) {
			v, _ := scope(p)
			return q.CountVendors(ctx, repository.CountVendorsParams{VendorID: v, Status: status(p)})
		},
	}
}

func branchesProducer() Producer {
	return tableProducer[repository.ExportVendorBranchesBatchRow]{
		header: headerOf(csvexport.VendorBranches, false),
		fetch: func(ctx context.Context, q Querier, p Params, _ time.Time, after int64, limit int32) ([]repository.ExportVendorBranchesBatchRow, error) {
			v, _ := scope(p)
			return q.ExportVendorBranchesBatch(ctx, repository.ExportVendorBranchesBatchParams{AfterID: after, VendorID: v, Status: p.StatusFilter, BatchSize: limit})
		},
		id: func(r repository.ExportVendorBranchesBatchRow) int64 { return r.ID },
		cells: func(r repository.ExportVendorBranchesBatchRow) []string {
			return csvexport.VendorBranchCells(csvexport.VendorBranchRow(r))
		},
		count: func(ctx context.Context, q Querier, p Params, _ time.Time) (int64, error) {
			v, _ := scope(p)
			return q.CountVendorBranches(ctx, repository.CountVendorBranchesParams{VendorID: v, Status: p.StatusFilter})
		},
	}
}

func vaultsProducer() Producer {
	return tableProducer[repository.ExportVendorVaultsBatchRow]{
		header: headerOf(csvexport.VendorVaults, false),
		fetch: func(ctx context.Context, q Querier, p Params, _ time.Time, after int64, limit int32) ([]repository.ExportVendorVaultsBatchRow, error) {
			v, b := scope(p)
			return q.ExportVendorVaultsBatch(ctx, repository.ExportVendorVaultsBatchParams{AfterID: after, VendorID: v, BranchID: b, Status: p.StatusFilter, BatchSize: limit})
		},
		id: func(r repository.ExportVendorVaultsBatchRow) int64 { return r.ID },
		cells: func(r repository.ExportVendorVaultsBatchRow) []string {
			return csvexport.VendorVaultCells(csvexport.VendorVaultRow(r))
		},
		count: func(ctx context.Context, q Querier, p Params, _ time.Time) (int64, error) {
			v, b := scope(p)
			return q.CountVendorVaults(ctx, repository.CountVendorVaultsParams{VendorID: v, BranchID: b, Status: p.StatusFilter})
		},
	}
}

func picsProducer() Producer {
	return tableProducer[repository.ExportVendorPicsBatchRow]{
		header: headerOf(csvexport.VendorPICs, false),
		fetch: func(ctx context.Context, q Querier, p Params, _ time.Time, after int64, limit int32) ([]repository.ExportVendorPicsBatchRow, error) {
			v, b := scope(p)
			return q.ExportVendorPicsBatch(ctx, repository.ExportVendorPicsBatchParams{AfterID: after, VendorID: v, BranchID: b, Status: p.StatusFilter, BatchSize: limit})
		},
		id: func(r repository.ExportVendorPicsBatchRow) int64 { return r.ID },
		cells: func(r repository.ExportVendorPicsBatchRow) []string {
			return csvexport.VendorPICCells(csvexport.VendorPICRow(r))
		},
		count: func(ctx context.Context, q Querier, p Params, _ time.Time) (int64, error) {
			v, b := scope(p)
			return q.CountVendorPics(ctx, repository.CountVendorPicsParams{VendorID: v, BranchID: b, Status: p.StatusFilter})
		},
	}
}

func atmsProducer() Producer {
	return tableProducer[repository.ExportATMsBatchRow]{
		header: headerOf(csvexport.ATMs, false),
		fetch: func(ctx context.Context, q Querier, p Params, _ time.Time, after int64, limit int32) ([]repository.ExportATMsBatchRow, error) {
			return q.ExportATMsBatch(ctx, repository.ExportATMsBatchParams{AfterID: after, Status: p.StatusFilter, BatchSize: limit})
		},
		id:    func(r repository.ExportATMsBatchRow) int64 { return r.ID },
		cells: func(r repository.ExportATMsBatchRow) []string { return csvexport.ATMCells(csvexport.ATMRow(r)) },
		count: func(ctx context.Context, q Querier, p Params, _ time.Time) (int64, error) {
			return q.CountATMs(ctx, p.StatusFilter)
		},
	}
}

func atmAssignmentsProducer() Producer {
	return tableProducer[repository.ExportATMAssignmentsBatchRow]{
		header: headerOf(csvexport.ATMAssignments, false),
		fetch: func(ctx context.Context, q Querier, p Params, _ time.Time, after int64, limit int32) ([]repository.ExportATMAssignmentsBatchRow, error) {
			return q.ExportATMAssignmentsBatch(ctx, repository.ExportATMAssignmentsBatchParams{AfterID: after, Status: p.StatusFilter, BatchSize: limit})
		},
		id: func(r repository.ExportATMAssignmentsBatchRow) int64 { return r.ID },
		cells: func(r repository.ExportATMAssignmentsBatchRow) []string {
			return csvexport.ATMAssignmentCells(csvexport.ATMAssignmentRow(r))
		},
		count: func(ctx context.Context, q Querier, p Params, _ time.Time) (int64, error) {
			return q.CountATMAssignments(ctx, p.StatusFilter)
		},
	}
}

func dsrMapsProducer() Producer {
	return tableProducer[repository.ExportDsrLocationMapsBatchRow]{
		header: headerOf(csvexport.DSRLocationMaps, true),
		fetch: func(ctx context.Context, q Querier, p Params, _ time.Time, after int64, limit int32) ([]repository.ExportDsrLocationMapsBatchRow, error) {
			v, b := scope(p)
			return q.ExportDsrLocationMapsBatch(ctx, repository.ExportDsrLocationMapsBatchParams{AfterID: after, VendorID: v, BranchID: b, Status: p.StatusFilter, BatchSize: limit})
		},
		id: func(r repository.ExportDsrLocationMapsBatchRow) int64 { return r.ID },
		cells: func(r repository.ExportDsrLocationMapsBatchRow) []string {
			return []string{strconv.FormatInt(r.ID, 10), r.VendorCode, r.BranchCode, r.VaultCode, r.DsrLocation, boolStr(r.IsActive)}
		},
		count: func(ctx context.Context, q Querier, p Params, _ time.Time) (int64, error) {
			v, b := scope(p)
			return q.CountDsrLocationMaps(ctx, repository.CountDsrLocationMapsParams{VendorID: v, BranchID: b, Status: p.StatusFilter})
		},
	}
}

// priceLevelOf is the price level filter ("all" when unset); vendor_id is mandatory.
func priceLevelOf(p Params) (string, error) {
	if p.VendorID == nil {
		return "", fmt.Errorf("%w: package-prices needs vendor_id", ErrValidation)
	}
	if p.PriceLevel != nil {
		return *p.PriceLevel, nil
	}
	return priceLevelAll, nil
}

func packagePricesProducer() Producer {
	return tableProducer[repository.ExportPackagePricesBatchRow]{
		header: headerOf(csvexport.PackagePrices, true),
		fetch: func(ctx context.Context, q Querier, p Params, asOf time.Time, after int64, limit int32) ([]repository.ExportPackagePricesBatchRow, error) {
			level, err := priceLevelOf(p)
			if err != nil {
				return nil, err
			}
			return q.ExportPackagePricesBatch(ctx, repository.ExportPackagePricesBatchParams{
				AfterID: after, VendorID: *p.VendorID, BranchID: p.BranchID, PriceLevel: level,
				Status: p.StatusFilter, AsOf: wibDate(asOf), BatchSize: limit,
			})
		},
		id: func(r repository.ExportPackagePricesBatchRow) int64 { return r.ID },
		cells: func(r repository.ExportPackagePricesBatchRow) []string {
			return []string{strconv.FormatInt(r.ID, 10), r.VendorCode, r.PriceLevel, r.BranchCode, r.TerminalID, r.Package, r.PackageCode,
				r.MachineGroup, r.PriceClass, r.TierMin, r.TierMax, r.BasePrice, r.Currency, r.SlaNote, r.EffectiveStartDate, r.EffectiveEndDate}
		},
		count: func(ctx context.Context, q Querier, p Params, asOf time.Time) (int64, error) {
			level, err := priceLevelOf(p)
			if err != nil {
				return 0, err
			}
			return q.CountPackagePrices(ctx, repository.CountPackagePricesParams{
				VendorID: *p.VendorID, BranchID: p.BranchID, PriceLevel: level, Status: p.StatusFilter, AsOf: wibDate(asOf),
			})
		},
	}
}

// DefaultProducers is NewProducers without a DSR start date (DSR report files are header only).
func DefaultProducers() map[Kind]map[string]Producer { return NewProducers(nil) }

// NewProducers maps kind -> entity -> producer (design "entity table per Kind"), master data and
// reports. dsrStart is DSR_LATE_START_DATE; nil = DSR report files are header only.
func NewProducers(dsrStart *time.Time) map[Kind]map[string]Producer {
	m := map[Kind]map[string]Producer{
		KindVendor: {
			"vendor":            vendorsProducer(true),
			"branches":          branchesProducer(),
			"vaults":            vaultsProducer(),
			"pics":              picsProducer(),
			"dsr-location-maps": dsrMapsProducer(),
		},
		// Cabang scope: PIC branch only, never vendor-wide PICs (D3, enforced in SQL).
		KindCabang: {
			"vaults":            vaultsProducer(),
			"pics":              picsProducer(),
			"dsr-location-maps": dsrMapsProducer(),
		},
		KindHargaPaket: {"package-prices": packagePricesProducer()},
		KindMasterDataGlobal: {
			csvexport.Vendors:        vendorsProducer(false),
			csvexport.VendorBranches: branchesProducer(),
			csvexport.VendorVaults:   vaultsProducer(),
			csvexport.VendorPICs:     picsProducer(),
			csvexport.ATMs:           atmsProducer(),
			csvexport.ATMAssignments: atmAssignmentsProducer(),
		},
	}
	for k, byEntity := range reportProducers(dsrStart) {
		m[k] = byEntity
	}
	return m
}
