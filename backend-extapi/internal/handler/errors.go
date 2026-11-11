package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/cimb-niaga/cms/backend-extapi/internal/extauth"
	"github.com/cimb-niaga/cms/backend-extapi/internal/extuser"
	"github.com/cimb-niaga/cms/pkg/auth"
	"github.com/cimb-niaga/cms/pkg/userrules"
)

// apiError is the flat error body of the user API (design: Error Handling).
// Field is set only for validation_error.
type apiError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

// writeError writes a flat {error, message} body. A 4xx is logged at warn with
// client_id, route and the error code — never the request body (Req 10.5); a
// 5xx logs cause at error level and sends no internal detail.
func writeError(w http.ResponseWriter, r *http.Request, status int, e apiError, cause error) {
	attrs := []any{
		slog.String("client_id", clientIDOf(r)),
		slog.String("route", r.Method+" "+r.URL.Path),
		slog.Int("status", status),
		slog.String("error", e.Error),
	}
	if status >= 500 {
		if cause != nil {
			attrs = append(attrs, slog.String("cause", cause.Error()))
		}
		slog.ErrorContext(r.Context(), "ext_api_error", attrs...)
	} else {
		slog.WarnContext(r.Context(), "ext_api_rejected", attrs...)
	}
	writeJSON(w, status, e)
}

// writeServiceError maps an extuser.Service error to HTTP.
func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	var (
		ve *auth.ValidationError
		re *userrules.ReferenceError
		ce *userrules.ConflictError
	)
	switch {
	case errors.As(err, &ve):
		writeError(w, r, http.StatusUnprocessableEntity, apiError{"validation_error", ve.Message, ve.Field}, err)
	case errors.As(err, &re):
		writeError(w, r, http.StatusBadRequest, apiError{Error: "invalid_reference", Message: re.Error()}, err)
	case errors.As(err, &ce):
		writeError(w, r, http.StatusConflict, apiError{Error: "conflict", Message: ce.Error()}, err)
	case errors.Is(err, extuser.ErrSelfSupervision):
		writeError(w, r, http.StatusBadRequest, apiError{Error: "bad_request", Message: err.Error()}, err)
	case errors.Is(err, extuser.ErrRoleNotAllowed):
		writeError(w, r, http.StatusForbidden, apiError{Error: "role_not_allowed", Message: err.Error()}, err)
	case errors.Is(err, extuser.ErrUserNotFound):
		writeError(w, r, http.StatusNotFound, apiError{Error: "not_found", Message: "user not found"}, err)
	default:
		writeError(w, r, http.StatusInternalServerError, apiError{Error: "internal_error", Message: "internal error"}, err)
	}
}

func badRequest(w http.ResponseWriter, r *http.Request, msg string) {
	writeError(w, r, http.StatusBadRequest, apiError{Error: "bad_request", Message: msg}, nil)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v) // headers already sent
}

func clientIDOf(r *http.Request) string {
	if cc, ok := extauth.GetClientContext(r.Context()); ok {
		return cc.Client.ClientID
	}
	return ""
}
