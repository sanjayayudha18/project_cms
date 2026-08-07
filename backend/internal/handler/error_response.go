package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// ErrorResponse is the standard error format returned by all API endpoints.
type ErrorResponse struct {
	Error   string       `json:"error"`
	Message string       `json:"message"`
	Details []FieldError `json:"details,omitempty"`
}

// FieldError represents a single field-level validation error.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError writes an ErrorResponse with the given status, error code, and message.
func writeError(w http.ResponseWriter, status int, errorCode, message string) {
	writeJSON(w, status, ErrorResponse{
		Error:   errorCode,
		Message: message,
	})
}

// writeValidationError writes a 422 response with field-level validation details.
func writeValidationError(w http.ResponseWriter, field, message string) {
	writeJSON(w, http.StatusUnprocessableEntity, ErrorResponse{
		Error:   "validation_error",
		Message: "Validasi gagal",
		Details: []FieldError{
			{Field: field, Message: message},
		},
	})
}

// writeUnauthorized writes a 401 Unauthorized response.
func writeUnauthorized(w http.ResponseWriter, message string) {
	writeError(w, http.StatusUnauthorized, "unauthorized", message)
}

// writeForbidden writes a 403 Forbidden response.
func writeForbidden(w http.ResponseWriter, message string) {
	writeError(w, http.StatusForbidden, "forbidden", message)
}

// writeTooManyRequests writes a 429 Too Many Requests response with Retry-After header.
func writeTooManyRequests(w http.ResponseWriter, message string, retryAfter int) {
	w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	writeError(w, http.StatusTooManyRequests, "rate_limited", message)
}

// writeServiceUnavailable writes a 503 Service Unavailable response.
func writeServiceUnavailable(w http.ResponseWriter, message string) {
	writeError(w, http.StatusServiceUnavailable, "service_unavailable", message)
}
