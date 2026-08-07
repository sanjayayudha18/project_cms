package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/auth"
	"github.com/cimb-niaga/cms/backend/internal/middleware"
)

// AuthHandler handles authentication-related HTTP endpoints.
type AuthHandler struct {
	authService  *auth.Service
	tokenService *auth.TokenService
	userRepo     auth.UserRepository
	rateLimiter  *middleware.RateLimiter
}

// NewAuthHandler creates a new AuthHandler with the given dependencies.
func NewAuthHandler(
	authService *auth.Service,
	tokenService *auth.TokenService,
	userRepo auth.UserRepository,
	rateLimiter *middleware.RateLimiter,
) *AuthHandler {
	return &AuthHandler{
		authService:  authService,
		tokenService: tokenService,
		userRepo:     userRepo,
		rateLimiter:  rateLimiter,
	}
}

// Routes returns a chi.Router with all auth endpoints mounted.
func (h *AuthHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/login", h.Login)
	r.Post("/refresh", h.Refresh)
	r.Post("/logout", h.Logout)
	r.With(middleware.RequireAuth(h.tokenService)).Get("/me", h.Me)
	return r
}

// loginRequest is the expected JSON body for POST /login.
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// loginResponse is the JSON response for a successful login.
type loginResponse struct {
	AccessToken string           `json:"access_token"`
	User        auth.UserProfile `json:"user"`
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
		Username:   req.Username,
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
	setRefreshCookie(w, refreshToken)

	writeJSON(w, http.StatusOK, loginResponse{
		AccessToken: resp.AccessToken,
		User:        resp.User,
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
		writeUnauthorized(w, "Sesi telah berakhir")
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
	identity := &auth.AuthIdentity{
		UserID:     user.ID,
		Username:   user.Username,
		Role:       user.Role,
		IsKaryawan: user.IsKaryawan,
		VendorID:   user.VendorID,
	}

	accessToken, refreshTokenNew, err := h.tokenService.GenerateTokenPair(identity)
	if err != nil {
		writeServiceUnavailable(w, "Gagal memperbarui sesi")
		return
	}

	// Set new refresh cookie
	setRefreshCookie(w, refreshTokenNew)

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

	writeJSON(w, http.StatusOK, auth.UserProfile{
		ID:         user.ID,
		Username:   user.Username,
		FullName:   user.FullName,
		Email:      user.Email,
		Role:       user.Role,
		IsKaryawan: user.IsKaryawan,
		VendorID:   user.VendorID,
	})
}

// handleAuthError maps auth package errors to appropriate HTTP responses.
func (h *AuthHandler) handleAuthError(w http.ResponseWriter, err error) {
	var validationErr *auth.ValidationError
	if errors.As(err, &validationErr) {
		writeValidationError(w, validationErr.Field, validationErr.Message)
		return
	}

	var rateLimitErr *auth.RateLimitError
	if errors.As(err, &rateLimitErr) {
		writeTooManyRequests(w, "Terlalu banyak percobaan login", rateLimitErr.RetryAfter)
		return
	}

	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeUnauthorized(w, "Username atau password salah")
	case errors.Is(err, auth.ErrAccountInactive):
		writeError(w, http.StatusForbidden, "account_inactive", "Akun tidak aktif")
	case errors.Is(err, auth.ErrPortalMismatch):
		writeError(w, http.StatusForbidden, "portal_mismatch", "Akun tidak memiliki akses ke portal ini")
	case errors.Is(err, auth.ErrLDAPNotConfigured):
		writeServiceUnavailable(w, "LDAP authentication tidak tersedia")
	case errors.Is(err, auth.ErrServiceUnavailable):
		writeServiceUnavailable(w, "Layanan sedang tidak tersedia")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
	}
}

// setRefreshCookie sets the refresh_token httpOnly cookie.
func setRefreshCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Path:     "/api/v1/auth",
		MaxAge:   604800, // 7 days in seconds
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

// extractClientIP gets the client IP from X-Forwarded-For or RemoteAddr.
func extractClientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		// Take the first IP in the chain (original client)
		parts := strings.SplitN(forwarded, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	// Strip port from RemoteAddr
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		// Handle IPv6 addresses in brackets like [::1]:8080
		if strings.Contains(addr, "]") {
			if bracketIdx := strings.LastIndex(addr, "]"); bracketIdx < idx {
				return addr[:idx]
			}
		} else {
			return addr[:idx]
		}
	}
	return addr
}
