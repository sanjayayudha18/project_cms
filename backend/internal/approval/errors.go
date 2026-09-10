package approval

import "errors"

// Sentinel errors the orchestrator wraps its own errors with, so callers
// (the HTTP handler) can map them to specific status codes via errors.Is —
// same convention as service.ErrDsrFileNotFound etc. in internal/service.
var (
	// ErrNotAuthorized: actor is not the effective approver for the current step.
	ErrNotAuthorized = errors.New("actor is not authorized for this approval step")
	// ErrRequestNotPending: the request has no active pending step to act on.
	ErrRequestNotPending = errors.New("approval request is not pending")
	// ErrPolicyNotFound: no approval_policies row matches (document_type, amount).
	ErrPolicyNotFound = errors.New("no approval policy matches this document")
	// ErrDelegationOverlap: the delegation range overlaps an existing one for
	// the same from_user_id (approval_delegations_no_overlap, migration 025).
	ErrDelegationOverlap = errors.New("delegation range overlaps an existing delegation for this user")
)
