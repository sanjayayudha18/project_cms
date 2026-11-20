package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// CashCountPicAdminRepository backs the "Mapping PIC cash count" endpoints (ScDD 5.1):
// list on the replica, submit-time lookups on the primary. Mutations run only in the applier.
type CashCountPicAdminRepository struct {
	queries     *db.Queries
	readQueries *db.Queries
}

// NewCashCountPicAdminRepository wraps the primary (dbConn) and replica (readConn) pools.
func NewCashCountPicAdminRepository(dbConn, readConn db.DBTX) *CashCountPicAdminRepository {
	return &CashCountPicAdminRepository{queries: db.New(dbConn), readQueries: db.New(readConn)}
}

// List returns PIC mappings filtered by role ("" = both) and status (replica).
func (r *CashCountPicAdminRepository) List(ctx context.Context, arg db.ListCashCountPicsAdminParams) ([]db.ListCashCountPicsAdminRow, error) {
	return r.readQueries.ListCashCountPicsAdmin(ctx, arg)
}

// GetByID returns a mapping incl. disabled rows; nil, nil if absent.
func (r *CashCountPicAdminRepository) GetByID(ctx context.Context, id int64) (*db.GetCashCountPicAdminByIDRow, error) {
	row, err := r.queries.GetCashCountPicAdminByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// UserEligible reports whether the user is active, ldap and holds the role required for picRole.
func (r *CashCountPicAdminRepository) UserEligible(ctx context.Context, userID int64, picRole string) (bool, error) {
	_, err := r.queries.GetCashCountPicEligibleUser(ctx, db.GetCashCountPicEligibleUserParams{UserID: userID, PicRole: picRole})
	return picRowFound(err)
}

// RegionInUse reports whether an active vendor_branches row uses the region_code.
func (r *CashCountPicAdminRepository) RegionInUse(ctx context.Context, regionCode string) (bool, error) {
	return r.queries.CashCountRegionCodeInUse(ctx, regionCode)
}

// AreaActive reports whether the cash count area exists and is active.
func (r *CashCountPicAdminRepository) AreaActive(ctx context.Context, areaID int64) (bool, error) {
	_, err := r.queries.GetActiveCashCountAreaID(ctx, areaID)
	return picRowFound(err)
}

// FindActive reports whether another active mapping has the same (role, user, region/area).
// RegionCode "" / AreaID 0 when not applicable.
func (r *CashCountPicAdminRepository) FindActive(ctx context.Context, arg db.FindActiveCashCountPicParams) (bool, error) {
	_, err := r.queries.FindActiveCashCountPic(ctx, arg)
	return picRowFound(err)
}

func picRowFound(err error) (bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
