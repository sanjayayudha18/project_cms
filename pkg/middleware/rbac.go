package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/cimb-niaga/cms/pkg/auth"
)

type contextKey string

const authContextKey contextKey = "auth_context"

// AuthContext is injected into request context after token validation.
type AuthContext struct {
	UserID     int64
	Username   string
	Role       string
	IsKaryawan bool
	VendorID   *int64
}

// RequireAuth validates the Bearer token and injects AuthContext.
// Returns 401 if token is missing, malformed, expired, or invalid signature.
func RequireAuth(tokenService *auth.TokenService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeJSONError(w, http.StatusUnauthorized, "token_invalid", "Token tidak valid")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
				writeJSONError(w, http.StatusUnauthorized, "token_invalid", "Token tidak valid")
				return
			}

			tokenStr := parts[1]

			claims, err := tokenService.ValidateAccessToken(tokenStr)
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, "token_invalid", "Token tidak valid")
				return
			}

			authCtx := &AuthContext{
				UserID:     claims.UserID,
				Username:   claims.Username,
				Role:       claims.Role,
				IsKaryawan: claims.IsKaryawan,
				VendorID:   claims.VendorID,
			}

			ctx := context.WithValue(r.Context(), authContextKey, authCtx)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRoles checks if the authenticated user's role is in allowedRoles.
// Returns 403 if not permitted. Empty allowedRoles allows any authenticated user.
func RequireRoles(allowedRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCtx, ok := GetAuthContext(r.Context())
			if !ok {
				writeJSONError(w, http.StatusUnauthorized, "token_invalid", "Token tidak valid")
				return
			}

			if len(allowedRoles) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			for _, role := range allowedRoles {
				if strings.EqualFold(authCtx.Role, role) {
					next.ServeHTTP(w, r)
					return
				}
			}

			writeJSONError(w, http.StatusForbidden, "forbidden", "Anda tidak memiliki akses ke resource ini")
		})
	}
}

// GetAuthContext extracts AuthContext from the request context.
func GetAuthContext(ctx context.Context) (*AuthContext, bool) {
	authCtx, ok := ctx.Value(authContextKey).(*AuthContext)
	return authCtx, ok
}

// writeJSONError writes a JSON error response with the given status code.
func writeJSONError(w http.ResponseWriter, statusCode int, errorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   errorCode,
		"message": message,
	})
}
