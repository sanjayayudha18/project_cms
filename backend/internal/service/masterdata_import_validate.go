package service

import (
	"strconv"
	"strings"
	"time"
)

func optStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// boolCol normalizes a true/false column in place. Blank takes def when
// creating and is an error when updating (an update must say what it means).
func boolCol(v map[string]string, col, def string, creating bool, add func(string, string)) {
	switch s := strings.ToLower(v[col]); s {
	case "true", "false":
		v[col] = s
	case "":
		if creating {
			v[col] = def
		} else {
			add(col, "wajib true atau false")
		}
	default:
		add(col, "harus true atau false")
	}
}

func requiredCols(v map[string]string, add func(string, string), cols ...string) {
	for _, c := range cols {
		if v[c] == "" {
			add(c, "wajib diisi")
		}
	}
}

func (e *importEnv) location(v map[string]string, add func(string, string), required bool) {
	if v["location_id"] == "" {
		if required {
			add("location_id", "wajib diisi")
		}
		return
	}
	n, err := strconv.ParseInt(v["location_id"], 10, 64)
	if err != nil || n <= 0 {
		add("location_id", "harus bilangan bulat positif")
		return
	}
	if !e.locations[n] {
		add("location_id", "lokasi tidak ditemukan")
	}
}

func (e *importEnv) validateFields(v map[string]string, creating bool, add func(string, string), fromErr func(error)) {
	boolCol(v, "is_active", "true", creating, add)
	if creating && v["is_active"] == "false" {
		add("is_active", "data baru harus true (nonaktifkan lewat perubahan terpisah)")
	}
	switch e.entity {
	case ExportVendors:
		validateVendor(v, add, fromErr)
	case ExportVendorBranches:
		e.validateVendorBranch(v, add, fromErr)
	case ExportVendorVaults:
		e.validateVault(v, add, fromErr)
	case ExportVendorPICs:
		e.validatePIC(v, creating, add)
	case ExportATMs:
		e.validateATM(v, creating, add, fromErr)
	case ExportATMAssignments:
		e.validateAssignment(v, add)
	}
}

func validateVendor(v map[string]string, add func(string, string), fromErr func(error)) {
	requiredCols(v, add, "code", "name")
	if v["contact_email"] != "" && !isValidEmail(v["contact_email"]) {
		add("contact_email", "format email tidak valid")
	}
	if n, err := normalizeNPWP(v["npwp"]); err != nil {
		fromErr(err)
	} else {
		v["npwp"] = n
	}
}

func (e *importEnv) validateVendorBranch(v map[string]string, add func(string, string), fromErr func(error)) {
	requiredCols(v, add, "vendor_code", "branch_code", "branch_name")
	if v["vendor_code"] != "" && e.vendors[v["vendor_code"]] == 0 {
		add("vendor_code", "vendor tidak ditemukan atau nonaktif")
	}
	rc := v["region_code"]
	if n, err := normalizeBranchRegionCode(&rc); err != nil {
		fromErr(err)
	} else {
		v["region_code"] = notesOrEmpty(n)
	}
	v["category"] = strings.ToUpper(v["category"])
	if !validVendorBranchCategories[v["category"]] {
		add("category", "harus ATM, CASH, atau ATM_CASH")
	}
	e.location(v, add, false)
}

func (e *importEnv) validateVault(v map[string]string, add func(string, string), fromErr func(error)) {
	requiredCols(v, add, "vendor_code", "branch_code", "vault_code")
	if v["vendor_code"] != "" && v["branch_code"] != "" && e.branches[v["vendor_code"]+"|"+v["branch_code"]] == 0 {
		add("branch_code", "cabang tidak ditemukan atau nonaktif")
	}
	p := VendorVaultUpdatePayload{Category: v["category"], CurrencyCode: v["currency_code"],
		MinCapacityAmount: optStr(v["min_capacity_amount"]), MaxCapacityAmount: optStr(v["max_capacity_amount"]),
		Latitude: optStr(v["latitude"]), Longitude: optStr(v["longitude"])}
	if err := validateVaultFields(&p); err != nil {
		fromErr(err)
	} else {
		v["category"], v["currency_code"] = p.Category, p.CurrencyCode
	}
	e.location(v, add, false)
}

func (e *importEnv) validatePIC(v map[string]string, creating bool, add func(string, string)) {
	requiredCols(v, add, "vendor_code", "name")
	if v["vendor_code"] != "" && e.vendors[v["vendor_code"]] == 0 {
		add("vendor_code", "vendor tidak ditemukan atau nonaktif")
	}
	if v["branch_code"] != "" && e.branches[v["vendor_code"]+"|"+v["branch_code"]] == 0 {
		add("branch_code", "cabang tidak ditemukan atau nonaktif untuk vendor ini")
	}
	// Same rules as VendorPicAdminService.validate.
	if v["email"] != "" && (!isValidEmail(v["email"]) || strings.ContainsAny(v["email"], "<> ")) {
		add("email", "format email tidak valid")
	}
	if v["phone"] != "" && !phoneRe.MatchString(strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(v["phone"])) {
		add("phone", "format telepon tidak valid (6-15 digit, boleh diawali +)")
	}
	boolCol(v, "is_notification_recipient", "false", creating, add)
}

func (e *importEnv) validateATM(v map[string]string, creating bool, add func(string, string), fromErr func(error)) {
	requiredCols(v, add, "terminal_id")
	e.location(v, add, true)
	locID, _ := strconv.ParseInt(v["location_id"], 10, 64)
	if _, _, _, _, _, err := validateATMEditableFields(locID, v["machine_type"], v["brand"], v["model"], v["operation_hours"], v["deployment_type"],
		optStr(v["priority_class"]), optStr(v["capacity_amount"]), optStr(v["low_threshold_amount"]), optStr(v["critical_threshold_amount"])); err != nil {
		fromErr(err)
	}
	boolCol(v, "blacklisted", "false", creating, add)
}

func (e *importEnv) validateAssignment(v map[string]string, add func(string, string)) {
	requiredCols(v, add, "terminal_id", "vendor_code", "branch_code", "package_code")
	if v["terminal_id"] != "" && e.atms[v["terminal_id"]] == 0 {
		add("terminal_id", "ATM tidak ditemukan atau nonaktif")
	}
	e.validateAssignmentPackage(v, add)
	start, serr := time.Parse(assignmentDateLayout, v["effective_start_date"])
	if serr != nil {
		add("effective_start_date", "wajib format YYYY-MM-DD")
	}
	if v["effective_end_date"] != "" {
		end, eerr := time.Parse(assignmentDateLayout, v["effective_end_date"])
		switch {
		case eerr != nil:
			add("effective_end_date", "wajib format YYYY-MM-DD")
		case serr == nil && end.Before(start):
			add("effective_end_date", "tidak boleh sebelum tanggal mulai")
		}
	}
}

// validateAssignmentPackage checks the package against its source: a branch special
// package, or a vendor-wide tariff label on an active vendor cabang.
func (e *importEnv) validateAssignmentPackage(v map[string]string, add func(string, string)) {
	switch v["package_source"] {
	case AssignmentSourceBranch:
		if v["package_code"] != "" && e.packages[v["vendor_code"]+"|"+v["branch_code"]+"|"+v["package_code"]] == 0 {
			add("package_code", "paket tidak ditemukan atau nonaktif")
		}
	case AssignmentSourceVendor:
		if v["vendor_code"] != "" && e.vendors[v["vendor_code"]] == 0 {
			add("vendor_code", "vendor tidak ditemukan atau nonaktif")
		} else if v["branch_code"] != "" && e.branches[v["vendor_code"]+"|"+v["branch_code"]] == 0 {
			add("branch_code", "cabang tidak ditemukan atau nonaktif")
		}
		if v["package_code"] != "" && !e.labels[v["vendor_code"]+"|"+v["package_code"]] {
			add("package_code", "vendor tidak punya tarif aktif untuk paket ini")
		}
	default:
		add("package_source", "harus branch atau vendor")
	}
}
