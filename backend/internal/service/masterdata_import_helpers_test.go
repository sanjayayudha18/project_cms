package service

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// CSV cells -> entity payloads: money stays the exact cell string, empty
// optional cells become nil (not ""), and a PIC without branch_code is vendor-wide.
func TestImportFieldMappers(t *testing.T) {
	vault := vaultFields(map[string]string{"category": "CASH", "currency_code": "IDR", "min_capacity_amount": "1000000.50", "location_id": "12"})
	if vault.Category != "CASH" || vault.MinCapacityAmount == nil || *vault.MinCapacityAmount != "1000000.50" ||
		vault.MaxCapacityAmount != nil || vault.LocationID == nil || *vault.LocationID != 12 {
		t.Errorf("vaultFields = %+v", vault)
	}

	env := &importEnv{branches: map[string]int64{"V1|B1": 9}}
	pic := picFields(map[string]string{"vendor_code": "V1", "branch_code": "B1", "name": "Ani", "is_notification_recipient": "true"}, env)
	if pic.VendorBranchID == nil || *pic.VendorBranchID != 9 || !pic.IsNotificationRecipient || pic.Email != nil {
		t.Errorf("branch PIC = %+v", pic)
	}
	if wide := picFields(map[string]string{"vendor_code": "V1", "name": "Budi"}, env); wide.VendorBranchID != nil || wide.IsNotificationRecipient {
		t.Errorf("vendor-wide PIC = %+v", wide)
	}

	if pic.Roles != nil {
		t.Errorf("blank roles cell: Roles = %v, want nil", *pic.Roles)
	}
	if r := picFields(map[string]string{"vendor_code": "V1", "name": "Budi", "roles": "none"}, env).Roles; r == nil || len(*r) != 0 {
		t.Errorf("none roles cell: Roles = %v, want empty non-nil", r)
	}

	loc, atm := atmFields(map[string]string{"location_id": "5", "machine_type": "CRM", "capacity_amount": "500000000",
		"low_threshold_amount": "", "blacklisted": "true", "priority_class": "VIP"})
	if loc != 5 || atm.LocationID != 5 || atm.MachineType != "CRM" || atm.CapacityAmount == nil || *atm.CapacityAmount != "500000000" ||
		atm.LowThresholdAmount != nil || !atm.Blacklisted || atm.PriorityClass == nil || *atm.PriorityClass != "VIP" {
		t.Errorf("atmFields = %d %+v", loc, atm)
	}
	if _, notBlacklisted := atmFields(map[string]string{"blacklisted": "TRUE"}); notBlacklisted.Blacklisted {
		t.Error(`only the exact "true" cell blacklists`)
	}
}

// A toggle row (only is_active changed) stages enable/disable, never an update.
func TestImport_StageToggle(t *testing.T) {
	repo := &fakeVendorBranchAdminRepo{getByIDFunc: func(context.Context, int64) (*db.GetVendorBranchAdminByIDRow, error) {
		return &db.GetVendorBranchAdminByIDRow{ID: 1}, nil
	}}
	sub := &fakeVendorBranchSubmitter{}
	svcs := ImportServices{Branches: NewVendorBranchAdminService(repo, sub, &fakeVendorBranchChildCounter{})}
	for active, wantOp := range map[string]string{"true": "enable", "false": "disable"} {
		row := ImportRow{Row: 2, Op: "update", ID: 1, Toggle: true, Cells: map[string]string{"is_active": active}}
		if err := stageRow(adminCtx(7), 7, ExportVendorBranches, row, &importEnv{}, svcs, "127.0.0.1"); err != nil {
			t.Fatalf("is_active=%s: %v", active, err)
		}
		if sub.lastRequest.Op != wantOp || sub.lastRequest.EntityID == nil || *sub.lastRequest.EntityID != 1 {
			t.Errorf("is_active=%s staged %+v, want op %s on id 1", active, sub.lastRequest, wantOp)
		}
	}
}

func TestImport_RowRejection(t *testing.T) {
	var invalid *ImportInvalidError
	if err := rowRejection(4, &ValidationError{Field: "npwp", Message: "tidak valid"}); !errors.As(err, &invalid) ||
		invalid.Errors[0] != (ImportRowError{Row: 4, Field: "npwp", Message: "tidak valid"}) {
		t.Errorf("validation -> %v", err)
	}
	if err := rowRejection(5, ErrVendorCodeConflict); !errors.As(err, &invalid) || invalid.Errors[0].Row != 5 || invalid.Errors[0].Field != "row" {
		t.Errorf("known conflict -> %v", err)
	}
	boom := errors.New("db down")
	if err := rowRejection(6, boom); errors.As(err, &invalid) || !errors.Is(err, boom) || !strings.Contains(err.Error(), "row 6") {
		t.Errorf("unexpected error must stay a 500, got %v", err)
	}
}

func TestExportTemplate(t *testing.T) {
	if _, err := ParseExportEntity("users"); !errors.Is(err, ErrExportUnknownEntity) {
		t.Errorf("unknown entity err = %v", err)
	}
	for _, entity := range []string{ExportVendors, ExportVendorBranches, ExportVendorVaults, ExportVendorPICs, ExportATMs, ExportATMAssignments} {
		if got, err := ParseExportEntity(entity); err != nil || got != entity {
			t.Errorf("ParseExportEntity(%s) = %q, %v", entity, got, err)
		}
		var buf bytes.Buffer
		if err := WriteExportTemplate(&buf, entity); err != nil {
			t.Fatalf("%s: %v", entity, err)
		}
		header, _ := MasterDataExportHeader(entity)
		if want := utf8BOM + strings.Join(header, ",") + "\n"; buf.String() != want {
			t.Errorf("%s template = %q, want BOM + header only %q", entity, buf.String(), want)
		}
	}
	if err := WriteExportTemplate(&bytes.Buffer{}, "users"); !errors.Is(err, ErrExportUnknownEntity) {
		t.Errorf("unknown template err = %v", err)
	}
}
