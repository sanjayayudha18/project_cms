package reports

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cimb-niaga/cms/pkg/csvexport"
)

// VendorPks is the Report_Kind of the vendor PKS export (export_jobs.kind).
const VendorPks = "vendor_pks"

// TodayWIB is the civil date (Asia/Jakarta) of now, as a UTC-midnight time.Time.
func TodayWIB(now time.Time) time.Time { return civilDate(now.In(WIB)) }

// VendorPksFileName is vendor-pks-YYYYMMDD.csv, dated by the WIB day the file is made.
func VendorPksFileName(now time.Time) string {
	return "vendor-pks-" + now.In(WIB).Format("20060102") + ".csv"
}

func int32Cell(p *int32) string {
	if p == nil {
		return ""
	}
	return strconv.Itoa(int(*p))
}

// ---- Daftar PKS (spec vendor-pks-list) ----

const (
	RenewalDaysMin     = 30
	RenewalDaysMax     = 3666
	RenewalDaysDefault = 270

	pksListQMaxLen = 100
)

// PksViewerRoles may list/export PKS; route guard and service check share it.
var PksViewerRoles = []string{"ADMIN", "ADMIN_PARAM", "ATM-SPV"}

// PksStatusLabels maps a per-row PKS status key to its label (CSV + screen).
var PksStatusLabels = map[string]string{
	"aktif": "Aktif", "need_renewal": "Need Renewal", "belum_berlaku": "Belum Berlaku",
	"expired": "Expired", "terminated": "Terminated", "diganti": "Diganti",
}

// PksListStatusFilters is the status filter whitelist: the two views plus every status key.
var PksListStatusFilters = func() map[string]struct{} {
	m := map[string]struct{}{"terkini": {}, "semua": {}}
	for k := range PksStatusLabels {
		m[k] = struct{}{}
	}
	return m
}()

// PksListFilter is the validated filter shared by the list screen and the export.
type PksListFilter struct{ Status, PksType, Vendor, Q string }

// ParsePksListFilter trims; empty status = "terkini"; status must be in PksListStatusFilters;
// pks_type/vendor are upper-cased; q is at most 100 characters. Code existence is checked by the caller.
func ParsePksListFilter(status, pksType, vendor, q string) (PksListFilter, error) {
	f := PksListFilter{
		Status:  strings.TrimSpace(status),
		PksType: strings.ToUpper(strings.TrimSpace(pksType)),
		Vendor:  strings.ToUpper(strings.TrimSpace(vendor)),
		Q:       strings.TrimSpace(q),
	}
	if f.Status == "" {
		f.Status = "terkini"
	}
	if _, ok := PksListStatusFilters[f.Status]; !ok {
		return PksListFilter{}, fmt.Errorf("invalid status %q", f.Status)
	}
	if utf8.RuneCountInString(f.Q) > pksListQMaxLen {
		return PksListFilter{}, fmt.Errorf("q must be at most %d characters", pksListQMaxLen)
	}
	return f, nil
}

// PksExportHeader is the new export header (Req 10.3): days instead of months, plus the reminder.
var PksExportHeader = []string{"Kode Vendor", "Nama Perusahaan", "Nama Direktur Utama", "Telepon Direktur Utama",
	"Email Direktur Utama", "Type PKS", "No. WOD", "Nama PKS", "Tanggal Efektif PKS", "Tanggal Expired PKS",
	"Masa Berlaku (hari)", "Reminder (hari)", "Status PKS"}

// PksExportRow is one PKS row of the export. Nil pointers = empty cell.
type PksExportRow struct {
	VendorCode, VendorName                     string
	DirectorName, DirectorPhone, DirectorEmail *string
	PksTypeCode, WodNumber, PksName            *string
	EffectiveDate, ExpiryDate                  *time.Time
	ValidityDays, RenewalDays                  *int32
	Status                                     *string // key of PksStatusLabels
}

// PksExportCells is one CSV line; text cells go through CSVSafe, numbers are plain integers.
func PksExportCells(r PksExportRow) []string {
	label := ""
	if r.Status != nil {
		label = PksStatusLabels[*r.Status]
	}
	text := []string{
		r.VendorCode, r.VendorName, str(r.DirectorName), str(r.DirectorPhone), str(r.DirectorEmail),
		str(r.PksTypeCode), str(r.WodNumber), str(r.PksName),
	}
	cells := make([]string, 0, len(PksExportHeader))
	for _, c := range text {
		cells = append(cells, csvexport.CSVSafe(c))
	}
	return append(cells, dateCell(r.EffectiveDate, nil), dateCell(r.ExpiryDate, nil),
		int32Cell(r.ValidityDays), int32Cell(r.RenewalDays), csvexport.CSVSafe(label))
}
