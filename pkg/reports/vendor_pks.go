package reports

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cimb-niaga/cms/pkg/csvexport"
)

// VendorPks is the Report_Kind of the vendor PKS export (export_jobs.kind).
const VendorPks = "vendor_pks"

// VendorPksRenewalMonthsDefault is the Renewal_Threshold when the env is empty (D6).
const VendorPksRenewalMonthsDefault = 9

const (
	vendorPksRenewalMin = 1
	vendorPksRenewalMax = 24
)

// VendorPksViewerRoles may request the export; route guard and service check share it.
var VendorPksViewerRoles = []string{"ADMIN", "ADMIN_PARAM"}

// PksStatuses maps a pks_status filter key to its CSV label (also the filter whitelist).
var PksStatuses = map[string]string{
	"aktif": "Aktif", "need_renewal": "Need Renewal", "expired": "Expired",
	"terminated": "Terminated", "belum_diisi": "Belum diisi",
}

// ParseVendorPksRenewalMonths parses the renewal threshold (months): empty = 9, otherwise 1..24.
func ParseVendorPksRenewalMonths(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return VendorPksRenewalMonthsDefault, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < vendorPksRenewalMin || n > vendorPksRenewalMax {
		return 0, fmt.Errorf("renewal months must be an integer %d-%d, got %q", vendorPksRenewalMin, vendorPksRenewalMax, raw)
	}
	return n, nil
}

// TodayWIB is the civil date (Asia/Jakarta) of now, as a UTC-midnight time.Time.
func TodayWIB(now time.Time) time.Time { return civilDate(now.In(WIB)) }

// VendorPksHeader is the column header of the export (Req 10.3).
var VendorPksHeader = []string{"Kode Vendor", "Nama Perusahaan", "Nama Direktur Utama", "Telepon Direktur Utama",
	"Email Direktur Utama", "Nama PKS", "Tanggal Efektif PKS", "Tanggal Expired PKS", "Masa Berlaku (bulan)",
	"Monitoring PKS (hari)", "Status PKS"}

// VendorPksRow is one vendor of the export. Nil pointers = empty cell (no PKS / no director).
type VendorPksRow struct {
	VendorCode, VendorName                     string
	DirectorName, DirectorPhone, DirectorEmail *string
	PksName                                    *string
	EffectiveDate, ExpiryDate                  *time.Time
	ValidityMonths, RemainingDays              *int32
	PksStatus                                  *string // key of PksStatuses
}

// VendorPksCells is one CSV line; text cells go through CSVSafe, numbers are plain integers.
func VendorPksCells(r VendorPksRow) []string {
	label := ""
	if r.PksStatus != nil {
		label = PksStatuses[*r.PksStatus]
	}
	return []string{
		csvexport.CSVSafe(r.VendorCode), csvexport.CSVSafe(r.VendorName),
		csvexport.CSVSafe(str(r.DirectorName)), csvexport.CSVSafe(str(r.DirectorPhone)), csvexport.CSVSafe(str(r.DirectorEmail)),
		csvexport.CSVSafe(str(r.PksName)), dateCell(r.EffectiveDate, nil), dateCell(r.ExpiryDate, nil),
		int32Cell(r.ValidityMonths), int32Cell(r.RemainingDays), label,
	}
}

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
