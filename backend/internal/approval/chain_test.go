package approval

import (
	"context"
	"testing"
)

// fakeChainRepository is an in-memory ChainRepository for table-driven tests —
// no DB needed, per the project's "mock repos for unit tests" convention.
type fakeChainRepository struct {
	users map[int64]ApproverInfo
}

func (f *fakeChainRepository) GetApproverInfo(_ context.Context, userID int64) (ApproverInfo, error) {
	info, ok := f.users[userID]
	if !ok {
		return ApproverInfo{}, errNotFound(userID)
	}
	return info, nil
}

type errNotFound int64

func (e errNotFound) Error() string { return "user not found" }

func int32p(v int32) *int32 { return &v }
func int64p(v int64) *int64 { return &v }

func TestBuildChain(t *testing.T) {
	tests := []struct {
		name          string
		users         map[int64]ApproverInfo
		makerID       int64
		requiredLevel int32
		wantLevels    []int32
		wantErr       bool
	}{
		{
			name: "reaches required level in one hop",
			users: map[int64]ApproverInfo{
				1: {UserID: 1, SupervisorID: int64p(2)},
				2: {UserID: 2, ApprovalLevel: int32p(3)},
			},
			makerID:       1,
			requiredLevel: 3,
			wantLevels:    []int32{3},
		},
		{
			name: "climbs multiple levels, skipping supervisors with no approval_level",
			users: map[int64]ApproverInfo{
				1: {UserID: 1, SupervisorID: int64p(2)},
				2: {UserID: 2, SupervisorID: int64p(3)}, // no approval_level: skipped
				3: {UserID: 3, SupervisorID: int64p(4), ApprovalLevel: int32p(2)},
				4: {UserID: 4, ApprovalLevel: int32p(3)},
			},
			makerID:       1,
			requiredLevel: 3,
			wantLevels:    []int32{2, 3},
		},
		{
			name: "chain incomplete: reaches top of hierarchy before required level",
			users: map[int64]ApproverInfo{
				1: {UserID: 1, SupervisorID: int64p(2)},
				2: {UserID: 2, ApprovalLevel: int32p(2)}, // no supervisor, level 2 < required 3
			},
			makerID:       1,
			requiredLevel: 3,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeChainRepository{users: tt.users}
			chain, err := BuildChain(context.Background(), repo, tt.makerID, tt.requiredLevel)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("BuildChain() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("BuildChain() unexpected error: %v", err)
			}

			if len(chain) != len(tt.wantLevels) {
				t.Fatalf("BuildChain() chain length = %d, want %d (chain=%+v)", len(chain), len(tt.wantLevels), chain)
			}
			for i, want := range tt.wantLevels {
				if chain[i].ApprovalLevel == nil || *chain[i].ApprovalLevel != want {
					t.Errorf("BuildChain() chain[%d].ApprovalLevel = %v, want %d", i, chain[i].ApprovalLevel, want)
				}
			}
		})
	}
}
