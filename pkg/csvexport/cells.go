package csvexport

import (
	"strconv"
	"strings"
)

// Neutral row types: fields mirror the shared sqlc Export*BatchRow structs
// (NULL is already "" in SQL; amounts/dates are text).

type VendorRow struct {
	ID                                                                 int64
	Code, Name, LegalName, Npwp, ContactEmail, ContactPhone, HqAddress string
	IsActive                                                           bool
}

type VendorBranchRow struct {
	ID                                                                           int64
	VendorCode, BranchCode, BranchName, Region, RegionCode, Category, LocationID string
	IsActive                                                                     bool
}

type VendorVaultRow struct {
	ID                                                                                    int64
	VendorCode, BranchCode, VaultCode, Category, CurrencyCode                             string
	MinCapacityAmount, MaxCapacityAmount, Latitude, Longitude, OperatingHours, LocationID string
	IsActive                                                                              bool
}

type VendorPICRow struct {
	ID                                                   int64
	VendorCode, BranchCode, Name, Position, Phone, Email string
	IsNotificationRecipient, IsActive                    bool
	Roles                                                []string
}

type ATMRow struct {
	ID                                                          int64
	TerminalID                                                  string
	LocationID                                                  int64
	MachineType, Brand, Model, OperationHours, DeploymentType   string
	CapacityAmount, LowThresholdAmount, CriticalThresholdAmount string
	Blacklisted                                                 bool
	EscrowAccount, PriorityClass                                string
	IsActive                                                    bool
}

type ATMAssignmentRow struct {
	ID                                                             int64
	TerminalID, VendorCode, BranchCode, PackageSource, PackageCode string
	EffectiveStartDate, EffectiveEndDate                           string
	IsActive                                                       bool
}

func boolCell(b bool) string { return strconv.FormatBool(b) }

func id(n int64) string { return strconv.FormatInt(n, 10) }

// PICRolesCell is the CSV form of a PIC's roles: codes joined by ";", or "none".
// The importer formats a parsed roles cell with it too, so equal sets compare equal.
func PICRolesCell(roles []string) string {
	if len(roles) == 0 {
		return PICRolesNone
	}
	return strings.Join(roles, ";")
}

func VendorCells(r VendorRow) []string {
	return []string{id(r.ID), r.Code, r.Name, r.LegalName, r.Npwp, r.ContactEmail, r.ContactPhone, r.HqAddress, boolCell(r.IsActive)}
}

func VendorBranchCells(r VendorBranchRow) []string {
	return []string{id(r.ID), r.VendorCode, r.BranchCode, r.BranchName, r.Region, r.RegionCode, r.Category, r.LocationID, boolCell(r.IsActive)}
}

func VendorVaultCells(r VendorVaultRow) []string {
	return []string{id(r.ID), r.VendorCode, r.BranchCode, r.VaultCode, r.Category, r.CurrencyCode,
		r.MinCapacityAmount, r.MaxCapacityAmount, r.Latitude, r.Longitude, r.OperatingHours, r.LocationID, boolCell(r.IsActive)}
}

func VendorPICCells(r VendorPICRow) []string {
	return []string{id(r.ID), r.VendorCode, r.BranchCode, r.Name, r.Position, r.Phone, r.Email,
		boolCell(r.IsNotificationRecipient), boolCell(r.IsActive), PICRolesCell(r.Roles)}
}

func ATMCells(r ATMRow) []string {
	return []string{id(r.ID), r.TerminalID, id(r.LocationID), r.MachineType, r.Brand, r.Model, r.OperationHours, r.DeploymentType,
		r.CapacityAmount, r.LowThresholdAmount, r.CriticalThresholdAmount, boolCell(r.Blacklisted), r.EscrowAccount, r.PriorityClass, boolCell(r.IsActive)}
}

func ATMAssignmentCells(r ATMAssignmentRow) []string {
	return []string{id(r.ID), r.TerminalID, r.VendorCode, r.BranchCode, r.PackageSource, r.PackageCode, r.EffectiveStartDate, r.EffectiveEndDate, boolCell(r.IsActive)}
}
