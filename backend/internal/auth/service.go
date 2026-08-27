package auth

import (
	"context"
	"strings"

	"github.com/cimb-niaga/cms/pkg/auth"
)

// Service orchestrates the login flow.
type Service struct {
	providers    []auth.Provider
	tokenService *auth.TokenService
	userRepo     auth.UserRepository
	rateLimiter  auth.RateLimiter
}

// LoginRequest holds the data required for a login attempt.
type LoginRequest struct {
	Username   string `json:"username" validate:"required,min=1,max=255"`
	Password   string `json:"password" validate:"required,min=1,max=255"`
	PortalType string // from X-Portal-Type header: "company" | "vendor"
	IP         string // from request context
}

// LoginResponse is returned on successful authentication.
type LoginResponse struct {
	AccessToken string      `json:"access_token"`
	User        UserProfile `json:"user"`
}

// UserProfile represents the user data returned in login and /me responses.
type UserProfile struct {
	ID         int64  `json:"id"`
	Username   string `json:"username"`
	FullName   string `json:"full_name"`
	Email      string `json:"email"`
	Role       string `json:"role"`
	IsKaryawan bool   `json:"is_karyawan"`
	VendorID   *int64 `json:"vendor_id,omitempty"`
}

// NewService creates a new auth Service with the given dependencies.
func NewService(providers []auth.Provider, tokenService *auth.TokenService, userRepo auth.UserRepository, rateLimiter auth.RateLimiter) *Service {
	return &Service{
		providers:    providers,
		tokenService: tokenService,
		userRepo:     userRepo,
		rateLimiter:  rateLimiter,
	}
}

// Login executes the authentication flow in strict order:
// 1. Input validation
// 2. Rate limit check
// 3. User lookup (not found / deleted_at → generic error)
// 4. is_active check
// 5. Credential verification via Provider (based on auth_source)
// 6. Portal type restriction
// 7. Generate tokens
// 8. Update last_login_at + reset rate limit
//
// Returns: LoginResponse, refreshToken (for httpOnly cookie), or error.
func (s *Service) Login(ctx context.Context, req LoginRequest) (*LoginResponse, string, error) {
	// Step 1: Input validation
	if err := s.validateInput(req); err != nil {
		return nil, "", err
	}

	// Step 2: Rate limit check
	if err := s.rateLimiter.Check(ctx, req.Username, req.IP); err != nil {
		return nil, "", err
	}

	// Step 3: User lookup
	user, err := s.userRepo.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, "", auth.ErrServiceUnavailable
	}
	if user == nil {
		return nil, "", auth.ErrInvalidCredentials
	}
	// deleted_at is already filtered by FindByUsername (WHERE deleted_at IS NULL),
	// but if returned user has deleted_at set, treat as not found
	if user.DeletedAt != nil {
		return nil, "", auth.ErrInvalidCredentials
	}

	// Step 4: is_active check
	if !user.IsActive {
		return nil, "", auth.ErrAccountInactive
	}

	// Step 5: Credential verification via selected Provider
	provider, err := s.selectProvider(user.AuthSource)
	if err != nil {
		return nil, "", err
	}

	_, authErr := provider.Authenticate(ctx, req.Username, req.Password)
	if authErr != nil {
		// Increment rate limit on credential failure
		_ = s.rateLimiter.IncrementFailed(ctx, req.Username, req.IP)
		return nil, "", auth.ErrInvalidCredentials
	}

	// Step 6: Portal type restriction
	if err := s.validatePortalAccess(user, req.PortalType); err != nil {
		return nil, "", err
	}

	// Step 7: Generate token pair
	identity := &auth.AuthIdentity{
		UserID:     user.ID,
		Username:   user.Username,
		Role:       user.Role,
		IsKaryawan: user.IsKaryawan,
		VendorID:   user.VendorID,
	}

	accessToken, refreshToken, err := s.tokenService.GenerateTokenPair(identity)
	if err != nil {
		return nil, "", auth.ErrServiceUnavailable
	}

	// Step 8: Update last_login_at + reset rate limit
	_ = s.userRepo.UpdateLastLogin(ctx, user.ID)
	_ = s.rateLimiter.ResetUsername(ctx, req.Username)

	response := &LoginResponse{
		AccessToken: accessToken,
		User: UserProfile{
			ID:         user.ID,
			Username:   user.Username,
			FullName:   user.FullName,
			Email:      user.Email,
			Role:       user.Role,
			IsKaryawan: user.IsKaryawan,
			VendorID:   user.VendorID,
		},
	}

	return response, refreshToken, nil
}

// validateInput checks that username, password, and portal type are valid.
// Returns a ValidationError for the first failing field.
func (s *Service) validateInput(req LoginRequest) error {
	// Username validation
	if strings.TrimSpace(req.Username) == "" {
		return &auth.ValidationError{Field: "username", Message: "wajib diisi"}
	}
	if len(req.Username) > 255 {
		return &auth.ValidationError{Field: "username", Message: "maksimal 255 karakter"}
	}

	// Password validation
	if strings.TrimSpace(req.Password) == "" {
		return &auth.ValidationError{Field: "password", Message: "wajib diisi"}
	}
	if len(req.Password) > 255 {
		return &auth.ValidationError{Field: "password", Message: "maksimal 255 karakter"}
	}

	// Portal type validation
	if req.PortalType != "company" && req.PortalType != "vendor" {
		return &auth.ValidationError{Field: "X-Portal-Type", Message: "harus bernilai 'company' atau 'vendor'"}
	}

	return nil
}

// selectProvider finds the appropriate auth provider for the given auth_source.
// Returns ErrLDAPNotConfigured if auth_source is 'ldap' and no provider supports it.
// Returns ErrUnsupportedAuthSource for unknown auth_source values.
func (s *Service) selectProvider(authSource string) (auth.Provider, error) {
	for _, p := range s.providers {
		if p.Supports(authSource) {
			return p, nil
		}
	}

	if authSource == "ldap" {
		return nil, auth.ErrLDAPNotConfigured
	}

	return nil, auth.ErrUnsupportedAuthSource
}

// validatePortalAccess checks that the user type matches the requested portal.
// is_karyawan=true + portal='vendor' → ErrPortalMismatch
// is_karyawan=false + portal='company' → ErrPortalMismatch
func (s *Service) validatePortalAccess(user *auth.UserRecord, portalType string) error {
	if user.IsKaryawan && portalType == "vendor" {
		return auth.ErrPortalMismatch
	}
	if !user.IsKaryawan && portalType == "company" {
		return auth.ErrPortalMismatch
	}
	return nil
}
