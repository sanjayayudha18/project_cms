package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/auth"
	"github.com/cimb-niaga/cms/backend/internal/rolemgmt"
	"github.com/cimb-niaga/cms/backend/internal/vendorrole"
	"github.com/cimb-niaga/cms/pkg/middleware"

	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
)

// ChangePasswordService is the subset of auth.ChangePasswordService the
// handler needs (mirrors the ApprovalOrchestrator/ApprovalReader pattern in
// approval_handler.go — a narrow interface so handler tests can fake it
// without a real DB).
type ChangePasswordService interface {
	ChangePassword(ctx context.Context, userID int64, req auth.ChangePasswordRequest, actorIP string) error
}

// AuthHandler handles authentication-related HTTP endpoints.
type AuthHandler struct {
	authService       *auth.Service
	tokenService      *pkgauth.TokenService
	userRepo          pkgauth.UserRepository
	rateLimiter       *middleware.RateLimiter
	changePasswordSvc ChangePasswordService
	permKeys          PermissionKeyReader
	partyScope        vendorrole.ScopeReader
}

// NewAuthHandler creates a new AuthHandler with the given dependencies.
func NewAuthHandler(
	authService *auth.Service,
	tokenService *pkgauth.TokenService,
	userRepo pkgauth.UserRepository,
	rateLimiter *middleware.RateLimiter,
	changePasswordSvc ChangePasswordService,
) *AuthHandler {
	return &AuthHandler{
		authService:       authService,
		tokenService:      tokenService,
		userRepo:          userRepo,
		rateLimiter:       rateLimiter,
		changePasswordSvc: changePasswordSvc,
	}
}

// Routes returns a chi.Router with all auth endpoints mounted.
func (h *AuthHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/login", h.Login)
	r.Post("/refresh", h.Refresh)
	r.Post("/logout", h.Logout)
	r.With(middleware.RequireAuth(h.tokenService)).Get("/me", h.Me)
	r.With(middleware.RequireAuth(h.tokenService)).Post("/change-password", h.ChangePassword)
	if h.permKeys != nil {
		r.With(middleware.RequireAuth(h.tokenService)).Get("/me/permissions", h.MyPermissions)
	}
	return r
}

// PermissionKeyReader lists the catalog keys granted to a role (primary read).
type PermissionKeyReader interface {
	ListRoleFeatureKeys(ctx context.Context, role string) ([]string, error)
}

// WithPermissions enables GET /me/permissions. Kept off the constructor so
// existing NewAuthHandler callers are unchanged.
func (h *AuthHandler) WithPermissions(p PermissionKeyReader) *AuthHandler {
	h.permKeys = p
	return h
}

// WithPartyScope enables party_scope in GET /me (role-management-revamp 5.3).
// Unset → party_scope stays null.
func (h *AuthHandler) WithPartyScope(p vendorrole.ScopeReader) *AuthHandler {
	h.partyScope = p
	return h
}

// vendorKeyPrefix marks Portal_Cabang catalog keys.
const vendorKeyPrefix = "vendor-"

// vendorPermissionKeys = the role's vendor-* grants ∪ VendorAlwaysOnKeys
// (non-vendor-* grants on a vendor role are stale and never leak).
func vendorPermissionKeys(granted []string) []string {
	keys := []string{}
	seen := map[string]bool{}
	for _, k := range append(append([]string{}, granted...), rolemgmt.VendorAlwaysOnKeys...) {
		if !seen[k] && (strings.HasPrefix(k, vendorKeyPrefix)) {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	return keys
}

// MyPermissions handles GET /me/permissions (spec permission-catalog-coverage
// Req 4): grants ∪ Always_On ∪ (Locked for APPACCESS/ADMIN); non-internal
// roles get an empty list. ponytail: Always_On/Locked keys are appended after
// the grants (not merged by sort_order); the FE treats the list as a set.
func (h *AuthHandler) MyPermissions(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	keys := []string{}
	if vendorrole.IsVendor(authCtx.Role) {
		granted, err := h.permKeys.ListRoleFeatureKeys(r.Context(), authCtx.Role)
		if err != nil {
			slog.Error("my permissions: list keys", "role", authCtx.Role, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
			return
		}
		keys = vendorPermissionKeys(granted)
	} else if rolemgmt.IsInternalRole(authCtx.Role) {
		granted, err := h.permKeys.ListRoleFeatureKeys(r.Context(), authCtx.Role)
		if err != nil {
			slog.Error("my permissions: list keys", "role", authCtx.Role, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
			return
		}
		seen := make(map[string]bool, len(granted))
		keys = append(keys, granted...)
		for _, k := range granted {
			seen[k] = true
		}
		extra := append([]string{}, rolemgmt.AlwaysOnKeys...)
		if authCtx.Role == "APPACCESS" || authCtx.Role == "ADMIN" {
			extra = append(extra, rolemgmt.LockedKeys...)
		}
		for _, k := range extra {
			if !seen[k] {
				keys = append(keys, k)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string][]string{"permissions": keys})
}

// loginRequest is the expected JSON body for POST /login.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// loginResponse is the JSON response for a successful login.
type loginResponse struct {
	AccessToken string           `json:"access_token"`
	User        auth.UserProfile `json:"user"`
	// PasswordDaysLeft/MustChangePassword mirror auth.LoginResponse — additive
	// fields, existing FE clients that don't read them are unaffected.
	PasswordDaysLeft   *int `json:"password_days_left,omitempty"`
	MustChangePassword bool `json:"must_change_password"`
}

// Login handles POST /login — authenticates user and returns tokens.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "Request body tidak valid")
		return
	}

	portalType := r.Header.Get("X-Portal-Type")
	clientIP := extractClientIP(r)

	loginReq := auth.LoginRequest{
		Email:      req.Email,
		Password:   req.Password,
		PortalType: portalType,
		IP:         clientIP,
	}

	resp, refreshToken, err := h.authService.Login(r.Context(), loginReq)
	if err != nil {
		h.handleAuthError(w, err)
		return
	}

	// Set refresh token as httpOnly cookie
	setRefreshCookie(w, refreshToken, h.tokenService.RefreshDeadline(refreshToken))

	writeJSON(w, http.StatusOK, loginResponse{
		AccessToken:        resp.AccessToken,
		User:               resp.User,
		PasswordDaysLeft:   resp.PasswordDaysLeft,
		MustChangePassword: resp.MustChangePassword,
	})
}

// Refresh handles POST /refresh — validates refresh token and issues new token pair.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil || cookie.Value == "" {
		writeUnauthorized(w, "Sesi telah berakhir")
		return
	}

	claims, err := h.tokenService.ValidateRefreshToken(r.Context(), cookie.Value)
	if err != nil {
		if !errors.Is(err, pkgauth.ErrServiceUnavailable) {
			// Session over (or token dead): stop the browser resending it.
			clearRefreshCookie(w)
		}
		writeUnauthorized(w, "Sesi berakhir, silakan login kembali")
		return
	}

	// Blacklist the old refresh token (rotation)
	_ = h.tokenService.BlacklistRefreshToken(r.Context(), cookie.Value)

	// Look up user profile
	user, err := h.userRepo.GetUserProfile(r.Context(), claims.UserID)
	if err != nil || user == nil {
		writeUnauthorized(w, "Sesi telah berakhir")
		return
	}

	// Generate new token pair
	identity := &pkgauth.AuthIdentity{
		UserID:     user.ID,
		Username:   user.Username,
		Role:       user.Role,
		IsKaryawan: user.IsKaryawan,
		VendorID:   user.VendorID,
	}

	accessToken, refreshTokenNew, err := h.tokenService.RotateTokenPair(identity, claims.ExpiresAt.Time)
	if err != nil {
		writeServiceUnavailable(w, "Gagal memperbarui sesi")
		return
	}

	// Set new refresh cookie
	setRefreshCookie(w, refreshTokenNew, h.tokenService.RefreshDeadline(refreshTokenNew))

	writeJSON(w, http.StatusOK, loginResponse{
		AccessToken: accessToken,
		User: auth.UserProfile{
			ID:         user.ID,
			Username:   user.Username,
			FullName:   user.FullName,
			Email:      user.Email,
			Role:       user.Role,
			IsKaryawan: user.IsKaryawan,
			VendorID:   user.VendorID,
		},
	})
}

// Logout handles POST /logout — blacklists refresh token and clears cookie.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err == nil && cookie.Value != "" {
		// Blacklist the refresh token, ignore errors (idempotent)
		_ = h.tokenService.BlacklistRefreshToken(r.Context(), cookie.Value)
	}

	// Always clear the cookie
	clearRefreshCookie(w)

	writeJSON(w, http.StatusOK, map[string]string{"message": "Berhasil logout"})
}

// Me handles GET /me — returns current user profile.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}

	user, err := h.userRepo.GetUserProfile(r.Context(), authCtx.UserID)
	if err != nil {
		writeServiceUnavailable(w, "Gagal mengambil profil")
		return
	}
	if user == nil {
		writeUnauthorized(w, "User tidak ditemukan")
		return
	}

	var scope []string // null for non-vendor roles
	if h.partyScope != nil && vendorrole.IsVendor(user.Role) {
		scope, err = h.partyScope.PartyScope(r.Context(), user.Role)
		if err != nil {
			slog.Error("me: party scope", "role", user.Role, "error", err)
			writeServiceUnavailable(w, "Gagal mengambil profil")
			return
		}
		if scope == nil {
			scope = []string{} // vendor role without scope = no access, not null
		}
	}

	writeJSON(w, http.StatusOK, meResponse{
		UserProfile: auth.UserProfile{
			ID:         user.ID,
			Username:   user.Username,
			FullName:   user.FullName,
			Email:      user.Email,
			Role:       user.Role,
			IsKaryawan: user.IsKaryawan,
			VendorID:   user.VendorID,
		},
		VendorName:       user.VendorName,
		VendorBranchID:   user.VendorBranchID,
		VendorBranchName: user.VendorBranchName,
		PartyScope:       scope,
	})
}

// meResponse is auth.UserProfile plus the vendor context for the VendorPortal
// scope line (vendor-party-scope Req 8.3). New fields are explicit null when
// empty (no omitempty); existing fields are unchanged.
type meResponse struct {
	auth.UserProfile
	VendorName       *string  `json:"vendor_name"`
	VendorBranchID   *int64   `json:"vendor_branch_id"`
	VendorBranchName *string  `json:"vendor_branch_name"`
	PartyScope       []string `json:"party_scope"` // vendor roles only; null otherwise
}

// changePasswordRequest is the expected JSON body for POST /change-password.
type changePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// ChangePassword handles POST /change-password — self-service password
// change for the caller's own account (RequireAuth). Requires the correct
// old password; not an admin reset. LDAP/Entra accounts are rejected.
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}

	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "Request body tidak valid")
		return
	}

	err := h.changePasswordSvc.ChangePassword(r.Context(), authCtx.UserID, auth.ChangePasswordRequest{
		OldPassword: req.OldPassword,
		NewPassword: req.NewPassword,
	}, extractClientIP(r))
	if err != nil {
		h.handleChangePasswordError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Password berhasil diubah"})
}

// handleChangePasswordError maps ChangePassword errors to HTTP responses.
func (h *AuthHandler) handleChangePasswordError(w http.ResponseWriter, err error) {
	var validationErr *pkgauth.ValidationError
	if errors.As(err, &validationErr) {
		writeValidationError(w, validationErr.Field, validationErr.Message)
		return
	}

	switch {
	case errors.Is(err, pkgauth.ErrInvalidCredentials):
		writeUnauthorized(w, "Password lama salah")
	case errors.Is(err, pkgauth.ErrChangeNotAllowed):
		writeError(w, http.StatusForbidden, "change_not_allowed", pkgauth.ErrChangeNotAllowed.Error())
	case errors.Is(err, pkgauth.ErrPasswordUnchanged):
		writeError(w, http.StatusBadRequest, "password_unchanged", pkgauth.ErrPasswordUnchanged.Error())
	case errors.Is(err, pkgauth.ErrServiceUnavailable):
		writeServiceUnavailable(w, "Layanan sedang tidak tersedia")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
	}
}

// handleAuthError maps auth package errors to appropriate HTTP responses.
func (h *AuthHandler) handleAuthError(w http.ResponseWriter, err error) {
	var validationErr *pkgauth.ValidationError
	if errors.As(err, &validationErr) {
		writeValidationError(w, validationErr.Field, validationErr.Message)
		return
	}

	var rateLimitErr *pkgauth.RateLimitError
	if errors.As(err, &rateLimitErr) {
		writeTooManyRequests(w, "Terlalu banyak percobaan login", rateLimitErr.RetryAfter)
		return
	}

	switch {
	case errors.Is(err, pkgauth.ErrInvalidCredentials):
		writeUnauthorized(w, "Username atau password salah")
	case errors.Is(err, pkgauth.ErrAccountInactive):
		writeError(w, http.StatusForbidden, "account_inactive", "Akun tidak aktif")
	case errors.Is(err, pkgauth.ErrPortalMismatch):
		writeError(w, http.StatusForbidden, "portal_mismatch", "Akun tidak memiliki akses ke portal ini")
	case errors.Is(err, pkgauth.ErrPasswordExpired):
		writeError(w, http.StatusForbidden, "password_expired", pkgauth.ErrPasswordExpired.Error())
	case errors.Is(err, pkgauth.ErrAccountLocked):
		writeError(w, http.StatusForbidden, "account_locked", pkgauth.ErrAccountLocked.Error())
	case errors.Is(err, pkgauth.ErrLDAPNotConfigured):
		writeServiceUnavailable(w, "LDAP authentication tidak tersedia")
	case errors.Is(err, pkgauth.ErrServiceUnavailable):
		writeServiceUnavailable(w, "Layanan sedang tidak tersedia")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
	}
}

// setRefreshCookie sets the refresh_token httpOnly cookie, living until the
// absolute session deadline.
func setRefreshCookie(w http.ResponseWriter, token string, deadline time.Time) {
	maxAge := max(int(time.Until(deadline).Seconds()), 0)
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Path:     "/api/v1/auth",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

// clearRefreshCookie removes the refresh_token cookie.
func clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/api/v1/auth",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

// extractClientIP returns the host part of r.RemoteAddr. Headers are never read
// here: pkg/middleware.TrustedRealIP already resolved X-Forwarded-For from
// trusted proxies into RemoteAddr (bare IP), so a client cannot spoof it.
func extractClientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
