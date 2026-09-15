package service

import (
	"errors"
	"testing"

	"pgregory.net/rapid"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Feature: replenishment-request-enhancements, Property 8: Himpunan transisi state machine
// Feature: replenishment-request-enhancements, Property 10: Otorisasi cancel-approved hanya Checker
//
// Both pure, no DB: nextState and checkActor only ever touch an in-memory
// db.VendorRequest struct (never a query), so neither needs Postgres — the
// "integration" label Property 10 carries in tasks.md assumed exercising the
// full Cancel() flow (task 8, not yet implemented); the authorization
// decision itself lives entirely in checkActor and is reachable directly.

var allVendorRequestStatuses = []string{
	"draft", "pending_approval", "approved", "rejected",
	"processing", "completed", "failed", "cancelled",
}

var allVendorRequestActions = []action{actionSubmit, actionApprove, actionReject, actionRevise, actionCancel}

// wantTransitions is the spec, kept independent of the production
// `transitions` var in vendor_request.go so this test can actually catch a
// regression there instead of trivially comparing a var to itself.
var wantTransitions = map[string]map[action]string{
	"draft":            {actionSubmit: "pending_approval", actionCancel: "cancelled"},
	"pending_approval": {actionApprove: "approved", actionReject: "rejected", actionCancel: "cancelled"},
	"rejected":         {actionRevise: "draft"},
	"approved":         {actionCancel: "cancelled"},
}

// TestProperty8_StateMachineTransitionSet checks nextState against every
// (status, action) pair: the allowed set matches wantTransitions exactly
// (including the new approved--cancel-->cancelled edge, Req 3.1/6.4), and
// every other pair is illegal — ok=false and the target status untouched
// ("" — nextState never returns a partially-filled target on rejection).
func TestProperty8_StateMachineTransitionSet(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		status := rapid.SampledFrom(allVendorRequestStatuses).Draw(rt, "status")
		a := rapid.SampledFrom(allVendorRequestActions).Draw(rt, "action")

		to, ok := nextState(status, a)
		wantTo, wantOk := wantTransitions[status][a]

		if ok != wantOk {
			rt.Fatalf("nextState(%q, %q): ok = %v, want %v", status, a, ok, wantOk)
		}
		if to != wantTo {
			rt.Fatalf("nextState(%q, %q) = %q, want %q", status, a, to, wantTo)
		}
		if !ok && to != "" {
			rt.Fatalf("nextState(%q, %q): illegal transition returned non-empty target %q", status, a, to)
		}
	})
}

// TestProperty10_CancelApprovedAuthorizationCheckerOnly checks checkActor's
// approved-status branch of actionCancel: any Checker role is allowed
// (including a Checker who is also the creator — no four-eyes on a direct
// cancellation), any non-Checker role is denied with ErrNotChecker.
func TestProperty10_CancelApprovedAuthorizationCheckerOnly(t *testing.T) {
	candidateRoles := []string{
		"ADMIN", "ATM-SPV", "BRANCH-ATM-SPV", // checkers
		"ATM-USER", "BRANCH-ATM-USER", "VENDOR", "UNKNOWN-ROLE", // non-checkers
	}

	rapid.Check(t, func(rt *rapid.T) {
		role := rapid.SampledFrom(candidateRoles).Draw(rt, "role")
		actorID := rapid.Int64Range(1, 1000).Draw(rt, "actorID")
		creatorSameAsActor := rapid.Bool().Draw(rt, "creatorSameAsActor")
		creatorID := actorID
		if !creatorSameAsActor {
			creatorID = actorID + 1
		}

		actor := Actor{UserID: actorID, Role: role}
		req := db.VendorRequest{Status: "approved", CreatedBy: creatorID}

		err := checkActor(actor, req, actionCancel)

		if isChecker(role) {
			if err != nil {
				rt.Fatalf("checker role %q denied cancel-approved: %v", role, err)
			}
			return
		}
		if err == nil {
			rt.Fatalf("non-checker role %q was allowed to cancel an approved request", role)
		}
		if !errors.Is(err, ErrNotChecker) {
			rt.Fatalf("non-checker role %q: got %v, want ErrNotChecker", role, err)
		}
	})
}
