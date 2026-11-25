package rolemgmt

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/cimb-niaga/cms/pkg/middleware"
)

// ReplicaReader is the read surface PermissionEvaluator needs. *Repository
// satisfies this automatically; narrow so tests can fake it without a DB.
type ReplicaReader interface {
	HasAnyPermission(ctx context.Context, roleName string, keys []string) (bool, error)
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
	return e.HasAnyPermission(ctx, roleName, []string{featureKey})
}

// HasAnyPermission reports whether roleName is mapped to at least one of keys
// (one replica query). It does not apply the always-on or internal-role rules;
// RequirePermission does.
func (e *PermissionEvaluator) HasAnyPermission(ctx context.Context, roleName string, keys []string) (bool, error) {
	return e.repo.HasAnyPermission(ctx, roleName, keys)
}

// RequirePermission is chi middleware for a screen mount (spec
// permission-catalog-coverage Req 3.1, 7.1-7.4, 8.1). The request passes when
// the caller's role holds ANY of keys:
//   - non-internal role (VENDOR-*, EXT-CLIENT) -> 403, no query;
//   - internal role and one of keys is an Always_On_Key -> allowed, no query;
//   - otherwise one replica query; no match -> 403;
//   - query error -> 500 and slog.Error with role + keys (fail closed).
//
// The 401/403 bodies match pkg/middleware.RequireRoles. It panics when called
// with no keys, so a bad mount fails at startup instead of at request time.
func RequirePermission(e *PermissionEvaluator, keys ...string) func(http.Handler) http.Handler {
	if len(keys) == 0 {
		panic("rolemgmt.RequirePermission: at least one key is required")
	}
	alwaysOn := false
	for _, k := range keys {
		if IsAlwaysOn(k) {
			alwaysOn = true
			break
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCtx, ok := middleware.GetAuthContext(r.Context())
			if !ok {
				writeJSONError(w, http.StatusUnauthorized, "token_invalid", "Token tidak valid")
				return
			}
			if !IsInternalRole(authCtx.Role) {
				writeForbidden(w)
				return
			}
			if alwaysOn {
				next.ServeHTTP(w, r)
				return
			}

			allowed, err := e.HasAnyPermission(r.Context(), authCtx.Role, keys)
			if err != nil {
				slog.ErrorContext(r.Context(), "permission check failed",
					"role", authCtx.Role, "keys", keys, "error", err)
				writeJSONError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan")
				return
			}
			if !allowed {
				writeForbidden(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func writeForbidden(w http.ResponseWriter) {
	writeJSONError(w, http.StatusForbidden, "forbidden", "Anda tidak memiliki akses ke resource ini")
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
