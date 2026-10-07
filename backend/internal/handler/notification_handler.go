package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

const (
	notificationDefaultPageSize = 20
	notificationMaxPageSize     = 100
)

// NotificationStore is what NotificationHandler needs (implemented by
// notification.Repository), defined where it's used.
type NotificationStore interface {
	List(ctx context.Context, userID int64, unreadOnly bool, page, pageSize int32) ([]db.ListNotificationsForUserRow, int64, error)
	UnreadCount(ctx context.Context, userID int64) (int64, error)
	MarkRead(ctx context.Context, id, userID int64) (bool, error)
	MarkAllRead(ctx context.Context, userID int64) (int64, error)
}

// NotificationHandler serves /api/v1/notifications (notification spec FR4)
// for every authenticated role, VENDOR-USER included. The user is always the
// JWT caller; there is no way to read or change someone else's
// notifications (someone else's id answers 404, like a missing one).
type NotificationHandler struct {
	store NotificationStore
}

func NewNotificationHandler(store NotificationStore) *NotificationHandler {
	return &NotificationHandler{store: store}
}

func (h *NotificationHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Get("/unread-count", h.UnreadCount)
	r.Post("/read-all", h.MarkAllRead)
	r.Post("/{id}/read", h.MarkRead)
	return r
}

type notificationResponse struct {
	ID        int64   `json:"id"`
	Type      string  `json:"type"`
	Title     string  `json:"title"`
	Body      string  `json:"body"`
	Link      *string `json:"link"`
	IsRead    bool    `json:"is_read"`
	CreatedAt string  `json:"created_at"`
}

type notificationListResponse struct {
	Items    []notificationResponse `json:"items"`
	Total    int64                  `json:"total"`
	Page     int32                  `json:"page"`
	PageSize int32                  `json:"page_size"`
}

func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	q := r.URL.Query()
	unreadOnly, err := parseOptionalBool(q.Get("unread_only"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "unread_only harus true atau false")
		return
	}
	page, ok := parseBoundedInt(q.Get("page"), 1, 1, 1<<20)
	if !ok {
		writeError(w, http.StatusBadRequest, "bad_request", "page tidak valid")
		return
	}
	pageSize, ok := parseBoundedInt(q.Get("page_size"), notificationDefaultPageSize, 1, notificationMaxPageSize)
	if !ok {
		writeError(w, http.StatusBadRequest, "bad_request", "page_size harus 1-100")
		return
	}

	rows, total, err := h.store.List(r.Context(), actor.UserID, unreadOnly, page, pageSize)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	items := make([]notificationResponse, len(rows))
	for i, n := range rows {
		items[i] = notificationResponse{
			ID: n.ID, Type: n.Type, Title: n.Title, Body: n.Body, Link: n.Link,
			IsRead: n.IsRead, CreatedAt: formatTimestamp(n.CreatedAt.Time),
		}
	}
	writeJSON(w, http.StatusOK, notificationListResponse{Items: items, Total: total, Page: page, PageSize: pageSize})
}

func (h *NotificationHandler) UnreadCount(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	n, err := h.store.UnreadCount(r.Context(), actor.UserID)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"unread_count": n})
}

func (h *NotificationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	id, err := parsePositiveID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	found, err := h.store.MarkRead(r.Context(), id, actor.UserID)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "not_found", "Notifikasi tidak ditemukan")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *NotificationHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeUnauthorized(w, "Token tidak valid")
		return
	}
	n, err := h.store.MarkAllRead(r.Context(), actor.UserID)
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"updated": n})
}

func (h *NotificationHandler) internalError(w http.ResponseWriter, r *http.Request, err error) {
	slog.ErrorContext(r.Context(), "notification handler", "path", r.URL.Path, "error", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan internal")
}

func parseOptionalBool(raw string) (bool, error) {
	if raw == "" {
		return false, nil
	}
	return strconv.ParseBool(raw)
}

// parseBoundedInt returns def for an empty value, and ok=false for a
// non-integer or a value outside [min, max].
func parseBoundedInt(raw string, def, min, max int32) (int32, bool) {
	if raw == "" {
		return def, true
	}
	v, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || int32(v) < min || int32(v) > max {
		return 0, false
	}
	return int32(v), true
}
