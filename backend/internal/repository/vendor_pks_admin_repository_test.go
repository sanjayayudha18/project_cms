//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

func TestPksRepositories(t *testing.T) {
	tx := integrationTx(t)
	ctx := context.Background()
	tag := time.Now().Format("150405.000000")

	var vendorID, userID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO vendors (code, name, kind, contact_email, is_active)
		 VALUES ($1, $2, 'FLM_VENDOR', $3, true) RETURNING id`,
		"V-PKS-"+tag, "Vendor PKS "+tag, "vpks."+tag+"@example.com",
	).Scan(&vendorID); err != nil {
		t.Fatalf("seed vendor: %v", err)
	}

	if err := tx.QueryRow(ctx, `SELECT id FROM users ORDER BY id LIMIT 1`).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	var typeID int64
	typeCode := "PT" + tag[:4]
	typeName := "PKS Type " + tag
	if err := tx.QueryRow(ctx,
		`INSERT INTO vendor_pks_types (code, name, is_active)
		 VALUES ($1, $2, true) RETURNING id`,
		typeCode, typeName,
	).Scan(&typeID); err != nil {
		t.Fatalf("seed pks type: %v", err)
	}

	wod := "WOD-" + tag
	pksName := "Contract " + tag
	eff := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	exp := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)

	var pksID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO vendor_pks (vendor_id, pks_name, effective_date, expiry_date, created_by, pks_type_id, wod_number, renewal_days)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, 90) RETURNING id`,
		vendorID, pksName, eff, exp, userID, typeID, wod,
	).Scan(&pksID); err != nil {
		t.Fatalf("seed vendor_pks: %v", err)
	}

	t.Run("PksTypeRepository", func(t *testing.T) {
		repo := NewPksTypeRepository(tx, tx)

		list, err := repo.List(ctx)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		found := false
		for _, item := range list {
			if item.ID == typeID {
				found = true
				if !item.InUse {
					t.Errorf("expected type %d to be in_use", typeID)
				}
				break
			}
		}
		if !found {
			t.Errorf("type %d not found in List", typeID)
		}

		byID, err := repo.GetByID(ctx, typeID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if byID == nil || byID.Code != typeCode {
			t.Errorf("GetByID = %+v, want code %s", byID, typeCode)
		}

		missing, err := repo.GetByID(ctx, -999)
		if err != nil {
			t.Fatalf("GetByID(-999): %v", err)
		}
		if missing != nil {
			t.Errorf("GetByID(-999) = %+v, want nil", missing)
		}

		byCode, err := repo.FindByCode(ctx, typeCode)
		if err != nil {
			t.Fatalf("FindByCode: %v", err)
		}
		if byCode == nil || byCode.ID != typeID {
			t.Errorf("FindByCode = %+v, want id %d", byCode, typeID)
		}

		missingCode, err := repo.FindByCode(ctx, "NON_EXISTENT")
		if err != nil {
			t.Fatalf("FindByCode(NON_EXISTENT): %v", err)
		}
		if missingCode != nil {
			t.Errorf("FindByCode(NON_EXISTENT) = %+v, want nil", missingCode)
		}

		pending, err := repo.CodePending(ctx, typeCode)
		if err != nil {
			t.Fatalf("CodePending: %v", err)
		}
		if pending {
			t.Errorf("CodePending(%s) = true, want false", typeCode)
		}

		inUse, err := repo.InUse(ctx, typeID)
		if err != nil {
			t.Fatalf("InUse: %v", err)
		}
		if !inUse {
			t.Errorf("InUse(%d) = false, want true", typeID)
		}
	})

	t.Run("VendorPksAdminRepository", func(t *testing.T) {
		repo := NewVendorPksAdminRepository(tx)

		v, err := repo.Vendor(ctx, vendorID)
		if err != nil {
			t.Fatalf("Vendor: %v", err)
		}
		if v == nil || v.Kind != "FLM_VENDOR" {
			t.Errorf("Vendor(%d) = %+v, want FLM_VENDOR", vendorID, v)
		}

		missingV, err := repo.Vendor(ctx, -999)
		if err != nil {
			t.Fatalf("Vendor(-999): %v", err)
		}
		if missingV != nil {
			t.Errorf("Vendor(-999) = %+v, want nil", missingV)
		}

		act, err := repo.Active(ctx, vendorID, typeID)
		if err != nil {
			t.Fatalf("Active: %v", err)
		}
		if act == nil || act.ID != pksID {
			t.Errorf("Active = %+v, want id %d", act, pksID)
		}

		missingAct, err := repo.Active(ctx, vendorID, -999)
		if err != nil {
			t.Fatalf("Active(-999): %v", err)
		}
		if missingAct != nil {
			t.Errorf("Active(-999) = %+v, want nil", missingAct)
		}

		pks, err := repo.GetByID(ctx, pksID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if pks == nil || pks.WodNumber != wod {
			t.Errorf("GetByID = %+v, want wod %s", pks, wod)
		}

		missingPks, err := repo.GetByID(ctx, -999)
		if err != nil {
			t.Fatalf("GetByID(-999): %v", err)
		}
		if missingPks != nil {
			t.Errorf("GetByID(-999) = %+v, want nil", missingPks)
		}

		pend, err := repo.Pending(ctx, vendorID)
		if err != nil {
			t.Fatalf("Pending: %v", err)
		}
		if len(pend) != 0 {
			t.Errorf("Pending = %+v, want empty", pend)
		}

		pt, err := repo.PksType(ctx, typeID)
		if err != nil {
			t.Fatalf("PksType: %v", err)
		}
		if pt == nil || pt.Code != typeCode {
			t.Errorf("PksType(%d) = %+v, want code %s", typeID, pt, typeCode)
		}

		missingPt, err := repo.PksType(ctx, -999)
		if err != nil {
			t.Fatalf("PksType(-999): %v", err)
		}
		if missingPt != nil {
			t.Errorf("PksType(-999) = %+v, want nil", missingPt)
		}

		ptByCode, err := repo.PksTypeByCode(ctx, typeCode)
		if err != nil {
			t.Fatalf("PksTypeByCode: %v", err)
		}
		if ptByCode == nil || ptByCode.ID != typeID {
			t.Errorf("PksTypeByCode(%s) = %+v, want id %d", typeCode, ptByCode, typeID)
		}

		missingPtByCode, err := repo.PksTypeByCode(ctx, "NON_EXISTENT")
		if err != nil {
			t.Fatalf("PksTypeByCode(NON_EXISTENT): %v", err)
		}
		if missingPtByCode != nil {
			t.Errorf("PksTypeByCode(NON_EXISTENT) = %+v, want nil", missingPtByCode)
		}

		wodTaken, err := repo.WodTaken(ctx, wod)
		if err != nil {
			t.Fatalf("WodTaken: %v", err)
		}
		if !wodTaken {
			t.Errorf("WodTaken(%s) = false, want true", wod)
		}

		wodNotTaken, err := repo.WodTaken(ctx, "NOT_TAKEN_"+tag)
		if err != nil {
			t.Fatalf("WodTaken(not taken): %v", err)
		}
		if wodNotTaken {
			t.Errorf("WodTaken(not taken) = true, want false")
		}
	})

	t.Run("PksListRepository", func(t *testing.T) {
		repo := NewPksListRepository(tx)

		todayDate := pgtype.Date{Time: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), Valid: true}
		vCode := "V-PKS-" + tag

		list, err := repo.List(ctx, db.ListPksAdminParams{
			Today:      todayDate,
			Status:     "semua",
			Vendor:     &vCode,
			PageLimit:  10,
			PageOffset: 0,
		})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(list) != 1 || list[0].ID != pksID {
			t.Fatalf("List = %+v, want id %d", list, pksID)
		}

		count, err := repo.Count(ctx, db.CountPksAdminParams{
			Today:  todayDate,
			Status: "semua",
			Vendor: &vCode,
		})
		if err != nil {
			t.Fatalf("Count: %v", err)
		}
		if count != 1 {
			t.Errorf("Count = %d, want 1", count)
		}

		pend, err := repo.Pending(ctx, db.ListPendingPksChangesParams{
			Vendor: &vCode,
		})
		if err != nil {
			t.Fatalf("Pending: %v", err)
		}
		if len(pend) != 0 {
			t.Errorf("Pending = %+v, want empty", pend)
		}

		locked, err := repo.LockForReminder(ctx, tx, pksID)
		if err != nil {
			t.Fatalf("LockForReminder: %v", err)
		}
		if locked.ID != pksID {
			t.Errorf("LockForReminder id = %d, want %d", locked.ID, pksID)
		}

		updated, err := repo.UpdateRenewalDays(ctx, tx, db.UpdateVendorPksRenewalDaysParams{
			ID:          pksID,
			RenewalDays: 60,
		})
		if err != nil {
			t.Fatalf("UpdateRenewalDays: %v", err)
		}
		if updated.RenewalDays != 60 {
			t.Errorf("RenewalDays = %d, want 60", updated.RenewalDays)
		}
	})
}
