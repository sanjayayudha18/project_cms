// Package rolemgmt implements the Role Management data-driven menu/feature
// permission layer (.kiro/specs/role-management): a catalog + role mapping
// read at runtime by RequirePermission, replacing hardcoded RequireRoles(...)
// for routes that opt into the dynamic model. See design.md for the full
// architecture and the documented Golden Rule #3 deviation (immediate-apply,
// audit-only, no maker-checker).
package rolemgmt

import (
	"fmt"
	"strings"
)

// Sentinel errors. Handlers map these to HTTP status codes per design.md's
// error table (Req 2-4): ErrRoleNameConflict -> 409, ErrCatalogEntryNotFound
// -> 400, ErrRoleNotFound -> 404, ErrNotAuthorized -> 403.
var (
	ErrRoleNameConflict     = fmt.Errorf("role name already exists")
	ErrCatalogEntryNotFound = fmt.Errorf("one or more catalog entries do not exist")
	ErrRoleNotFound         = fmt.Errorf("role not found")
	ErrNotAuthorized        = fmt.Errorf("actor is not authorized for role management")
	ErrRoleNotAssignable    = fmt.Errorf("role is not assignable to this screen")
)

// RoleNotAssignableError is returned when a permission update grants a
// screen key to a role outside that key's menu_features.assignable_roles
// (spec permission-catalog-coverage Req 3.6). Handlers map it to 422
// role_not_assignable; errors.Is(err, ErrRoleNotAssignable) matches it.
type RoleNotAssignableError struct {
	Label string
	Roles []string
}

func (e *RoleNotAssignableError) Error() string {
	return fmt.Sprintf("Layar %q hanya untuk role: %s", e.Label, strings.Join(e.Roles, ", "))
}

// Is lets errors.Is(err, ErrRoleNotAssignable) match.
func (e *RoleNotAssignableError) Is(target error) bool { return target == ErrRoleNotAssignable }

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
