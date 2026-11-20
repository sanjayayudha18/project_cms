//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

func TestCashCountRepositories(t *testing.T) {
	tx := integrationTx(t)
	ctx := context.Background()
	tag := time.Now().Format("150405")

	var adminUserID int64
	if err := tx.QueryRow(ctx,
		`SELECT u.id FROM users u JOIN roles r ON r.id = u.role_id WHERE r.role = 'ADMIN' ORDER BY u.id LIMIT 1`,
	).Scan(&adminUserID); err != nil {
		t.Fatalf("fetch admin user: %v", err)
	}

	t.Run("CashCountCategoryParamsAdminRepository", func(t *testing.T) {
		repo := NewCashCountCategoryParamsAdminRepository(tx, tx)

		list, err := repo.List(ctx)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(list) == 0 {
			t.Errorf("expected at least 1 seeded category params, got 0")
		}

		act, err := repo.GetActive(ctx)
		if err != nil {
			t.Fatalf("GetActive: %v", err)
		}
		if act == nil {
			t.Errorf("expected active category params, got nil")
		}
	})

	t.Run("CashCountAnomalyCategoryAdminRepository", func(t *testing.T) {
		code := "ANOM_" + tag
		name := "Anomaly " + tag
		var id int64
		if err := tx.QueryRow(ctx,
			`INSERT INTO cash_count_anomaly_categories (code, name, is_active) VALUES ($1, $2, true) RETURNING id`,
			code, name,
		).Scan(&id); err != nil {
			t.Fatalf("seed anomaly category: %v", err)
		}

		repo := NewCashCountAnomalyCategoryAdminRepository(tx, tx)

		list, err := repo.List(ctx, "active")
		if err != nil {
			t.Fatalf("List(active): %v", err)
		}
		found := false
		for _, item := range list {
			if item.ID == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("id %d not found in List", id)
		}

		got, err := repo.GetByID(ctx, id)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got == nil || got.Code != code {
			t.Errorf("GetByID = %+v, want code %s", got, code)
		}

		missing, err := repo.GetByID(ctx, -999)
		if err != nil {
			t.Fatalf("GetByID(-999): %v", err)
		}
		if missing != nil {
			t.Errorf("GetByID(-999) = %+v, want nil", missing)
		}

		foundID, err := repo.FindByCode(ctx, code)
		if err != nil {
			t.Fatalf("FindByCode: %v", err)
		}
		if foundID == nil || *foundID != id {
			t.Errorf("FindByCode(%s) = %v, want %d", code, foundID, id)
		}

		missingCode, err := repo.FindByCode(ctx, "NON_EXISTENT")
		if err != nil {
			t.Fatalf("FindByCode(NON_EXISTENT): %v", err)
		}
		if missingCode != nil {
			t.Errorf("FindByCode(NON_EXISTENT) = %v, want nil", missingCode)
		}
	})

	t.Run("CashCountChecklistItemAdminRepository", func(t *testing.T) {
		category := "SELECTED_MACHINE"
		day := "DAY1"
		pointNo := int16(99)
		itemText := "Check machine " + tag
		var id int64
		if err := tx.QueryRow(ctx,
			`INSERT INTO cash_count_checklist_items (category, day, point_no, item_text, is_active)
			 VALUES ($1, $2, $3, $4, true) RETURNING id`,
			category, day, pointNo, itemText,
		).Scan(&id); err != nil {
			t.Fatalf("seed checklist item: %v", err)
		}

		repo := NewCashCountChecklistItemAdminRepository(tx, tx)

		list, err := repo.List(ctx, db.ListCashCountChecklistItemsAdminParams{
			Category: category,
			Status:   "active",
		})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		found := false
		for _, item := range list {
			if item.ID == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("id %d not found in List", id)
		}

		got, err := repo.GetByID(ctx, id)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got == nil || got.ID != id {
			t.Errorf("GetByID = %+v, want id %d", got, id)
		}

		missing, err := repo.GetByID(ctx, -999)
		if err != nil {
			t.Fatalf("GetByID(-999): %v", err)
		}
		if missing != nil {
			t.Errorf("GetByID(-999) = %+v, want nil", missing)
		}

		foundPoint, err := repo.FindActivePoint(ctx, category, day, pointNo, 0)
		if err != nil {
			t.Fatalf("FindActivePoint: %v", err)
		}
		if foundPoint == nil || *foundPoint != id {
			t.Errorf("FindActivePoint = %v, want %d", foundPoint, id)
		}

		excludedPoint, err := repo.FindActivePoint(ctx, category, day, pointNo, id)
		if err != nil {
			t.Fatalf("FindActivePoint(excluded): %v", err)
		}
		if excludedPoint != nil {
			t.Errorf("FindActivePoint(excluded) = %v, want nil", excludedPoint)
		}
	})

	t.Run("CashCountAreaAdminRepository", func(t *testing.T) {
		code := "AREA_" + tag
		name := "Area " + tag
		var areaID int64
		if err := tx.QueryRow(ctx,
			`INSERT INTO cash_count_areas (code, name, is_active) VALUES ($1, $2, true) RETURNING id`,
			code, name,
		).Scan(&areaID); err != nil {
			t.Fatalf("seed area: %v", err)
		}

		repo := NewCashCountAreaAdminRepository(tx, tx)

		list, err := repo.List(ctx, "active")
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		found := false
		for _, item := range list {
			if item.ID == areaID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("area %d not found in List", areaID)
		}

		got, err := repo.GetByID(ctx, areaID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got == nil || got.Code != code {
			t.Errorf("GetByID = %+v, want code %s", got, code)
		}

		missing, err := repo.GetByID(ctx, -999)
		if err != nil {
			t.Fatalf("GetByID(-999): %v", err)
		}
		if missing != nil {
			t.Errorf("GetByID(-999) = %+v, want nil", missing)
		}

		foundID, err := repo.FindByCode(ctx, code)
		if err != nil {
			t.Fatalf("FindByCode: %v", err)
		}
		if foundID == nil || *foundID != areaID {
			t.Errorf("FindByCode(%s) = %v, want %d", code, foundID, areaID)
		}

		missingCode, err := repo.FindByCode(ctx, "NON_EXISTENT")
		if err != nil {
			t.Fatalf("FindByCode(NON_EXISTENT): %v", err)
		}
		if missingCode != nil {
			t.Errorf("FindByCode(NON_EXISTENT) = %v, want nil", missingCode)
		}

		conflicts, err := repo.RegionConflicts(ctx, []string{"REG01"}, areaID)
		if err != nil {
			t.Fatalf("RegionConflicts: %v", err)
		}
		_ = conflicts

		unknowns, err := repo.UnknownRegionCodes(ctx, []string{"UNKNOWN_REG"})
		if err != nil {
			t.Fatalf("UnknownRegionCodes: %v", err)
		}
		_ = unknowns

		executors, err := repo.ActiveExecutorCount(ctx, areaID)
		if err != nil {
			t.Fatalf("ActiveExecutorCount: %v", err)
		}
		if executors != 0 {
			t.Errorf("ActiveExecutorCount = %d, want 0", executors)
		}
	})

	t.Run("CashCountPicAdminRepository", func(t *testing.T) {
		code := "PICAR_" + tag
		name := "Pic Area " + tag
		var areaID int64
		if err := tx.QueryRow(ctx,
			`INSERT INTO cash_count_areas (code, name, is_active) VALUES ($1, $2, true) RETURNING id`,
			code, name,
		).Scan(&areaID); err != nil {
			t.Fatalf("seed area: %v", err)
		}

		var picID int64
		if err := tx.QueryRow(ctx,
			`INSERT INTO cash_count_pics (pic_role, user_id, area_id, is_active) VALUES ('EXECUTOR', $1, $2, true) RETURNING id`,
			adminUserID, areaID,
		).Scan(&picID); err != nil {
			t.Fatalf("seed pic: %v", err)
		}

		repo := NewCashCountPicAdminRepository(tx, tx)

		list, err := repo.List(ctx, db.ListCashCountPicsAdminParams{
			PicRole: "EXECUTOR",
			Status:  "active",
		})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		found := false
		for _, item := range list {
			if item.ID == picID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("pic %d not found in List", picID)
		}

		got, err := repo.GetByID(ctx, picID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got == nil || got.ID != picID {
			t.Errorf("GetByID = %+v, want id %d", got, picID)
		}

		missing, err := repo.GetByID(ctx, -999)
		if err != nil {
			t.Fatalf("GetByID(-999): %v", err)
		}
		if missing != nil {
			t.Errorf("GetByID(-999) = %+v, want nil", missing)
		}

		eligible, err := repo.UserEligible(ctx, adminUserID, "EXECUTOR")
		if err != nil {
			t.Fatalf("UserEligible: %v", err)
		}
		_ = eligible

		regInUse, err := repo.RegionInUse(ctx, "NON_EXISTENT")
		if err != nil {
			t.Fatalf("RegionInUse: %v", err)
		}
		if regInUse {
			t.Errorf("RegionInUse(NON_EXISTENT) = true, want false")
		}

		areaActive, err := repo.AreaActive(ctx, areaID)
		if err != nil {
			t.Fatalf("AreaActive: %v", err)
		}
		if !areaActive {
			t.Errorf("AreaActive(%d) = false, want true", areaID)
		}

		findAct, err := repo.FindActive(ctx, db.FindActiveCashCountPicParams{
			PicRole: "EXECUTOR",
			UserID:  adminUserID,
			AreaID:  areaID,
		})
		if err != nil {
			t.Fatalf("FindActive: %v", err)
		}
		if !findAct {
			t.Errorf("FindActive = false, want true")
		}

		actPics, err := repo.ActivePics(ctx)
		if err != nil {
			t.Fatalf("ActivePics: %v", err)
		}
		if len(actPics) == 0 {
			t.Errorf("ActivePics returned 0 items")
		}

		actAreas, err := repo.ActiveAreas(ctx)
		if err != nil {
			t.Fatalf("ActiveAreas: %v", err)
		}
		if len(actAreas) == 0 {
			t.Errorf("ActiveAreas returned 0 items")
		}

		var adminUsername string
		_ = tx.QueryRow(ctx, "SELECT username FROM users WHERE id = $1", adminUserID).Scan(&adminUsername)

		uID, ok, err := repo.UserIDByUsername(ctx, adminUsername)
		if err != nil {
			t.Fatalf("UserIDByUsername: %v", err)
		}
		if !ok || uID != adminUserID {
			t.Errorf("UserIDByUsername(%s) = (%d, %v), want (%d, true)", adminUsername, uID, ok, adminUserID)
		}

		_, ok, err = repo.UserIDByUsername(ctx, "NON_EXISTENT_USER_"+tag)
		if err != nil {
			t.Fatalf("UserIDByUsername(non-existent): %v", err)
		}
		if ok {
			t.Errorf("UserIDByUsername(non-existent) = ok=true, want false")
		}
	})
}
