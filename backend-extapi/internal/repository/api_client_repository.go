package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/cimb-niaga/cms/backend-extapi/internal/extauth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ApiClientRepository reads api_clients. Always the PRIMARY pool, so a client
// that was just disabled or rotated is rejected immediately (Req 1.5, 4.2).
type ApiClientRepository struct{ q *Queries }

// NewApiClientRepository wraps the primary pool.
func NewApiClientRepository(primary DBTX) *ApiClientRepository {
	return &ApiClientRepository{q: New(primary)}
}

// GetByClientID returns nil, nil when the client does not exist.
func (r *ApiClientRepository) GetByClientID(ctx context.Context, clientID string) (*extauth.Client, error) {
	row, err := r.q.GetApiClientByClientID(ctx, clientID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get api client: %w", err)
	}
	return &extauth.Client{
		ID:                row.ID,
		ClientID:          row.ClientID,
		Name:              row.Name,
		SecretHash:        row.SecretHash,
		CredentialVersion: row.CredentialVersion,
		Scopes:            row.Scopes,
		AssignableRoles:   row.AssignableRoles,
		AllowedCIDRs:      row.AllowedCidrs,
		ServiceUserID:     row.ServiceUserID,
		IsActive:          row.IsActive,
	}, nil
}

// TxRunner runs fn in one primary-pool transaction: commit when fn returns
// nil, rollback otherwise. The extuser Store (task 6.1) is built on the tx.
type TxRunner struct{ pool *pgxpool.Pool }

// NewTxRunner wraps the primary pool.
func NewTxRunner(pool *pgxpool.Pool) *TxRunner { return &TxRunner{pool: pool} }

// Run executes fn inside a transaction.
func (t *TxRunner) Run(ctx context.Context, fn func(tx DBTX) error) error {
	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	if err := fn(tx); err != nil {
		// Rollback on a fresh context: ctx may be the reason fn failed.
		_ = tx.Rollback(context.WithoutCancel(ctx))
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
