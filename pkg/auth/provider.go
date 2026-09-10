package auth

import "context"

// AuthIdentity is the result of successful authentication.
type AuthIdentity struct {
	UserID        int64
	Username      string
	Role          string
	IsKaryawan    bool
	VendorID      *int64
	SupervisorID  *int64 // nullable — reporting line (RBAC-Setup)
	ApprovalLevel *int32 // nullable — maker-checker level, independent of Role
}

// Provider abstracts credential verification.
// Implementations: LocalProvider (bcrypt), future LDAPProvider.
type Provider interface {
	Authenticate(ctx context.Context, username, password string) (*AuthIdentity, error)
	Supports(authSource string) bool
}
