package notification

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Repository backs /api/v1/notifications. Every method is scoped to the
// caller's own user id. List/UnreadCount read the replica (polled every 60 s
// by both portals); MarkRead/MarkAllRead write the primary (CLAUDE.md Sec 6).
type Repository struct {
	db     *db.Queries // primary
	dbRead *db.Queries // replica
}

// NewRepository creates a Repository. Pass the primary pool for both
// arguments when no replica is configured.
func NewRepository(primary, replica db.DBTX) *Repository {
	return &Repository{db: db.New(primary), dbRead: db.New(replica)}
}

// List returns one page of the user's notifications (newest first) and the
// total matching count.
func (r *Repository) List(ctx context.Context, userID int64, unreadOnly bool, page, pageSize int32) ([]db.ListNotificationsForUserRow, int64, error) {
	rows, err := r.dbRead.ListNotificationsForUser(ctx, db.ListNotificationsForUserParams{
		UserID:     userID,
		UnreadOnly: unreadOnly,
		PageLimit:  pageSize,
		PageOffset: (page - 1) * pageSize,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list notifications: %w", err)
	}
	total, err := r.dbRead.CountNotificationsForUser(ctx, db.CountNotificationsForUserParams{UserID: userID, UnreadOnly: unreadOnly})
	if err != nil {
		return nil, 0, fmt.Errorf("count notifications: %w", err)
	}
	return rows, total, nil
}

// UnreadCount returns how many of the user's notifications are unread.
func (r *Repository) UnreadCount(ctx context.Context, userID int64) (int64, error) {
	return r.dbRead.CountUnreadNotifications(ctx, userID)
}

// MarkRead marks one of the user's notifications read. found=false means it
// does not exist or belongs to someone else (handler answers 404 for both).
// Already-read rows still report found=true (idempotent).
func (r *Repository) MarkRead(ctx context.Context, id, userID int64) (bool, error) {
	_, err := r.db.MarkNotificationRead(ctx, db.MarkNotificationReadParams{ID: id, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("mark notification read: %w", err)
	}
	return true, nil
}

// MarkAllRead marks every unread notification of the user read and returns
// how many changed.
func (r *Repository) MarkAllRead(ctx context.Context, userID int64) (int64, error) {
	return r.db.MarkAllNotificationsRead(ctx, userID)
}
