package approval

import (
	"context"
	"fmt"
	"time"
)

// AvailabilityRepository is the read access ResolveEffectiveApprover needs.
type AvailabilityRepository interface {
	// IsOnLeave reports whether userID has a leave window covering at.
	IsOnLeave(ctx context.Context, userID int64, at time.Time) (bool, error)
	// FindActiveDelegate returns the delegate covering at for fromUserID, or
	// nil if none is active.
	FindActiveDelegate(ctx context.Context, fromUserID int64, at time.Time) (*int64, error)
}

// ResolveEffectiveApprover returns who must actually act on a step assigned to
// approverID: approverID itself, or their active delegate if approverID is on
// leave. Rejects the result if it resolves to makerID (maker != checker).
func ResolveEffectiveApprover(ctx context.Context, repo AvailabilityRepository, approverID, makerID int64, at time.Time) (int64, error) {
	effective := approverID

	onLeave, err := repo.IsOnLeave(ctx, approverID, at)
	if err != nil {
		return 0, fmt.Errorf("check leave status for user %d: %w", approverID, err)
	}
	if onLeave {
		delegate, err := repo.FindActiveDelegate(ctx, approverID, at)
		if err != nil {
			return 0, fmt.Errorf("find delegate for user %d: %w", approverID, err)
		}
		if delegate == nil {
			return 0, fmt.Errorf("approver %d is on leave and has no active delegate", approverID)
		}
		effective = *delegate
	}

	if effective == makerID {
		return 0, fmt.Errorf("resolved approver %d is the maker: maker cannot approve their own request", effective)
	}

	return effective, nil
}
