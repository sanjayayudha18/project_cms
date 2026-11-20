//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

func TestVendorPicAdminRepository(t *testing.T) {
	tx := integrationTx(t)
	ctx := context.Background()
	tag := time.Now().Format("150405.000000")

	var vendorID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO vendors (code, name, contact_email, is_active) VALUES ($1, $2, $3, true) RETURNING id`,
		"VP_VND_"+tag, "Vendor PIC "+tag, "vpic."+tag+"@example.com",
	).Scan(&vendorID); err != nil {
		t.Fatalf("seed vendor: %v", err)
	}

	var branchID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO vendor_branches (vendor_id, branch_code, branch_name, is_active) VALUES ($1, $2, $3, true) RETURNING id`,
		vendorID, "VP_BR_"+tag, "Branch PIC "+tag,
	).Scan(&branchID); err != nil {
		t.Fatalf("seed branch: %v", err)
	}

	nameActive := "Active PIC " + tag
	var activePicID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO vendor_pics (vendor_id, vendor_branch_id, name, is_notification_recipient, is_active, is_director)
		 VALUES ($1, $2, $3, true, true, false) RETURNING id`,
		vendorID, branchID, nameActive,
	).Scan(&activePicID); err != nil {
		t.Fatalf("seed active pic: %v", err)
	}

	nameDirector := "Director PIC " + tag
	var directorPicID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO vendor_pics (vendor_id, vendor_branch_id, name, is_notification_recipient, is_active, is_director, roles)
		 VALUES ($1, NULL, $2, false, true, true, '{}') RETURNING id`,
		vendorID, nameDirector,
	).Scan(&directorPicID); err != nil {
		t.Fatalf("seed director pic: %v", err)
	}

	nameDisabled := "Disabled PIC " + tag
	var disabledPicID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO vendor_pics (vendor_id, vendor_branch_id, name, is_notification_recipient, is_active, is_director, deleted_at)
		 VALUES ($1, $2, $3, false, false, false, now()) RETURNING id`,
		vendorID, branchID, nameDisabled,
	).Scan(&disabledPicID); err != nil {
		t.Fatalf("seed disabled pic: %v", err)
	}

	repo := NewVendorPicAdminRepository(tx, tx)

	t.Run("List and Count", func(t *testing.T) {
		q := tag
		list, err := repo.List(ctx, db.ListVendorPicsAdminParams{
			VendorID:   vendorID,
			Q:          &q,
			Status:     "all",
			PageLimit:  10,
			PageOffset: 0,
		})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(list) != 3 {
			t.Errorf("len(List) = %d, want 3", len(list))
		}

		count, err := repo.Count(ctx, db.CountVendorPicsAdminParams{
			VendorID: vendorID,
			Q:        &q,
			Status:   "all",
		})
		if err != nil {
			t.Fatalf("Count: %v", err)
		}
		if count != 3 {
			t.Errorf("Count = %d, want 3", count)
		}
	})

	t.Run("GetByID", func(t *testing.T) {
		p, err := repo.GetByID(ctx, activePicID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if p == nil || p.ID != activePicID {
			t.Errorf("GetByID = %+v, want id %d", p, activePicID)
		}

		missing, err := repo.GetByID(ctx, -999)
		if err != nil {
			t.Fatalf("GetByID(-999): %v", err)
		}
		if missing != nil {
			t.Errorf("GetByID(-999) = %+v, want nil", missing)
		}
	})

	t.Run("CountNotificationRecipients", func(t *testing.T) {
		recipients, err := repo.CountNotificationRecipients(ctx, vendorID)
		if err != nil {
			t.Fatalf("CountNotificationRecipients: %v", err)
		}
		if recipients != 1 {
			t.Errorf("CountNotificationRecipients = %d, want 1", recipients)
		}
	})

	t.Run("OtherActiveDirectorExists", func(t *testing.T) {
		exists, err := repo.OtherActiveDirectorExists(ctx, vendorID, nil)
		if err != nil {
			t.Fatalf("OtherActiveDirectorExists(nil): %v", err)
		}
		if !exists {
			t.Errorf("OtherActiveDirectorExists(nil) = false, want true")
		}

		exists, err = repo.OtherActiveDirectorExists(ctx, vendorID, &directorPicID)
		if err != nil {
			t.Fatalf("OtherActiveDirectorExists(directorPicID): %v", err)
		}
		if exists {
			t.Errorf("OtherActiveDirectorExists(directorPicID) = true, want false")
		}
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
