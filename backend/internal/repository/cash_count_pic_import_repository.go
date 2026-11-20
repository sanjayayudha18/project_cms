package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Reads for the replace-all CSV import of mapping PIC (ScDD 5.1 task 9.1). All on the
// PRIMARY: the import validates a write flow, so replica lag must not hide a fresh mapping.

// ActivePics returns every active mapping with its username and area code.
func (r *CashCountPicAdminRepository) ActivePics(ctx context.Context) ([]db.ListCashCountPicsAdminRow, error) {
	return r.queries.ListCashCountPicsAdmin(ctx, db.ListCashCountPicsAdminParams{PicRole: "", Status: "active"})
}

// ActiveAreas returns the active cash count areas (code -> id lookup for the file).
func (r *CashCountPicAdminRepository) ActiveAreas(ctx context.Context) ([]db.ListCashCountAreasAdminRow, error) {
	return r.queries.ListCashCountAreasAdmin(ctx, "active")
}

// UserIDByUsername resolves a username (any status; eligibility is checked separately).
func (r *CashCountPicAdminRepository) UserIDByUsername(ctx context.Context, username string) (int64, bool, error) {
	id, err := r.queries.FindUserAdminByUsername(ctx, username)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}
