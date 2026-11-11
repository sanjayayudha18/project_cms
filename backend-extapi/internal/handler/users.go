package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cimb-niaga/cms/backend-extapi/internal/extauth"
	"github.com/cimb-niaga/cms/backend-extapi/internal/extuser"
)

const (
	maxUserBody       = 64 << 10
	maxUsernameQuery  = 100
	userLocationPrefx = "/api/ext/v1/users/"
)

// UserService is what UserHandler needs from extuser.Service.
type UserService interface {
	Create(ctx context.Context, a extuser.Actor, req extuser.CreateRequest) (extuser.User, error)
	Update(ctx context.Context, a extuser.Actor, id int64, req extuser.UpdateRequest) (extuser.User, error)
	Disable(ctx context.Context, a extuser.Actor, id int64) error
	Enable(ctx context.Context, a extuser.Actor, id int64) (extuser.User, error)
	Get(ctx context.Context, id int64) (extuser.User, error)
	GetByUsername(ctx context.Context, username string) (extuser.User, error)
}

// UserHandler serves /api/ext/v1/users*. Routes must sit behind RequireClient
// and RequireScope; the id comes from the {id} path value.
type UserHandler struct{ svc UserService }

func NewUserHandler(svc UserService) *UserHandler { return &UserHandler{svc: svc} }

// ExtUser is the wire representation (no auth_source, deleted_at, password data).
type ExtUser struct {
	ID            int64     `json:"id"`
	Username      string    `json:"username"`
	FullName      string    `json:"full_name"`
	Email         string    `json:"email"`
	Role          string    `json:"role"`
	EmployeeID    *string   `json:"employee_id"`
	SupervisorID  *int64    `json:"supervisor_id"`
	ApprovalLevel *int32    `json:"approval_level"`
	IsActive      bool      `json:"is_active"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func toExtUser(u extuser.User) ExtUser {
	return ExtUser{
		ID: u.ID, Username: u.Username, FullName: u.FullName, Email: u.Email, Role: u.Role,
		EmployeeID: u.EmployeeID, SupervisorID: u.SupervisorID, ApprovalLevel: u.ApprovalLevel,
		IsActive: u.IsActive, CreatedAt: u.CreatedAt.UTC(), UpdatedAt: u.UpdatedAt.UTC(),
	}
}

type createBody struct {
	Username      string `json:"username"`
	FullName      string `json:"full_name"`
	Email         string `json:"email"`
	Role          string `json:"role"`
	EmployeeID    string `json:"employee_id"`
	SupervisorID  *int64 `json:"supervisor_id"`
	ApprovalLevel *int32 `json:"approval_level"`
}

type updateBody struct {
	FullName      string `json:"full_name"`
	Email         string `json:"email"`
	Role          string `json:"role"`
	EmployeeID    string `json:"employee_id"`
	SupervisorID  *int64 `json:"supervisor_id"`
	ApprovalLevel *int32 `json:"approval_level"`
}

// Create is POST /users.
func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFrom(w, r)
	if !ok {
		return
	}
	var b createBody
	if !decodeBody(w, r, &b) {
		return
	}
	u, err := h.svc.Create(r.Context(), actor, extuser.CreateRequest{
		Username: b.Username, FullName: b.FullName, Email: b.Email, Role: b.Role,
		EmployeeID: b.EmployeeID, SupervisorID: b.SupervisorID, ApprovalLevel: b.ApprovalLevel,
	})
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Location", userLocationPrefx+strconv.FormatInt(u.ID, 10))
	writeJSON(w, http.StatusCreated, toExtUser(u))
}

// Update is PUT /users/{id}.
func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFrom(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var b updateBody
	if !decodeBody(w, r, &b) {
		return
	}
	u, err := h.svc.Update(r.Context(), actor, id, extuser.UpdateRequest{
		FullName: b.FullName, Email: b.Email, Role: b.Role,
		EmployeeID: b.EmployeeID, SupervisorID: b.SupervisorID, ApprovalLevel: b.ApprovalLevel,
	})
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toExtUser(u))
}

// Disable is DELETE /users/{id}.
func (h *UserHandler) Disable(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFrom(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.svc.Disable(r.Context(), actor, id); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Enable is POST /users/{id}/enable.
func (h *UserHandler) Enable(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFrom(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	u, err := h.svc.Enable(r.Context(), actor, id)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toExtUser(u))
}

// Get is GET /users/{id}.
func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	u, err := h.svc.Get(r.Context(), id)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toExtUser(u))
}

// FindByUsername is GET /users?username=.
func (h *UserHandler) FindByUsername(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("username")
	if n := utf8.RuneCountInString(name); strings.TrimSpace(name) == "" || n > maxUsernameQuery {
		badRequest(w, r, "username query parameter is required (1-100 characters)")
		return
	}
	u, err := h.svc.GetByUsername(r.Context(), name)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toExtUser(u))
}

// actorFrom builds the audit actor from the verified ClientContext.
func actorFrom(w http.ResponseWriter, r *http.Request) (extuser.Actor, bool) {
	cc, ok := extauth.GetClientContext(r.Context())
	if !ok {
		// Router misconfiguration: the handler ran without RequireClient.
		writeError(w, r, http.StatusInternalServerError, apiError{Error: "internal_error", Message: "internal error"},
			errors.New("client context missing"))
		return extuser.Actor{}, false
	}
	return extuser.Actor{
		ServiceUserID:   cc.Client.ServiceUserID,
		ClientID:        cc.Client.ClientID,
		AssignableRoles: cc.Client.AssignableRoles,
		IP:              cc.IP,
	}, true
}

// pathID parses {id} as a positive integer.
func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		badRequest(w, r, "id must be a positive integer")
		return 0, false
	}
	return id, true
}

// decodeBody enforces JSON content type (415), rejects unknown fields and
// trailing data (400). Decoder errors other than unknown fields are not echoed.
func decodeBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mt != "application/json" {
		writeError(w, r, http.StatusUnsupportedMediaType,
			apiError{Error: "unsupported_media_type", Message: "Content-Type must be application/json"}, nil)
		return false
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxUserBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		badRequest(w, r, decodeMessage(err))
		return false
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		badRequest(w, r, "request body must contain a single JSON object")
		return false
	}
	return true
}

// decodeMessage names the offending field for unknown-field errors only.
func decodeMessage(err error) string {
	if f, ok := strings.CutPrefix(err.Error(), "json: unknown field "); ok {
		return "unknown field " + f
	}
	var te *json.UnmarshalTypeError
	if errors.As(err, &te) && te.Field != "" {
		return "invalid type for field " + strconv.Quote(te.Field)
	}
	return "malformed JSON body"
}
