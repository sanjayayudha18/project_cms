// Package approval resolves maker-checker approval routing: who must approve
// (the supervisor chain, up to a required level) and who actually can (the
// leave/delegation fallback). No HTTP, no orchestration — pure lookups.
package approval

import (
	"context"
	"fmt"
)

// ApproverInfo is one user's position in the reporting-line hierarchy.
type ApproverInfo struct {
	UserID        int64
	SupervisorID  *int64
	ApprovalLevel *int32
}

// ChainRepository is the read access BuildChain needs.
type ChainRepository interface {
	GetApproverInfo(ctx context.Context, userID int64) (ApproverInfo, error)
}

// BuildChain walks up the supervisor_id chain from makerID, collecting each
// supervisor that has an approval_level set (supervisors without a level are
// skipped — they're not approvers), until one is found whose level is >=
// requiredLevel. Returns an error if the chain reaches the top (no supervisor)
// before that happens.
func BuildChain(ctx context.Context, repo ChainRepository, makerID int64, requiredLevel int32) ([]ApproverInfo, error) {
	var chain []ApproverInfo
	currentID := makerID

	for {
		current, err := repo.GetApproverInfo(ctx, currentID)
		if err != nil {
			return nil, fmt.Errorf("load approval info for user %d: %w", currentID, err)
		}
		if current.SupervisorID == nil {
			return nil, fmt.Errorf("approval chain incomplete: reached top of hierarchy at user %d before required level %d", currentID, requiredLevel)
		}

		supervisor, err := repo.GetApproverInfo(ctx, *current.SupervisorID)
		if err != nil {
			return nil, fmt.Errorf("load approval info for user %d: %w", *current.SupervisorID, err)
		}

		if supervisor.ApprovalLevel != nil {
			chain = append(chain, supervisor)
			if *supervisor.ApprovalLevel >= requiredLevel {
				return chain, nil
			}
		}

		currentID = supervisor.UserID
	}
}
