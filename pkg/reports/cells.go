package reports

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/cimb-niaga/cms/pkg/csvexport"
)

// DsrLateDailyRow is one branch-day of the DSR late report.
type DsrLateDailyRow struct {
	ReportDate                                     string
	VendorCode, VendorName, BranchCode, BranchName string
	RegionCode                                     *string
	Deadline                                       time.Time
	FirstReceivedAt                                *time.Time
	Status                                         string
	LateMinutes                                    int32
	ReceiptChecksum                                string
	MatchedLabels                                  []string
}

// DsrLateSummaryRow is one vendor branch of the monthly summary.
type DsrLateSummaryRow struct {
	VendorCode, VendorName, BranchCode, BranchName string
	RegionCode                                     *string
	ObligedDays, OkDays, LateDays                  int32
	NotSentDays, PendingDays, LateMinutesTotal     int32
}

// Amount is one denom line of a classification.
type Amount struct {
	Denom  int32
	Order  int64
	Itm    int64
	Vendor *int64
	Diff   int64
}

// AmountMismatchRow is one classification with its per-denom amounts.
type AmountMismatchRow struct {
	TicketNumber, RequestNumber        *string
	TerminalID                         string
	VendorName, BranchCode, RegionCode *string
	ScheduledDate, RealizedDate        *time.Time
	Classification                     string
	WorkdayOffset                      *int16
	Currency                           string
	Amounts                            []Amount
}

// TripRow is trips per vendor x terminal.
type TripRow struct {
	VendorID            *int64
	VendorName          *string
	TerminalID          string
	Trips               int64
	FirstDate, LastDate *time.Time
}

var headers = map[string][]string{
	DsrLateDaily: {"report_date", "vendor_code", "vendor", "branch_code", "branch_name", "region_code",
		"deadline_wib", "first_received_at_wib", "status", "late_minutes", "receipt_checksum", "matched_locations"},
	DsrLateSummary: {"vendor_code", "vendor", "branch_code", "branch_name", "region_code",
		"obliged_days", "ok_days", "late_days", "not_sent_days", "pending_days", "late_minutes_total"},
	ReplenishmentAmountMismatch: {"report_date", "ticket_number", "request_number", "terminal_id", "vendor", "branch_code", "region_code",
		"scheduled_date", "realized_date", "classification", "workday_offset", "denom", "order_amount", "itm_amount",
		"vendor_amount", "diff", "currency"},
	ReplenishmentTrips: {"vendor_id", "vendor", "terminal_id", "trips", "first_date", "last_date"},
}

// Header returns a copy of the column header of a Report_Kind (nil for an unknown kind).
func Header(report string) []string {
	return append([]string(nil), headers[report]...)
}

// DsrLateDailyCells: R3 matched_locations = what decided the status, for later disputes.
func DsrLateDailyCells(r DsrLateDailyRow) []string {
	return []string{r.ReportDate, r.VendorCode, r.VendorName, r.BranchCode, r.BranchName, str(r.RegionCode),
		wibCell(&r.Deadline), wibCell(r.FirstReceivedAt), r.Status, itoa(r.LateMinutes),
		r.ReceiptChecksum, strings.Join(r.MatchedLabels, "; ")}
}

// DsrLateSummaryCells is one summary line.
func DsrLateSummaryCells(r DsrLateSummaryRow) []string {
	return []string{r.VendorCode, r.VendorName, r.BranchCode, r.BranchName, str(r.RegionCode),
		itoa(r.ObligedDays), itoa(r.OkDays), itoa(r.LateDays), itoa(r.NotSentDays), itoa(r.PendingDays), itoa(r.LateMinutesTotal)}
}

// AmountMismatchLines returns one line per denom.
func AmountMismatchLines(r AmountMismatchRow) [][]string {
	lines := make([][]string, 0, len(r.Amounts))
	for _, am := range r.Amounts {
		lines = append(lines, []string{dateCell(r.ScheduledDate, r.RealizedDate), str(r.TicketNumber), str(r.RequestNumber), r.TerminalID,
			str(r.VendorName), str(r.BranchCode), str(r.RegionCode), dateCell(r.ScheduledDate, nil), dateCell(r.RealizedDate, nil),
			r.Classification, intCell(r.WorkdayOffset), strconv.Itoa(int(am.Denom)), strconv.FormatInt(am.Order, 10),
			strconv.FormatInt(am.Itm, 10), int64Cell(am.Vendor), strconv.FormatInt(am.Diff, 10), r.Currency})
	}
	return lines
}

// TripCells is one trips line.
func TripCells(r TripRow) []string {
	return []string{int64Cell(r.VendorID), str(r.VendorName), r.TerminalID, strconv.FormatInt(r.Trips, 10),
		dateCell(r.FirstDate, nil), dateCell(r.LastDate, nil)}
}

// FileName is the download name (with .csv): DSR kinds use month (YYYY-MM), replenishment kinds from-to.
func FileName(report string, month, from, to time.Time) string {
	const day = "2006-01-02"
	switch report {
	case DsrLateDaily:
		return "dsr-late-daily-" + month.Format("2006-01") + ".csv"
	case DsrLateSummary:
		return "dsr-late-summary-" + month.Format("2006-01") + ".csv"
	case ReplenishmentAmountMismatch:
		return "replenishment-amount-mismatch-" + from.Format(day) + "-" + to.Format(day) + ".csv"
	case ReplenishmentTrips:
		return "replenishment-trips-" + from.Format(day) + "-" + to.Format(day) + ".csv"
	}
	return ""
}

// WriteCSV writes BOM + header of the report, then every row (CSVSafe per cell, via csvexport.Writer).
func WriteCSV(w io.Writer, report string, rows [][]string) error {
	h := Header(report)
	if len(h) == 0 {
		return fmt.Errorf("reports: unknown report %q", report)
	}
	cw, err := csvexport.NewWriter(w, h)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	return cw.Flush()
}

func itoa(n int32) string { return strconv.Itoa(int(n)) }

// wibCell formats a timestamp in WIB; nil = empty cell.
func wibCell(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.In(WIB).Format("2006-01-02 15:04:05")
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func int64Cell(p *int64) string {
	if p == nil {
		return ""
	}
	return strconv.FormatInt(*p, 10)
}

func intCell(p *int16) string {
	if p == nil {
		return ""
	}
	return strconv.Itoa(int(*p))
}

// dateCell formats the first non-nil date.
func dateCell(a, b *time.Time) string {
	for _, t := range []*time.Time{a, b} {
		if t != nil {
			return t.Format("2006-01-02")
		}
	}
	return ""
}
