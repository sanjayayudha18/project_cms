package repository

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// MasterDataChangeRepository is the primary-pool repository backing the
// generic master-data maker-checker staging table (plan.md Fase 2).
type MasterDataChangeRepository struct {
	queries *db.Queries
}

// NewMasterDataChangeRepository creates a MasterDataChangeRepository wrapping the given database connection.
func NewMasterDataChangeRepository(dbConn db.DBTX) *MasterDataChangeRepository {
	return &MasterDataChangeRepository{queries: db.New(dbConn)}
}

// FindPending returns the id of an existing pending change request for
// (entityType, entityID), or nil if there is none (T2.2 idempotency guard).
func (r *MasterDataChangeRepository) FindPending(ctx context.Context, entityType string, entityID int64) (*int64, error) {
	row, err := r.queries.FindPendingMasterDataChangeRequest(ctx, db.FindPendingMasterDataChangeRequestParams{
		EntityType: entityType,
		EntityID:   &entityID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &row.ID, nil
}

// Create inserts a new master_data_change_requests row (T2.1 Submit).
func (r *MasterDataChangeRepository) Create(ctx context.Context, arg db.CreateMasterDataChangeRequestParams) (db.MasterDataChangeRequest, error) {
	return r.queries.CreateMasterDataChangeRequest(ctx, arg)
}

// SetApprovalRequestID links a staged change request to the approval_requests
// row Orchestrator.SubmitForApproval created for it (T2.1).
func (r *MasterDataChangeRepository) SetApprovalRequestID(ctx context.Context, id, approvalRequestID int64) error {
	return r.queries.SetMasterDataChangeRequestApprovalID(ctx, db.SetMasterDataChangeRequestApprovalIDParams{
		ID:                id,
		ApprovalRequestID: &approvalRequestID,
	})
}

// GetByID returns a change request by id (T2.4 apply-on-approve hook).
func (r *MasterDataChangeRepository) GetByID(ctx context.Context, id int64) (db.MasterDataChangeRequest, error) {
	return r.queries.GetMasterDataChangeRequestByID(ctx, id)
}

// MarkApproved flips status to 'approved' once the approval chain's final
// step passes, before the apply attempt (T2.4). Committed immediately
// (non-transactional, same pool as everything else here) so it survives
// even if the apply transaction that follows rolls back.
func (r *MasterDataChangeRepository) MarkApproved(ctx context.Context, id int64) error {
	return r.queries.MarkMasterDataChangeRequestApproved(ctx, id)
}

// MarkApplyFailed records the apply error on a change request whose status
// stays 'approved' (T2.4: "approved-but-not-applied"). Committed
// immediately, independent of the apply transaction that failed.
func (r *MasterDataChangeRepository) MarkApplyFailed(ctx context.Context, id int64, errMsg string) error {
	return r.queries.MarkMasterDataChangeRequestApplyFailed(ctx, db.MarkMasterDataChangeRequestApplyFailedParams{
		ID:    id,
		Error: &errMsg,
	})
}

// MarkStale records that the entity changed since submit (T2.5): status
// flips to 'stale', the entity is left untouched (the apply transaction
// that detected this never wrote anything and gets rolled back by its
// caller).
func (r *MasterDataChangeRepository) MarkStale(ctx context.Context, id int64, errMsg string) error {
	return r.queries.MarkMasterDataChangeRequestStale(ctx, db.MarkMasterDataChangeRequestStaleParams{
		ID:    id,
		Error: &errMsg,
	})
}

// MarkRejected flips status to 'rejected' (T2.6). No entity mutation
// happens on reject, so this is the only write the reject path needs.
func (r *MasterDataChangeRepository) MarkRejected(ctx context.Context, id int64) error {
	return r.queries.MarkMasterDataChangeRequestRejected(ctx, id)
}

// ListByBatch returns every change request of an import batch, in id order
// (T5.4 batch apply/reject).
func (r *MasterDataChangeRepository) ListByBatch(ctx context.Context, batchID int64) ([]db.MasterDataChangeRequest, error) {
	return r.queries.ListMasterDataChangeRequestsByBatch(ctx, &batchID)
}

// List returns a page of change requests, filtered by entity_type/status
// (T2.7). ponytail: uses the same connection as everything else here for
// now; swap to the dbRead pool when DATABASE_REPLICA_URL wiring lands (same
// TODO convention as VendorAdminRepository/ATMAdminRepository).
func (r *MasterDataChangeRepository) List(ctx context.Context, arg db.ListMasterDataChangeRequestsParams) ([]db.MasterDataChangeRequest, error) {
	return r.queries.ListMasterDataChangeRequests(ctx, arg)
}

// Count returns the total number of change requests matching List's filters,
// without pagination (T2.7).
func (r *MasterDataChangeRepository) Count(ctx context.Context, arg db.CountMasterDataChangeRequestsParams) (int64, error) {
	return r.queries.CountMasterDataChangeRequests(ctx, arg)
}
