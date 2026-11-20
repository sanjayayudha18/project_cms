//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

func TestVendorVaultAdminRepository(t *testing.T) {
	tx := integrationTx(t)
	ctx := context.Background()
	tag := time.Now().Format("150405.000000")

	var vendorID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO vendors (code, name, contact_email, is_active) VALUES ($1, $2, $3, true) RETURNING id`,
		"VV_VND_"+tag, "Vendor Vault "+tag, "vv."+tag+"@example.com",
	).Scan(&vendorID); err != nil {
		t.Fatalf("seed vendor: %v", err)
	}

	var branchID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO vendor_branches (vendor_id, branch_code, branch_name, is_active) VALUES ($1, $2, $3, true) RETURNING id`,
		vendorID, "VV_BR_"+tag, "Branch Vault "+tag,
	).Scan(&branchID); err != nil {
		t.Fatalf("seed branch: %v", err)
	}

	vaultCodeActive := "VV_VLT_ACT_" + tag
	vaultNameActive := "VV_Name_ACT_" + tag
	var activeVaultID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO vendor_vaults (vendor_branch_id, vault_code, vault_name, type, category, currency_code, is_active) VALUES ($1, $2, $3, 'main', 'ATM', 'IDR', true) RETURNING id`,
		branchID, vaultCodeActive, vaultNameActive,
	).Scan(&activeVaultID); err != nil {
		t.Fatalf("seed active vault: %v", err)
	}

	vaultCodeDisabled := "VV_VLT_DIS_" + tag
	vaultNameDisabled := "VV_Name_DIS_" + tag
	var disabledVaultID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO vendor_vaults (vendor_branch_id, vault_code, vault_name, type, category, currency_code, is_active, deleted_at) VALUES ($1, $2, $3, 'main', 'ATM', 'IDR', false, now()) RETURNING id`,
		branchID, vaultCodeDisabled, vaultNameDisabled,
	).Scan(&disabledVaultID); err != nil {
		t.Fatalf("seed disabled vault: %v", err)
	}

	repo := NewVendorVaultAdminRepository(tx, tx)

	t.Run("List and Count", func(t *testing.T) {
		q := tag
		list, err := repo.List(ctx, db.ListVendorVaultsAdminParams{
			VendorID:       vendorID,
			VendorBranchID: &branchID,
			Q:              &q,
			Status:         "all",
			PageLimit:      10,
			PageOffset:     0,
		})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(list) != 2 {
			t.Errorf("len(List) = %d, want 2", len(list))
		}

		count, err := repo.Count(ctx, db.CountVendorVaultsAdminParams{
			VendorID:       vendorID,
			VendorBranchID: &branchID,
			Q:              &q,
			Status:         "all",
		})
		if err != nil {
			t.Fatalf("Count: %v", err)
		}
		if count != 2 {
			t.Errorf("Count = %d, want 2", count)
		}
	})

	t.Run("GetByID", func(t *testing.T) {
		v, err := repo.GetByID(ctx, activeVaultID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if v == nil || v.ID != activeVaultID {
			t.Errorf("GetByID = %+v, want id %d", v, activeVaultID)
		}

		missing, err := repo.GetByID(ctx, -999)
		if err != nil {
			t.Fatalf("GetByID(-999): %v", err)
		}
		if missing != nil {
			t.Errorf("GetByID(-999) = %+v, want nil", missing)
		}
	})

	t.Run("FindByCode", func(t *testing.T) {
		id, err := repo.FindByCode(ctx, vaultCodeActive)
		if err != nil {
			t.Fatalf("FindByCode: %v", err)
		}
		if id == nil || *id != activeVaultID {
			t.Errorf("FindByCode = %v, want %d", id, activeVaultID)
		}

		missing, err := repo.FindByCode(ctx, "NON_EXISTENT")
		if err != nil {
			t.Fatalf("FindByCode(NON_EXISTENT): %v", err)
		}
		if missing != nil {
			t.Errorf("FindByCode(NON_EXISTENT) = %v, want nil", missing)
		}
	})

	t.Run("FindByBranchAndName", func(t *testing.T) {
		id, err := repo.FindByBranchAndName(ctx, branchID, vaultNameActive)
		if err != nil {
			t.Fatalf("FindByBranchAndName: %v", err)
		}
		if id == nil || *id != activeVaultID {
			t.Errorf("FindByBranchAndName = %v, want %d", id, activeVaultID)
		}

		missing, err := repo.FindByBranchAndName(ctx, branchID, "NON_EXISTENT")
		if err != nil {
			t.Fatalf("FindByBranchAndName(NON_EXISTENT): %v", err)
		}
		if missing != nil {
			t.Errorf("FindByBranchAndName(NON_EXISTENT) = %v, want nil", missing)
		}
	})

	t.Run("GenerateNextVaultName", func(t *testing.T) {
		name, err := repo.GenerateNextVaultName(ctx, branchID)
		if err != nil {
			t.Fatalf("GenerateNextVaultName: %v", err)
		}
		if name == "" {
			t.Errorf("GenerateNextVaultName returned empty string")
		}

		missingName, err := repo.GenerateNextVaultName(ctx, -999)
		if err != nil {
			t.Logf("GenerateNextVaultName(-999) err: %v", err)
		}
		_ = missingName
	})

	t.Run("CountActiveByBranch", func(t *testing.T) {
		count, err := repo.CountActiveByBranch(ctx, branchID)
		if err != nil {
			t.Fatalf("CountActiveByBranch: %v", err)
		}
		if count != 1 {
			t.Errorf("CountActiveByBranch = %d, want 1", count)
		}
	})

	t.Run("BranchVendorID", func(t *testing.T) {
		vID, err := repo.BranchVendorID(ctx, branchID)
		if err != nil {
			t.Fatalf("BranchVendorID: %v", err)
		}
		if vID == nil || *vID != vendorID {
			t.Errorf("BranchVendorID = %v, want %d", vID, vendorID)
		}

		missing, err := repo.BranchVendorID(ctx, -999)
		if err != nil {
			t.Fatalf("BranchVendorID(-999): %v", err)
		}
		if missing != nil {
			t.Errorf("BranchVendorID(-999) = %v, want nil", missing)
		}
	})
}
