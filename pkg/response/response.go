package response

import (
	"encoding/json"
	"net/http"
)

// Envelope is the standard API response wrapper used by services that
// opt into the shared response format (currently backend-cit; the ATM
// backend keeps its existing flat JSON shape for frontend compatibility).
type Envelope struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *ErrorBody  `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// ErrorBody carries the error code, message, and optional field details.
type ErrorBody struct {
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Details []FieldError `json:"details,omitempty"`
}

// FieldError represents a single field-level validation error.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Meta carries pagination metadata for list responses.
type Meta struct {
	Page       int `json:"page,omitempty"`
	PerPage    int `json:"per_page,omitempty"`
	TotalRows  int `json:"total_rows,omitempty"`
	TotalPages int `json:"total_pages,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// WriteSuccess writes a 200 success response: {"success":true,"data":<data>}.
func WriteSuccess(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, Envelope{Success: true, Data: data})
}

// WriteCreated writes a 201 success response: {"success":true,"data":<data>}.
func WriteCreated(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusCreated, Envelope{Success: true, Data: data})
}

// WriteError writes an error response with the given HTTP status:
// {"success":false,"error":{"code":<code>,"message":<message>}}.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, Envelope{Success: false, Error: &ErrorBody{Code: code, Message: message}})
}

// WriteValidationError writes a 422 response with field-level validation details.
func WriteValidationError(w http.ResponseWriter, details []FieldError) {
	writeJSON(w, http.StatusUnprocessableEntity, Envelope{
		Success: false,
		Error: &ErrorBody{
			Code:    "validation_error",
			Message: "Validasi gagal",
			Details: details,
		},
	})
}
