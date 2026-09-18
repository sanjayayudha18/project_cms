package rolemgmt

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/cimb-niaga/cms/pkg/middleware"
)

// ReplicaReader is the read surface PermissionEvaluator needs. *Repository
// satisfies this automatically; narrow so tests can fake it without a DB.
type ReplicaReader interface {
	HasPermission(ctx context.Context, roleName, featureKey string) (bool, error)
}

// PermissionEvaluator answers runtime permission checks for Dynamic_Route
// (Req 1.3, 5.3, 5.5): every decision reads role_permissions fresh via the
// replica pool, with no cache and no hardcoded role list (design.md "Caching
// consideration" -- YAGNI until profiling shows a hot path; a permission
// change is visible on the very next evaluation, satisfying Req 5.5's "no
// restart required").
type PermissionEvaluator struct {
	repo ReplicaReader
}

// NewPermissionEvaluator creates a PermissionEvaluator with the given dependency.
func NewPermissionEvaluator(repo ReplicaReader) *PermissionEvaluator {
	return &PermissionEvaluator{repo: repo}
}

// HasPermission reports whether roleName is mapped to the catalog entry with
// the given featureKey. Decision comes purely from role_permissions -- never
// consults a hardcoded role list (Req 5.3, design property 7).
func (e *PermissionEvaluator) HasPermission(ctx context.Context, roleName, featureKey string) (bool, error) {
	return e.repo.HasPermission(ctx, roleName, featureKey)
}

// RequirePermission is chi middleware for a Dynamic_Route: it allows the
// request through only if the authenticated user's role is mapped to
// featureKey in role_permissions (Req 1.3, 5.3), returning 403 flat JSON
// otherwise. This is separate from -- and does not alter -- the legacy
// static pkg/middleware.RequireRoles(...) guard used elsewhere (Req 5.2);
// existing routes keep using that unchanged.
func RequirePermission(e *PermissionEvaluator, featureKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCtx, ok := middleware.GetAuthContext(r.Context())
			if !ok {
				writeJSONError(w, http.StatusUnauthorized, "token_invalid", "Token tidak valid")
				return
			}

			allowed, err := e.HasPermission(r.Context(), authCtx.Role, featureKey)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan")
				return
			}
			if !allowed {
				writeJSONError(w, http.StatusForbidden, "forbidden", "Anda tidak memiliki akses ke resource ini")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// writeJSONError writes a flat {"error","message"} JSON error response,
// mirroring pkg/middleware.writeJSONError's shape (unexported there, so
// duplicated here rather than exported across package boundaries).
func writeJSONError(w http.ResponseWriter, statusCode int, errorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":   errorCode,
		"message": message,
	})
}
