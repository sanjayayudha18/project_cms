// Package rolemgmt implements the Role Management data-driven menu/feature
// permission layer (.kiro/specs/role-management): a catalog + role mapping
// read at runtime by RequirePermission, replacing hardcoded RequireRoles(...)
// for routes that opt into the dynamic model. See design.md for the full
// architecture and the documented Golden Rule #3 deviation (immediate-apply,
// audit-only, no maker-checker).
package rolemgmt

import "fmt"

// Sentinel errors. Handlers map these to HTTP status codes per design.md's
// error table (Req 2-4): ErrRoleNameConflict -> 409, ErrCatalogEntryNotFound
// -> 400, ErrRoleNotFound -> 404, ErrNotAuthorized -> 403.
var (
	ErrRoleNameConflict     = fmt.Errorf("role name already exists")
	ErrCatalogEntryNotFound = fmt.Errorf("one or more catalog entries do not exist")
	ErrRoleNotFound         = fmt.Errorf("role not found")
	ErrNotAuthorized        = fmt.Errorf("actor is not authorized for role management")
)

// ValidationError represents a structured field-level validation error.
// Mirrors service.ValidationError's shape (internal/service/atm_portal.go)
// so handlers can format both the same way.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}
