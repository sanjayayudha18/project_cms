package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/auth"
	"github.com/cimb-niaga/cms/backend/internal/db"
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

// UserAdminServicer is the subset of auth.UserAdminService the handler
// needs: List/Get (read-only, Req 2/9.5) plus Create/Update (Req 3/4).
// Disable/Enable are NOT here -- they go through DeactivateService below.
type UserAdminServicer interface {
	List(ctx context.Context, arg db.ListUsersAdminParams) ([]db.ListUsersAdminRow, error)
	Count(ctx context.Context, arg db.CountUsersAdminParams) (int64, error)
	Get(ctx context.Context, id int64) (*db.GetUserAdminByIDRow, error)
	Create(ctx context.Context, actorID int64, req auth.CreateUserRequest, actorIP string) (db.CreateUserAdminRow, error)
	Update(ctx context.Context, actorID, id int64, req auth.UpdateUserRequest, actorIP string) (db.UpdateUserAdminRow, error)
}

// DeactivateService is the subset of auth.DeactivateUserService the handler
// needs for disable/enable (Req 5) -- the existing, previously-unwired
// soft-delete/reactivate service. Not re-implemented per design.md.
type DeactivateService interface {
	Deactivate(ctx context.Context, actorID, targetUserID int64, actorIP string) error
	Reactivate(ctx context.Context, actorID, targetUserID int64, actorIP string) error
}

// AdminUserHandler handles APPACCESS-only user management endpoints:
// list/get/create/update/disable/enable plus the existing
// set-initial-password account-provisioning action.
// Mount behind RequireAuth + RequireRoles("APPACCESS") — see cmd/api/main.go.
type AdminUserHandler struct {
	setInitialPasswordSvc SetInitialPasswordService
	userAdminSvc          UserAdminServicer
	deactivateSvc         DeactivateService
}

// NewAdminUserHandler creates a new AdminUserHandler with the given dependencies.
func NewAdminUserHandler(setInitialPasswordSvc SetInitialPasswordService, userAdminSvc UserAdminServicer, deactivateSvc DeactivateService) *AdminUserHandler {
	return &AdminUserHandler{
		setInitialPasswordSvc: setInitialPasswordSvc,
		userAdminSvc:          userAdminSvc,
		deactivateSvc:         deactivateSvc,
	}
}

// Routes returns a chi.Router with all admin user endpoints mounted.
func (h *AdminUserHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Get("/{id}", h.Get)
	r.Put("/{id}", h.Update)
	r.Post("/{id}/disable", h.Disable)
	r.Post("/{id}/enable", h.Enable)
	r.Post("/{id}/set-initial-password", h.SetInitialPassword)
	return r
}

// List handles GET / — filterable, paginated user list (Req 2).
func (h *AdminUserHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	page, pageSize, err := parsePageParams(q)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	status, err := parseStatusParam(q)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	var vendorID *int64
	if v := q.Get("vendor_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "vendor_id tidak valid")
			return
		}
		vendorID = &id
	}
	var qParam, roleParam *string
	if v := q.Get("q"); v != "" {
		qParam = &v
	}
	if v := q.Get("role"); v != "" {
		roleParam = &v
	}

	listArg := db.ListUsersAdminParams{
		Q: qParam, Role: roleParam, VendorID: vendorID, Status: status,
		PageLimit: int64(pageSize), PageOffset: int64((page - 1) * pageSize),
	}
	rows, err := h.userAdminSvc.List(r.Context(), listArg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	total, err := h.userAdminSvc.Count(r.Context(), db.CountUsersAdminParams{
		Q: qParam, Role: roleParam, VendorID: vendorID, Status: status,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}

	items := make([]map[string]any, len(rows))
	for i, row := range rows {
		items[i] = listUserRowToResponse(row)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"users": items, "page": page, "page_size": pageSize, "total": total,
	})
}

// Get handles GET /{id}.
func (h *AdminUserHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	row, err := h.userAdminSvc.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	if row == nil {
		writeError(w, http.StatusNotFound, "not_found", "Pengguna tidak ditemukan")
		return
	}
	writeJSON(w, http.StatusOK, getUserRowToResponse(*row))
}

type createUserRequestBody struct {
	Username          string `json:"username"`
	FullName          string `json:"full_name"`
	Email             string `json:"email"`
	Role              string `json:"role"`
	IsKaryawan        bool   `json:"is_karyawan"`
	AuthSource        string `json:"auth_source"`
	TemporaryPassword string `json:"temporary_password"`
	EmployeeID        string `json:"employee_id"`
	VendorID          *int64 `json:"vendor_id"`
	SupervisorID      *int64 `json:"supervisor_id"`
	ApprovalLevel     *int32 `json:"approval_level"`
}

// Create handles POST / (Req 3).
func (h *AdminUserHandler) Create(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}

	var body createUserRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}

	created, err := h.userAdminSvc.Create(r.Context(), authCtx.UserID, auth.CreateUserRequest{
		Username: body.Username, FullName: body.FullName, Email: body.Email, Role: body.Role,
		IsKaryawan: body.IsKaryawan, AuthSource: body.AuthSource, TemporaryPassword: body.TemporaryPassword,
		EmployeeID: body.EmployeeID, VendorID: body.VendorID, SupervisorID: body.SupervisorID,
		ApprovalLevel: body.ApprovalLevel,
	}, extractClientIP(r))
	if err != nil {
		h.handleUserAdminError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, createUserRowToResponse(created))
}

type updateUserRequestBody struct {
	Username      *string `json:"username"`
	AuthSource    *string `json:"auth_source"`
	FullName      string  `json:"full_name"`
	Email         string  `json:"email"`
	Role          string  `json:"role"`
	IsKaryawan    bool    `json:"is_karyawan"`
	EmployeeID    string  `json:"employee_id"`
	VendorID      *int64  `json:"vendor_id"`
	SupervisorID  *int64  `json:"supervisor_id"`
	ApprovalLevel *int32  `json:"approval_level"`
}

// Update handles PUT /{id} (Req 4).
func (h *AdminUserHandler) Update(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}

	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	var body updateUserRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}

	updated, err := h.userAdminSvc.Update(r.Context(), authCtx.UserID, id, auth.UpdateUserRequest{
		Username: body.Username, AuthSource: body.AuthSource, FullName: body.FullName, Email: body.Email,
		Role: body.Role, IsKaryawan: body.IsKaryawan, EmployeeID: body.EmployeeID, VendorID: body.VendorID,
		SupervisorID: body.SupervisorID, ApprovalLevel: body.ApprovalLevel,
	}, extractClientIP(r))
	if err != nil {
		h.handleUserAdminError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, updateUserRowToResponse(updated))
}

// Disable handles POST /{id}/disable (Req 5.1, 5.4, 5.5, 5.7). Existence is
// pre-checked via userAdminSvc.Get (incl. soft-deleted) so an unknown id is
// a clean 404 with no audit entry -- DeactivateUserService.Deactivate itself
// has no such guard.
func (h *AdminUserHandler) Disable(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}

	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	if id == authCtx.UserID {
		writeError(w, http.StatusBadRequest, "self_disable_forbidden", "Tidak dapat menonaktifkan akun sendiri")
		return
	}

	existing, err := h.userAdminSvc.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "not_found", "Pengguna tidak ditemukan")
		return
	}

	if err := h.deactivateSvc.Deactivate(r.Context(), authCtx.UserID, id, extractClientIP(r)); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Pengguna berhasil dinonaktifkan"})
}

// Enable handles POST /{id}/enable (Req 5.2, 5.6). Same existence
// pre-check as Disable: DeactivateUserService.Reactivate silently no-ops on
// an unknown id but would otherwise still write an audit entry.
func (h *AdminUserHandler) Enable(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}

	id, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	existing, err := h.userAdminSvc.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "not_found", "Pengguna tidak ditemukan")
		return
	}

	if err := h.deactivateSvc.Reactivate(r.Context(), authCtx.UserID, id, extractClientIP(r)); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Pengguna berhasil diaktifkan kembali"})
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

// handleUserAdminError maps UserAdminService.Create/Update errors to HTTP
// responses per design.md's "Error -> HTTP mapping" table.
func (h *AdminUserHandler) handleUserAdminError(w http.ResponseWriter, err error) {
	var validationErr *pkgauth.ValidationError
	var conflictErr *auth.ConflictError
	var referenceErr *auth.ReferenceError

	switch {
	case errors.As(err, &validationErr):
		writeValidationError(w, validationErr.Field, validationErr.Message)
	case errors.As(err, &conflictErr):
		writeError(w, http.StatusConflict, "conflict", conflictErr.Error())
	case errors.As(err, &referenceErr):
		writeError(w, http.StatusBadRequest, "invalid_reference", referenceErr.Error())
	case errors.Is(err, pkgauth.ErrUserNotFound):
		writeError(w, http.StatusNotFound, "not_found", pkgauth.ErrUserNotFound.Error())
	case errors.Is(err, auth.ErrImmutableField):
		writeError(w, http.StatusBadRequest, "bad_request", auth.ErrImmutableField.Error())
	case errors.Is(err, auth.ErrSelfSupervision):
		writeError(w, http.StatusBadRequest, "bad_request", auth.ErrSelfSupervision.Error())
	case errors.Is(err, auth.ErrLdapFieldsNotAllowed):
		writeError(w, http.StatusBadRequest, "bad_request", auth.ErrLdapFieldsNotAllowed.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
	}
}

// --- query param helpers (shared with AdminVendorHandler) -----------------

// parsePageParams parses page/page_size, defaulting to 1/25 and capping
// page_size at 100 (Req 2.6, 6.4).
func parsePageParams(q url.Values) (page, pageSize int, err error) {
	page, pageSize = 1, 25
	if v := q.Get("page"); v != "" {
		page, err = strconv.Atoi(v)
		if err != nil || page < 1 {
			return 0, 0, errors.New("page tidak valid")
		}
	}
	if v := q.Get("page_size"); v != "" {
		pageSize, err = strconv.Atoi(v)
		if err != nil || pageSize < 1 {
			return 0, 0, errors.New("page_size tidak valid")
		}
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize, nil
}

// parseStatusParam parses status, defaulting to "active" when absent
// (Req 2.5, 6.3 / Resolved Decision 5).
func parseStatusParam(q url.Values) (string, error) {
	status := q.Get("status")
	if status == "" {
		return "active", nil
	}
	if status != "active" && status != "disabled" && status != "all" {
		return "", errors.New("status tidak valid")
	}
	return status, nil
}

// --- response mapping ------------------------------------------------------

// formatTimestamptz formats a nullable pgtype.Timestamptz as RFC3339, or nil
// for SQL NULL.
func formatTimestamptz(t pgtype.Timestamptz) *string {
	if !t.Valid {
		return nil
	}
	s := t.Time.Format(time.RFC3339)
	return &s
}

func listUserRowToResponse(r db.ListUsersAdminRow) map[string]any {
	return map[string]any{
		"id": r.ID, "username": r.Username, "full_name": r.FullName, "email": r.Email,
		"role": r.Role, "is_karyawan": r.IsKaryawan, "auth_source": r.AuthSource,
		"vendor_id": r.VendorID, "is_active": r.IsActive, "deleted_at": formatTimestamptz(r.DeletedAt),
		"supervisor_id": r.SupervisorID, "approval_level": r.ApprovalLevel,
		"last_login_at": formatTimestamptz(r.LastLoginAt),
	}
}

func getUserRowToResponse(r db.GetUserAdminByIDRow) map[string]any {
	return map[string]any{
		"id": r.ID, "username": r.Username, "full_name": r.FullName, "email": r.Email,
		"role": r.Role, "is_karyawan": r.IsKaryawan, "auth_source": r.AuthSource,
		"employee_id": r.EmployeeID, "vendor_id": r.VendorID, "is_active": r.IsActive,
		"deleted_at": formatTimestamptz(r.DeletedAt), "supervisor_id": r.SupervisorID,
		"approval_level": r.ApprovalLevel, "last_login_at": formatTimestamptz(r.LastLoginAt),
	}
}

func createUserRowToResponse(r db.CreateUserAdminRow) map[string]any {
	return map[string]any{
		"id": r.ID, "username": r.Username, "full_name": r.FullName, "email": r.Email,
		"role_id": r.RoleID, "is_karyawan": r.IsKaryawan, "auth_source": r.AuthSource,
		"employee_id": r.EmployeeID, "vendor_id": r.VendorID, "is_active": r.IsActive,
		"deleted_at": formatTimestamptz(r.DeletedAt), "supervisor_id": r.SupervisorID,
		"approval_level": r.ApprovalLevel, "last_login_at": formatTimestamptz(r.LastLoginAt),
	}
}

func updateUserRowToResponse(r db.UpdateUserAdminRow) map[string]any {
	return map[string]any{
		"id": r.ID, "username": r.Username, "full_name": r.FullName, "email": r.Email,
		"role_id": r.RoleID, "is_karyawan": r.IsKaryawan, "auth_source": r.AuthSource,
		"employee_id": r.EmployeeID, "vendor_id": r.VendorID, "is_active": r.IsActive,
		"deleted_at": formatTimestamptz(r.DeletedAt), "supervisor_id": r.SupervisorID,
		"approval_level": r.ApprovalLevel, "last_login_at": formatTimestamptz(r.LastLoginAt),
	}
}
