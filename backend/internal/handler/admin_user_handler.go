package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/auth"
	"github.com/cimb-niaga/cms/pkg/middleware"

	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
)

// SetInitialPasswordService is the subset of auth.SetInitialPasswordService
// the handler needs (mirrors the ChangePasswordService/ApprovalOrchestrator
// narrow-interface pattern used elsewhere in this package, so handler tests
// can fake it without a real DB).
type SetInitialPasswordService interface {
	SetInitialPassword(ctx context.Context, actorID, targetUserID int64, req auth.SetInitialPasswordRequest, actorIP string) error
}

// AdminUserHandler handles APPACCESS-only account-provisioning endpoints.
// Mount behind RequireAuth + RequireRoles("APPACCESS") — see cmd/api/main.go.
type AdminUserHandler struct {
	setInitialPasswordSvc SetInitialPasswordService
}

// NewAdminUserHandler creates a new AdminUserHandler with the given dependency.
func NewAdminUserHandler(svc SetInitialPasswordService) *AdminUserHandler {
	return &AdminUserHandler{setInitialPasswordSvc: svc}
}

// Routes returns a chi.Router with all admin user-provisioning endpoints mounted.
func (h *AdminUserHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/{id}/set-initial-password", h.SetInitialPassword)
	return r
}

type setInitialPasswordRequest struct {
	NewPassword string `json:"new_password"`
}

// SetInitialPassword handles POST /{id}/set-initial-password — APPACCESS
// sets a target user's password with no old password required, forcing a
// change on that user's next login.
func (h *AdminUserHandler) SetInitialPassword(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}

	targetUserID, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	var req setInitialPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "Request body tidak valid")
		return
	}

	err = h.setInitialPasswordSvc.SetInitialPassword(r.Context(), authCtx.UserID, targetUserID, auth.SetInitialPasswordRequest{
		NewPassword: req.NewPassword,
	}, extractClientIP(r))
	if err != nil {
		h.handleSetInitialPasswordError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Password awal berhasil diset"})
}

// handleSetInitialPasswordError maps SetInitialPassword errors to HTTP responses.
func (h *AdminUserHandler) handleSetInitialPasswordError(w http.ResponseWriter, err error) {
	var validationErr *pkgauth.ValidationError
	if errors.As(err, &validationErr) {
		writeValidationError(w, validationErr.Field, validationErr.Message)
		return
	}

	switch {
	case errors.Is(err, pkgauth.ErrUserNotFound):
		writeError(w, http.StatusNotFound, "not_found", pkgauth.ErrUserNotFound.Error())
	case errors.Is(err, pkgauth.ErrChangeNotAllowed):
		writeError(w, http.StatusForbidden, "change_not_allowed", pkgauth.ErrChangeNotAllowed.Error())
	case errors.Is(err, pkgauth.ErrServiceUnavailable):
		writeServiceUnavailable(w, "Layanan sedang tidak tersedia")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
	}
}
