package approval

import (
	"context"
	"testing"
	"time"
)

// fakeAvailabilityRepository is an in-memory AvailabilityRepository for
// table-driven tests — no DB needed.
type fakeAvailabilityRepository struct {
	onLeave   map[int64]bool
	delegates map[int64]int64
}

func (f *fakeAvailabilityRepository) IsOnLeave(_ context.Context, userID int64, _ time.Time) (bool, error) {
	return f.onLeave[userID], nil
}

func (f *fakeAvailabilityRepository) FindActiveDelegate(_ context.Context, fromUserID int64, _ time.Time) (*int64, error) {
	to, ok := f.delegates[fromUserID]
	if !ok {
		return nil, nil
	}
	return &to, nil
}

func TestResolveEffectiveApprover(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		onLeave    map[int64]bool
		delegates  map[int64]int64
		approverID int64
		makerID    int64
		want       int64
		wantErr    bool
	}{
		{
			name:       "approver available: resolves to approver themselves",
			approverID: 10,
			makerID:    1,
			want:       10,
		},
		{
			name:       "approver on leave with active delegate: resolves to delegate",
			onLeave:    map[int64]bool{10: true},
			delegates:  map[int64]int64{10: 20},
			approverID: 10,
			makerID:    1,
			want:       20,
		},
		{
			name:       "approver on leave with no delegate: clear error",
			onLeave:    map[int64]bool{10: true},
			approverID: 10,
			makerID:    1,
			wantErr:    true,
		},
		{
			name:       "delegate is the maker: rejected",
			onLeave:    map[int64]bool{10: true},
			delegates:  map[int64]int64{10: 1},
			approverID: 10,
			makerID:    1,
			wantErr:    true,
		},
		{
			name:       "approver themselves is the maker: rejected",
			approverID: 1,
			makerID:    1,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeAvailabilityRepository{onLeave: tt.onLeave, delegates: tt.delegates}
			got, err := ResolveEffectiveApprover(context.Background(), repo, tt.approverID, tt.makerID, now)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("ResolveEffectiveApprover() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveEffectiveApprover() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ResolveEffectiveApprover() = %d, want %d", got, tt.want)
			}
		})
	}
}
