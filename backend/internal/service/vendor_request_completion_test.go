package service

import (
	"context"
	"errors"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

func TestCompletionTransitions(t *testing.T) {
	cases := []struct {
		from string
		a    action
		to   string
		ok   bool
	}{
		{"approved", actionSubmitCompletion, "completion_pending", true},
		{"completion_pending", actionApproveCompletion, "completed", true},
		{"completion_pending", actionRejectCompletion, "approved", true},
		{"completion_pending", actionCancel, "", false}, // FR1.4
		{"approved", actionCancel, "cancelled", true},   // unchanged
		{"pending_approval", actionSubmitCompletion, "", false},
		{"completed", actionApproveCompletion, "", false}, // double approve
		{"draft", actionSubmitCompletion, "", false},
	}
	for _, c := range cases {
		to, ok := nextState(c.from, c.a)
		if ok != c.ok || to != c.to {
			t.Errorf("nextState(%s, %s) = (%q, %v), want (%q, %v)", c.from, c.a, to, ok, c.to, c.ok)
		}
	}
}

func TestCheckCompletionActor(t *testing.T) {
	maker := int64(10)
	req := db.VendorRequest{CreatedBy: 1, CompletionSubmittedBy: &maker}
	cases := []struct {
		name  string
		actor Actor
		a     action
		want  error
	}{
		{"ATM-USER submits", Actor{UserID: 10, Role: "ATM-USER"}, actionSubmitCompletion, nil},
		{"BRANCH-ATM-USER submits", Actor{UserID: 10, Role: "branch-atm-user"}, actionSubmitCompletion, nil},
		{"SPV cannot submit", Actor{UserID: 10, Role: "ATM-SPV"}, actionSubmitCompletion, ErrNotAuthorized},
		{"SPV approves", Actor{UserID: 20, Role: "ATM-SPV"}, actionApproveCompletion, nil},
		{"ATM-USER cannot approve", Actor{UserID: 20, Role: "ATM-USER"}, actionApproveCompletion, ErrNotChecker},
		{"reporter cannot approve own report", Actor{UserID: 10, Role: "ADMIN"}, actionApproveCompletion, ErrSelfApproval},
		{"reporter cannot reject own report", Actor{UserID: 10, Role: "ADMIN"}, actionRejectCompletion, ErrSelfApproval},
		{"order creator may approve the report", Actor{UserID: 1, Role: "BRANCH-ATM-SPV"}, actionApproveCompletion, nil},
	}
	for _, c := range cases {
		if got := checkCompletionActor(c.actor, req, c.a); !errors.Is(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestValidateCompletionResults(t *testing.T) {
	terms := []string{"T1", "T2"}
	ok := []CompletionResultInput{{"T1", "success"}, {"T2", "failed"}}
	if err := validateCompletionResults(terms, ok); err != nil {
		t.Fatalf("valid payload rejected: %v", err)
	}
	bad := map[string][]CompletionResultInput{
		"empty":          nil,
		"missing T2":     {{"T1", "success"}},
		"extra terminal": {{"T1", "success"}, {"T2", "success"}, {"T3", "success"}},
		"unknown":        {{"T1", "success"}, {"T9", "success"}},
		"duplicate":      {{"T1", "success"}, {"T1", "failed"}},
		"bad result":     {{"T1", "success"}, {"T2", "done"}},
	}
	for name, in := range bad {
		var ve *ValidationError
		if err := validateCompletionResults(terms, in); !errors.As(err, &ve) {
			t.Errorf("%s: want ValidationError, got %v", name, err)
		}
	}
}

func TestVisitSisa(t *testing.T) {
	cases := []struct{ remaining, sisa, over int32 }{
		{5, 5, 0}, {4, 4, 0}, {0, 0, 0}, {-1, 0, 1}, {-3, 0, 3},
	}
	for _, c := range cases {
		s, o := visitSisa(c.remaining)
		if s != c.sisa || o != c.over {
			t.Errorf("visitSisa(%d) = (%d,%d), want (%d,%d)", c.remaining, s, o, c.sisa, c.over)
		}
	}
}

// review N1: an empty reject-report reason is its own sentinel so the handler
// names the body field it actually came from ("reason", not "rejection_reason").
func TestRejectCompletion_EmptyReason(t *testing.T) {
	svc := &VendorRequestService{} // validation runs before any DB access
	if _, err := svc.RejectCompletion(context.Background(), Actor{UserID: 2, Role: "ATM-SPV"}, 1, "   "); !errors.Is(err, ErrCompletionReasonEmpty) {
		t.Fatalf("err = %v, want ErrCompletionReasonEmpty", err)
	}
}
