package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/rolemgmt"
	"github.com/cimb-niaga/cms/pkg/middleware"
)

// RoleMgmtServicer is the subset of rolemgmt.PermissionService the handler
// needs: ListRoles/ListCatalog (read-only, Req 1.1/3.5) plus
// CreateRole/UpdateRolePermissions (Req 2-4, 6).
type RoleMgmtServicer interface {
	ListRoles(ctx context.Context) ([]db.ListRolesWithPermissionsRow, error)
	ListCatalog(ctx context.Context) ([]db.MenuFeature, error)
	CreateRole(ctx context.Context, actorID int64, actorRole string, req rolemgmt.CreateRoleRequest, actorIP string) (db.Role, error)
	UpdateRolePermissions(ctx context.Context, actorID int64, actorRole string, roleID int64, featureIDs []int64, actorIP string) ([]int64, error)
}

// RoleMgmtHandler handles APPACCESS/ADMIN-only Role Management endpoints:
// list roles+permissions, list catalog, create role, update role permissions.
// Mount behind RequireAuth + RequireRoles("APPACCESS", "ADMIN") — see
// cmd/api/main.go. This static guard protects the endpoints that administer
// permissions themselves; it is deliberately separate from the dynamic
// rolemgmt.RequirePermission middleware, which is for other, new business
// routes that opt into the data-driven model (design.md "Middleware").
type RoleMgmtHandler struct {
	svc RoleMgmtServicer
}

// NewRoleMgmtHandler creates a new RoleMgmtHandler with the given dependency.
func NewRoleMgmtHandler(svc RoleMgmtServicer) *RoleMgmtHandler {
	return &RoleMgmtHandler{svc: svc}
}

// Routes returns a chi.Router with all Role Management endpoints mounted.
func (h *RoleMgmtHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.ListRoles)
	r.Get("/catalog", h.ListCatalog)
	r.Post("/", h.CreateRole)
	r.Put("/{id}/permissions", h.UpdateRolePermissions)
	return r
}

// ListRoles handles GET / (Req 3.5): every role with its mapped catalog entries.
func (h *RoleMgmtHandler) ListRoles(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.ListRoles(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"roles": rolesWithPermissionsToResponse(rows)})
}

// ListCatalog handles GET /catalog (Req 1.1): the full active menu/feature catalog.
func (h *RoleMgmtHandler) ListCatalog(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.ListCatalog(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
		return
	}
	items := make([]map[string]any, len(rows))
	for i, row := range rows {
		items[i] = catalogEntryToResponse(row)
	}
	writeJSON(w, http.StatusOK, map[string]any{"catalog": items})
}

type createRoleRequestBody struct {
	Role        string `json:"role"`
	Description string `json:"description"`
}

// CreateRole handles POST / (Req 2.1-2.5).
func (h *RoleMgmtHandler) CreateRole(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := middleware.GetAuthContext(r.Context())
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}

	var body createRoleRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}

	created, err := h.svc.CreateRole(r.Context(), authCtx.UserID, authCtx.Role, rolemgmt.CreateRoleRequest{
		Role: body.Role, Description: body.Description,
	}, extractClientIP(r))
	if err != nil {
		h.handleRoleMgmtError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, roleToResponse(created))
}

type updateRolePermissionsRequestBody struct {
	MenuFeatureIDs []int64 `json:"menu_feature_ids"`
}

// UpdateRolePermissions handles PUT /{id}/permissions (Req 3.1-3.4): full-set
// replace of the catalog entries granted to a role.
func (h *RoleMgmtHandler) UpdateRolePermissions(w http.ResponseWriter, r *http.Request) {
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

	var body updateRolePermissionsRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Body tidak valid")
		return
	}

	updated, err := h.svc.UpdateRolePermissions(r.Context(), authCtx.UserID, authCtx.Role, id, body.MenuFeatureIDs, extractClientIP(r))
	if err != nil {
		h.handleRoleMgmtError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"role_id": id, "permissions": updated})
}

// handleRoleMgmtError maps rolemgmt.PermissionService errors to HTTP
// responses per design.md's "Error -> HTTP mapping" table.
func (h *RoleMgmtHandler) handleRoleMgmtError(w http.ResponseWriter, err error) {
	var validationErr *rolemgmt.ValidationError

	switch {
	case errors.As(err, &validationErr):
		writeValidationError(w, validationErr.Field, validationErr.Message)
	case errors.Is(err, rolemgmt.ErrRoleNameConflict):
		writeError(w, http.StatusConflict, "conflict", rolemgmt.ErrRoleNameConflict.Error())
	case errors.Is(err, rolemgmt.ErrCatalogEntryNotFound):
		writeError(w, http.StatusBadRequest, "invalid_reference", rolemgmt.ErrCatalogEntryNotFound.Error())
	case errors.Is(err, rolemgmt.ErrRoleNotFound):
		writeError(w, http.StatusNotFound, "not_found", rolemgmt.ErrRoleNotFound.Error())
	case errors.Is(err, rolemgmt.ErrNotAuthorized):
		writeForbidden(w, rolemgmt.ErrNotAuthorized.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
	}
}

// --- response mapping ------------------------------------------------------

func roleToResponse(r db.Role) map[string]any {
	return map[string]any{"id": r.ID, "role": r.Role, "description": r.Description}
}

func catalogEntryToResponse(r db.MenuFeature) map[string]any {
	return map[string]any{
		"id": r.ID, "parent_id": r.ParentID, "key": r.Key, "label": r.Label,
		"kind": r.Kind, "sort_order": r.SortOrder,
	}
}

// rolesWithPermissionsToResponse groups the flat (role, granted entry) rows
// from ListRolesWithPermissions into one object per role, matching design.md's
// GET / response shape. A role with no grants (all MenuFeature* fields nil,
// from the repository's LEFT JOIN) yields an empty permissions array, not a
// one-element array of nulls.
func rolesWithPermissionsToResponse(rows []db.ListRolesWithPermissionsRow) []map[string]any {
	order := make([]int64, 0)
	byRole := make(map[int64]map[string]any)

	for _, row := range rows {
		entry, ok := byRole[row.RoleID]
		if !ok {
			entry = map[string]any{
				"id": row.RoleID, "role": row.Role, "description": row.Description,
				"permissions": []map[string]any{},
			}
			byRole[row.RoleID] = entry
			order = append(order, row.RoleID)
		}
		if row.MenuFeatureID == nil {
			continue
		}
		perms := entry["permissions"].([]map[string]any)
		entry["permissions"] = append(perms, map[string]any{
			"menu_feature_id": *row.MenuFeatureID,
			"parent_id":       row.MenuFeatureParentID,
			"key":             *row.MenuFeatureKey,
			"label":           *row.MenuFeatureLabel,
			"kind":            *row.MenuFeatureKind,
		})
	}

	items := make([]map[string]any, len(order))
	for i, roleID := range order {
		items[i] = byRole[roleID]
	}
	return items
}
