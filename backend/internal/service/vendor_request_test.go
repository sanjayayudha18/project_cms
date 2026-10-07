package service

import (
	"testing"
	"time"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Property 1 (design.md): only valid transitions mutate state.
func TestNextState(t *testing.T) {
	tests := []struct {
		name   string
		status string
		action action
		want   string
		wantOK bool
	}{
		{"draft submit", "draft", actionSubmit, "pending_approval", true},
		{"draft cancel", "draft", actionCancel, "cancelled", true},
		{"pending approve", "pending_approval", actionApprove, "approved", true},
		{"pending reject", "pending_approval", actionReject, "rejected", true},
		{"pending cancel", "pending_approval", actionCancel, "cancelled", true},
		{"rejected revise", "rejected", actionRevise, "draft", true},
		{"draft approve invalid", "draft", actionApprove, "", false},
		{"draft reject invalid", "draft", actionReject, "", false},
		{"draft revise invalid", "draft", actionRevise, "", false},
		{"approved submit invalid", "approved", actionSubmit, "", false},
		{"cancelled submit invalid", "cancelled", actionSubmit, "", false},
		{"unknown status invalid", "processing", actionSubmit, "", false},
		{"rejected submit invalid", "rejected", actionSubmit, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := nextState(tt.status, tt.action)
			if ok != tt.wantOK {
				t.Fatalf("nextState(%q, %q) ok = %v, want %v", tt.status, tt.action, ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Fatalf("nextState(%q, %q) = %q, want %q", tt.status, tt.action, got, tt.want)
			}
		})
	}
}

// Property 2/3 (design.md): four-eyes principle and actor-role gating,
// including the resolved cancel union rule (creator OR checker on
// pending_approval; creator only on draft).
func TestCheckActor(t *testing.T) {
	const (
		creatorID = int64(1)
		checkerID = int64(2)
		otherID   = int64(3)
	)
	draftReq := db.VendorRequest{CreatedBy: creatorID, Status: "draft"}
	pendingReq := db.VendorRequest{CreatedBy: creatorID, Status: "pending_approval"}

	tests := []struct {
		name    string
		actor   Actor
		req     db.VendorRequest
		action  action
		wantErr error
	}{
		{"submit by creator ok", Actor{UserID: creatorID, Role: "ATM-USER"}, draftReq, actionSubmit, nil},
		{"submit by non-creator rejected", Actor{UserID: otherID, Role: "ATM-USER"}, draftReq, actionSubmit, ErrNotCreator},
		{"revise by creator ok", Actor{UserID: creatorID, Role: "ATM-USER"}, draftReq, actionRevise, nil},
		{"revise by non-creator rejected", Actor{UserID: otherID, Role: "ATM-USER"}, draftReq, actionRevise, ErrNotCreator},

		{"approve by checker ok", Actor{UserID: checkerID, Role: "ATM-SPV"}, pendingReq, actionApprove, nil},
		{"approve by non-checker rejected", Actor{UserID: checkerID, Role: "ATM-USER"}, pendingReq, actionApprove, ErrNotChecker},
		{"approve by creator (self) rejected", Actor{UserID: creatorID, Role: "ATM-SPV"}, pendingReq, actionApprove, ErrSelfApproval},
		{"reject by checker ok", Actor{UserID: checkerID, Role: "BRANCH-ATM-SPV"}, pendingReq, actionReject, nil},
		{"reject by creator (self) rejected", Actor{UserID: creatorID, Role: "ATM-SPV"}, pendingReq, actionReject, ErrSelfApproval},
		{"admin approve non-creator ok", Actor{UserID: checkerID, Role: "ADMIN"}, pendingReq, actionApprove, nil},
		{"admin approve own request rejected (four-eyes still applies)", Actor{UserID: creatorID, Role: "ADMIN"}, pendingReq, actionApprove, ErrSelfApproval},

		{"cancel draft by creator ok", Actor{UserID: creatorID, Role: "ATM-USER"}, draftReq, actionCancel, nil},
		{"cancel draft by checker rejected (draft is creator-only)", Actor{UserID: checkerID, Role: "ATM-SPV"}, draftReq, actionCancel, ErrNotCreator},
		{"cancel pending by creator ok (union rule)", Actor{UserID: creatorID, Role: "ATM-USER"}, pendingReq, actionCancel, nil},
		{"cancel pending by checker ok (union rule)", Actor{UserID: checkerID, Role: "ATM-SPV"}, pendingReq, actionCancel, nil},
		{"cancel pending by unrelated maker rejected", Actor{UserID: otherID, Role: "ATM-USER"}, pendingReq, actionCancel, ErrNotAuthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkActor(tt.actor, tt.req, tt.action)
			if err != tt.wantErr {
				t.Fatalf("checkActor(%+v, status=%s, %q) = %v, want %v", tt.actor, tt.req.Status, tt.action, err, tt.wantErr)
			}
		})
	}
}

// Req 4.6/8.6: duplicate (terminal_id, periode_pred, denom) within one
// payload is rejected regardless of amount_replenish.
func TestValidateItemsPayload_Duplicates(t *testing.T) {
	date := mustParseDate(t, "2026-09-12")
	items := []ItemInput{
		{TerminalID: "ATM001", PeriodePred: date, Denom: 100000, AmountReplenish: 1},
		{TerminalID: "ATM001", PeriodePred: date, Denom: 100000, AmountReplenish: 2},
	}
	if err := validateItemsPayload(items, "", 0); err != ErrDuplicateItems {
		t.Fatalf("want ErrDuplicateItems, got %v", err)
	}
}

func TestValidateItemsPayload_Empty(t *testing.T) {
	if err := validateItemsPayload(nil, "", 0); err != ErrEmptyItems {
		t.Fatalf("want ErrEmptyItems, got %v", err)
	}
}

func TestValidateItemsPayload_MaxItems(t *testing.T) {
	date := mustParseDate(t, "2026-09-12")
	items := []ItemInput{
		{TerminalID: "ATM001", PeriodePred: date, Denom: 100000, AmountReplenish: 1},
		{TerminalID: "ATM002", PeriodePred: date, Denom: 100000, AmountReplenish: 1},
	}
	if err := validateItemsPayload(items, "", 1); err == nil {
		t.Fatal("want a validation error for exceeding maxItems, got nil")
	}
	if err := validateItemsPayload(items, "", 0); err != nil {
		t.Fatalf("maxItems=0 means no cap, want nil, got %v", err)
	}
}

func mustParseDate(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("parse date %q: %v", s, err)
	}
	return parsed
}
