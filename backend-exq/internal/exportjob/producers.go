package exportjob

import (
	"context"
	"fmt"
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

type producer struct {
	header  []string
	produce func(ctx context.Context, q Querier, p Params, asOf time.Time, emit func([]string) error) error
}

func (x producer) Header() []string { return x.header }

func (x producer) Produce(ctx context.Context, q Querier, p Params, asOf time.Time, emit func([]string) error) error {
	return x.produce(ctx, q, p, asOf, emit)
}

// paged runs the keyset loop: fetch(afterID) until a page is shorter than PageSize.
func paged[R any](ctx context.Context, fetch func(afterID int64) ([]R, error), idOf func(R) int64, cells func(R) []string, emit func([]string) error) error {
	var after int64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		rows, err := fetch(after)
		if err != nil {
			return fmt.Errorf("export page after id %d: %w", after, err)
		}
		for _, r := range rows {
			if err := emit(cells(r)); err != nil {
				return err
			}
			after = idOf(r)
		}
		if len(rows) < PageSize {
			return nil
		}
	}
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
	return producer{headerOf(csvexport.Vendors, false), func(ctx context.Context, q Querier, p Params, _ time.Time, emit func([]string) error) error {
		v, _ := scope(p)
		status := p.StatusFilter
		if forceAll { // Req 4.6: the vendor profile row is always exported
			status = "all"
		}
		return paged(ctx, func(a int64) ([]repository.ExportVendorsBatchRow, error) {
			return q.ExportVendorsBatch(ctx, repository.ExportVendorsBatchParams{AfterID: a, VendorID: v, Status: status, BatchSize: PageSize})
		}, func(r repository.ExportVendorsBatchRow) int64 { return r.ID },
			func(r repository.ExportVendorsBatchRow) []string {
				return csvexport.VendorCells(csvexport.VendorRow(r))
			}, emit)
	}}
}

func branchesProducer() Producer {
	return producer{headerOf(csvexport.VendorBranches, false), func(ctx context.Context, q Querier, p Params, _ time.Time, emit func([]string) error) error {
		v, _ := scope(p)
		return paged(ctx, func(a int64) ([]repository.ExportVendorBranchesBatchRow, error) {
			return q.ExportVendorBranchesBatch(ctx, repository.ExportVendorBranchesBatchParams{AfterID: a, VendorID: v, Status: p.StatusFilter, BatchSize: PageSize})
		}, func(r repository.ExportVendorBranchesBatchRow) int64 { return r.ID },
			func(r repository.ExportVendorBranchesBatchRow) []string {
				return csvexport.VendorBranchCells(csvexport.VendorBranchRow(r))
			}, emit)
	}}
}

func vaultsProducer() Producer {
	return producer{headerOf(csvexport.VendorVaults, false), func(ctx context.Context, q Querier, p Params, _ time.Time, emit func([]string) error) error {
		v, b := scope(p)
		return paged(ctx, func(a int64) ([]repository.ExportVendorVaultsBatchRow, error) {
			return q.ExportVendorVaultsBatch(ctx, repository.ExportVendorVaultsBatchParams{AfterID: a, VendorID: v, BranchID: b, Status: p.StatusFilter, BatchSize: PageSize})
		}, func(r repository.ExportVendorVaultsBatchRow) int64 { return r.ID },
			func(r repository.ExportVendorVaultsBatchRow) []string {
				return csvexport.VendorVaultCells(csvexport.VendorVaultRow(r))
			}, emit)
	}}
}

func picsProducer() Producer {
	return producer{headerOf(csvexport.VendorPICs, false), func(ctx context.Context, q Querier, p Params, _ time.Time, emit func([]string) error) error {
		v, b := scope(p)
		return paged(ctx, func(a int64) ([]repository.ExportVendorPicsBatchRow, error) {
			return q.ExportVendorPicsBatch(ctx, repository.ExportVendorPicsBatchParams{AfterID: a, VendorID: v, BranchID: b, Status: p.StatusFilter, BatchSize: PageSize})
		}, func(r repository.ExportVendorPicsBatchRow) int64 { return r.ID },
			func(r repository.ExportVendorPicsBatchRow) []string {
				return csvexport.VendorPICCells(csvexport.VendorPICRow(r))
			}, emit)
	}}
}

func atmsProducer() Producer {
	return producer{headerOf(csvexport.ATMs, false), func(ctx context.Context, q Querier, p Params, _ time.Time, emit func([]string) error) error {
		return paged(ctx, func(a int64) ([]repository.ExportATMsBatchRow, error) {
			return q.ExportATMsBatch(ctx, repository.ExportATMsBatchParams{AfterID: a, Status: p.StatusFilter, BatchSize: PageSize})
		}, func(r repository.ExportATMsBatchRow) int64 { return r.ID },
			func(r repository.ExportATMsBatchRow) []string { return csvexport.ATMCells(csvexport.ATMRow(r)) }, emit)
	}}
}

func atmAssignmentsProducer() Producer {
	return producer{headerOf(csvexport.ATMAssignments, false), func(ctx context.Context, q Querier, p Params, _ time.Time, emit func([]string) error) error {
		return paged(ctx, func(a int64) ([]repository.ExportATMAssignmentsBatchRow, error) {
			return q.ExportATMAssignmentsBatch(ctx, repository.ExportATMAssignmentsBatchParams{AfterID: a, Status: p.StatusFilter, BatchSize: PageSize})
		}, func(r repository.ExportATMAssignmentsBatchRow) int64 { return r.ID },
			func(r repository.ExportATMAssignmentsBatchRow) []string {
				return csvexport.ATMAssignmentCells(csvexport.ATMAssignmentRow(r))
			}, emit)
	}}
}

func dsrMapsProducer() Producer {
	return producer{headerOf(csvexport.DSRLocationMaps, true), func(ctx context.Context, q Querier, p Params, _ time.Time, emit func([]string) error) error {
		v, b := scope(p)
		return paged(ctx, func(a int64) ([]repository.ExportDsrLocationMapsBatchRow, error) {
			return q.ExportDsrLocationMapsBatch(ctx, repository.ExportDsrLocationMapsBatchParams{AfterID: a, VendorID: v, BranchID: b, Status: p.StatusFilter, BatchSize: PageSize})
		}, func(r repository.ExportDsrLocationMapsBatchRow) int64 { return r.ID },
			func(r repository.ExportDsrLocationMapsBatchRow) []string {
				return []string{strconv.FormatInt(r.ID, 10), r.VendorCode, r.BranchCode, r.VaultCode, r.DsrLocation, boolStr(r.IsActive)}
			}, emit)
	}}
}

func packagePricesProducer() Producer {
	return producer{headerOf(csvexport.PackagePrices, true), func(ctx context.Context, q Querier, p Params, asOf time.Time, emit func([]string) error) error {
		if p.VendorID == nil {
			return fmt.Errorf("%w: package-prices needs vendor_id", ErrValidation)
		}
		level := priceLevelAll
		if p.PriceLevel != nil {
			level = *p.PriceLevel
		}
		return paged(ctx, func(a int64) ([]repository.ExportPackagePricesBatchRow, error) {
			return q.ExportPackagePricesBatch(ctx, repository.ExportPackagePricesBatchParams{
				AfterID: a, VendorID: *p.VendorID, BranchID: p.BranchID, PriceLevel: level,
				Status: p.StatusFilter, AsOf: wibDate(asOf), BatchSize: PageSize,
			})
		}, func(r repository.ExportPackagePricesBatchRow) int64 { return r.ID },
			func(r repository.ExportPackagePricesBatchRow) []string {
				return []string{strconv.FormatInt(r.ID, 10), r.VendorCode, r.PriceLevel, r.BranchCode, r.TerminalID, r.Package, r.PackageCode,
					r.MachineGroup, r.PriceClass, r.TierMin, r.TierMax, r.BasePrice, r.Currency, r.SlaNote, r.EffectiveStartDate, r.EffectiveEndDate}
			}, emit)
	}}
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
