package exportjob

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend-exq/internal/repository"
	"github.com/cimb-niaga/cms/pkg/reports"
)

// errNoReportParams: a report job whose report_params cannot be read must fail, never export unfiltered.
var errNoReportParams = errors.New("report job has no readable report_params")

// ReportQuerier is the subset of *repository.Queries the report producers call (shared SQL of backend/).
type ReportQuerier interface {
	ListDsrLateDaily(ctx context.Context, arg repository.ListDsrLateDailyParams) ([]repository.ListDsrLateDailyRow, error)
	ListDsrLateSummary(ctx context.Context, arg repository.ListDsrLateSummaryParams) ([]repository.ListDsrLateSummaryRow, error)
	ListReplenishClassifications(ctx context.Context, arg repository.ListReplenishClassificationsParams) ([]repository.ListReplenishClassificationsRow, error)
	ListReplenishClassificationAmounts(ctx context.Context, ids []int64) ([]repository.ReplenishClassificationAmount, error)
	ReportReplenishTrips(ctx context.Context, arg repository.ReportReplenishTripsParams) ([]repository.ReportReplenishTripsRow, error)
}

var _ ReportQuerier = (*repository.Queries)(nil)

type reportFn func(ctx context.Context, q Querier, r ReportParams, asOf time.Time, emit func([]string) error) error

// reportProducer fails a job without report_params before running fn.
func reportProducer(kind Kind, fn reportFn) Producer {
	return producer{reports.Header(string(kind)), func(ctx context.Context, q Querier, p Params, asOf time.Time, emit func([]string) error) error {
		if p.Report == nil {
			return fmt.Errorf("%w: %s", errNoReportParams, kind)
		}
		return fn(ctx, q, *p.Report, asOf, emit)
	}}
}

// reportProducers maps the four report kinds (entity = legacy `report` value). dsrStart nil
// (DSR_LATE_START_DATE unset) = DSR files are header only (Req 1.5).
func reportProducers(dsrStart *time.Time) map[Kind]map[string]Producer {
	return map[Kind]map[string]Producer{
		KindDsrLateDaily:                {"daily": reportProducer(KindDsrLateDaily, dsrDailyFn(dsrStart))},
		KindDsrLateSummary:              {"summary": reportProducer(KindDsrLateSummary, dsrSummaryFn(dsrStart))},
		KindReplenishmentAmountMismatch: {"amount_mismatch": reportProducer(KindReplenishmentAmountMismatch, amountMismatchFn)},
		KindReplenishmentTrips:          {"trips": reportProducer(KindReplenishmentTrips, tripsFn)},
	}
}

func parseDay(s *string, layout string) (pgtype.Date, error) {
	if s == nil {
		return pgtype.Date{}, fmt.Errorf("%w: missing date", errNoReportParams)
	}
	t, err := time.Parse(layout, *s)
	if err != nil {
		return pgtype.Date{}, fmt.Errorf("%w: %w", errNoReportParams, err)
	}
	return pgtype.Date{Time: t, Valid: true}, nil
}

func datePtr(d pgtype.Date) *time.Time {
	if !d.Valid {
		return nil
	}
	t := d.Time
	return &t
}

func tsPtrIfValid(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := ts.Time.In(reports.WIB)
	return &t
}

// dsrWindow is the shared query window; ok=false means no start date: header only.
func dsrWindow(r ReportParams, start *time.Time, asOf time.Time) (month, startDate pgtype.Date, at pgtype.Timestamptz, ok bool, err error) {
	if start == nil {
		return
	}
	if month, err = parseDay(r.Month, "2006-01"); err != nil {
		return
	}
	return month, wibDate(*start), pgtype.Timestamptz{Time: asOf, Valid: true}, true, nil
}

func dsrDailyFn(start *time.Time) reportFn {
	return func(ctx context.Context, q Querier, r ReportParams, asOf time.Time, emit func([]string) error) error {
		month, startDate, at, ok, err := dsrWindow(r, start, asOf)
		if err != nil || !ok {
			return err
		}
		// ponytail: OFFSET paging; fine because the whole job reads one REPEATABLE READ snapshot.
		for offset := int64(0); ; offset += PageSize {
			if err := ctx.Err(); err != nil {
				return err
			}
			limit := int64(PageSize)
			rows, err := q.ListDsrLateDaily(ctx, repository.ListDsrLateDailyParams{
				VendorID: r.VendorID, VendorBranchID: r.VendorBranchID, Status: r.Status,
				PageOffset: offset, PageLimit: &limit, MonthStart: month, StartDate: startDate, AsOf: at,
			})
			if err != nil {
				return fmt.Errorf("list dsr late daily offset %d: %w", offset, err)
			}
			for _, x := range rows {
				if err := emit(reports.DsrLateDailyCells(reports.DsrLateDailyRow{
					ReportDate: x.ReportDate.Time.Format("2006-01-02"), VendorCode: x.VendorCode, VendorName: x.VendorName,
					BranchCode: x.BranchCode, BranchName: x.BranchName, RegionCode: x.RegionCode, Deadline: x.Deadline.Time,
					FirstReceivedAt: tsPtrIfValid(x.FirstReceivedAt), Status: x.Status, LateMinutes: x.LateMinutes,
					ReceiptChecksum: x.ReceiptChecksum, MatchedLabels: x.MatchedLocations,
				})); err != nil {
					return err
				}
			}
			if len(rows) < PageSize {
				return nil
			}
		}
	}
}

func dsrSummaryFn(start *time.Time) reportFn {
	return func(ctx context.Context, q Querier, r ReportParams, asOf time.Time, emit func([]string) error) error {
		month, startDate, at, ok, err := dsrWindow(r, start, asOf)
		if err != nil || !ok {
			return err
		}
		rows, err := q.ListDsrLateSummary(ctx, repository.ListDsrLateSummaryParams{
			VendorID: r.VendorID, VendorBranchID: r.VendorBranchID, MonthStart: month, StartDate: startDate, AsOf: at,
		})
		if err != nil {
			return fmt.Errorf("list dsr late summary: %w", err)
		}
		for _, x := range rows {
			if err := emit(reports.DsrLateSummaryCells(reports.DsrLateSummaryRow{
				VendorCode: x.VendorCode, VendorName: x.VendorName, BranchCode: x.BranchCode, BranchName: x.BranchName,
				RegionCode: x.RegionCode, ObligedDays: x.ObligedDays, OkDays: x.OkDays, LateDays: x.LateDays,
				NotSentDays: x.NotSentDays, PendingDays: x.PendingDays, LateMinutesTotal: x.LateMinutesTotal,
			})); err != nil {
				return err
			}
		}
		return nil
	}
}

func replenishRange(r ReportParams) (from, to pgtype.Date, err error) {
	if from, err = parseDay(r.DateFrom, "2006-01-02"); err != nil {
		return
	}
	to, err = parseDay(r.DateTo, "2006-01-02")
	return
}

// amountMismatchFn pages classifications (only_diff + final_only, like the legacy report) and stops at
// ReplenishmentExportLimit classifications, as the legacy single query did; a row is one line per denom.
func amountMismatchFn(ctx context.Context, q Querier, r ReportParams, _ time.Time, emit func([]string) error) error {
	from, to, err := replenishRange(r)
	if err != nil {
		return err
	}
	for offset := int64(0); offset < reports.ReplenishmentExportLimit; offset += PageSize {
		if err := ctx.Err(); err != nil {
			return err
		}
		limit := min(int64(PageSize), reports.ReplenishmentExportLimit-offset)
		rows, err := q.ListReplenishClassifications(ctx, repository.ListReplenishClassificationsParams{
			DateFrom: from, DateTo: to, VendorID: r.VendorID, RegionCode: r.RegionCode, Classification: r.Classification,
			OnlyDiff: true, FinalOnly: true, PageOffset: offset, PageLimit: limit,
		})
		if err != nil {
			return fmt.Errorf("list classifications offset %d: %w", offset, err)
		}
		if err := emitMismatchPage(ctx, q, rows, emit); err != nil {
			return err
		}
		if int64(len(rows)) < limit {
			return nil
		}
	}
	return nil
}

func emitMismatchPage(ctx context.Context, q Querier, rows []repository.ListReplenishClassificationsRow, emit func([]string) error) error {
	if len(rows) == 0 {
		return nil
	}
	ids := make([]int64, len(rows))
	for i, x := range rows {
		ids[i] = x.ID
	}
	amountRows, err := q.ListReplenishClassificationAmounts(ctx, ids)
	if err != nil {
		return fmt.Errorf("list classification amounts: %w", err)
	}
	amounts := make(map[int64][]reports.Amount, len(rows))
	for _, a := range amountRows {
		var diff int64
		if a.Diff != nil {
			diff = *a.Diff
		}
		amounts[a.ClassificationID] = append(amounts[a.ClassificationID],
			reports.Amount{Denom: a.Denom, Order: a.OrderAmount, Itm: a.ItmAmount, Vendor: a.VendorAmount, Diff: diff})
	}
	for _, x := range rows {
		lines := reports.AmountMismatchLines(reports.AmountMismatchRow{
			TicketNumber: x.TicketNumber, RequestNumber: x.RequestNumber, TerminalID: x.TerminalID, VendorName: x.VendorName,
			BranchCode: x.BranchCode, RegionCode: x.RegionCode, ScheduledDate: datePtr(x.ScheduledDate),
			RealizedDate: datePtr(x.RealizedDate), Classification: x.Classification, WorkdayOffset: x.WorkdayOffset,
			Currency: "IDR", Amounts: amounts[x.ID],
		})
		for _, l := range lines {
			if err := emit(l); err != nil {
				return err
			}
		}
	}
	return nil
}

// tripsFn: region_code is accepted and audited but not applied, same as the legacy report.
func tripsFn(ctx context.Context, q Querier, r ReportParams, _ time.Time, emit func([]string) error) error {
	from, to, err := replenishRange(r)
	if err != nil {
		return err
	}
	rows, err := q.ReportReplenishTrips(ctx, repository.ReportReplenishTripsParams{DateFrom: from, DateTo: to, VendorID: r.VendorID})
	if err != nil {
		return fmt.Errorf("report replenish trips: %w", err)
	}
	for _, x := range rows {
		if err := emit(reports.TripCells(reports.TripRow{
			VendorID: x.VendorID, VendorName: x.VendorName, TerminalID: x.TerminalID, Trips: x.Trips,
			FirstDate: datePtr(x.FirstDate), LastDate: datePtr(x.LastDate),
		})); err != nil {
			return err
		}
	}
	return nil
}
