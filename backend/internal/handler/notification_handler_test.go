package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/pkg/middleware"
)

type fakeNotificationStore struct {
	rows     []db.ListNotificationsForUserRow
	total    int64
	unread   int64
	owned    map[int64]int64 // notification id -> owner user id
	lastUser int64
	lastArgs [3]any // unreadOnly, page, pageSize
	err      error
}

func (f *fakeNotificationStore) List(_ context.Context, userID int64, unreadOnly bool, page, pageSize int32) ([]db.ListNotificationsForUserRow, int64, error) {
	f.lastUser, f.lastArgs = userID, [3]any{unreadOnly, page, pageSize}
	return f.rows, f.total, f.err
}

func (f *fakeNotificationStore) UnreadCount(_ context.Context, userID int64) (int64, error) {
	f.lastUser = userID
	return f.unread, f.err
}

func (f *fakeNotificationStore) MarkRead(_ context.Context, id, userID int64) (bool, error) {
	f.lastUser = userID
	return f.owned[id] == userID, f.err
}

func (f *fakeNotificationStore) MarkAllRead(_ context.Context, userID int64) (int64, error) {
	f.lastUser = userID
	return 3, f.err
}

func notificationRequest(method, target string, userID int64) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	if userID != 0 {
		req = req.WithContext(middleware.WithAuthContext(req.Context(), &middleware.AuthContext{UserID: userID, Role: "ATM-SPV"}))
	}
	return req
}

func serveNotification(store *fakeNotificationStore, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	NewNotificationHandler(store).Routes().ServeHTTP(rec, req)
	return rec
}

func TestNotificationHandler_ListUsesCallerAndDefaults(t *testing.T) {
	link := "/replenishment/vendor-requests/9"
	created := time.Date(2026, 10, 7, 2, 0, 0, 0, time.UTC)
	store := &fakeNotificationStore{total: 1, rows: []db.ListNotificationsForUserRow{{
		ID: 5, Type: "visit_quota.over_quota", Title: "Kelebihan kuota kunjungan", Body: "isi", Link: &link,
		CreatedAt: pgtype.Timestamptz{Time: created, Valid: true},
	}}}

	rec := serveNotification(store, notificationRequest(http.MethodGet, "/", 42))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	if store.lastUser != 42 || store.lastArgs != [3]any{false, int32(1), int32(20)} {
		t.Fatalf("store called with user %d args %v", store.lastUser, store.lastArgs)
	}
	var body struct {
		Items []struct {
			ID        int64   `json:"id"`
			Type      string  `json:"type"`
			Title     string  `json:"title"`
			Link      *string `json:"link"`
			IsRead    bool    `json:"is_read"`
			CreatedAt string  `json:"created_at"`
		} `json:"items"`
		Total    int64 `json:"total"`
		Page     int   `json:"page"`
		PageSize int   `json:"page_size"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Total != 1 || body.Page != 1 || body.PageSize != 20 || len(body.Items) != 1 {
		t.Fatalf("body = %+v", body)
	}
	if it := body.Items[0]; it.ID != 5 || it.Link == nil || *it.Link != link || it.CreatedAt != "2026-10-07T02:00:00Z" {
		t.Fatalf("item = %+v", it)
	}
}

func TestNotificationHandler_ListParsesQuery(t *testing.T) {
	store := &fakeNotificationStore{}
	rec := serveNotification(store, notificationRequest(http.MethodGet, "/?unread_only=true&page=3&page_size=100", 1))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if store.lastArgs != [3]any{true, int32(3), int32(100)} {
		t.Fatalf("args = %v", store.lastArgs)
	}
}

func TestNotificationHandler_BadRequests(t *testing.T) {
	for _, target := range []string{
		"/?page=0", "/?page=x", "/?page_size=101", "/?page_size=0", "/?unread_only=maybe",
	} {
		rec := serveNotification(&fakeNotificationStore{}, notificationRequest(http.MethodGet, target, 1))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400", target, rec.Code)
		}
	}
	rec := serveNotification(&fakeNotificationStore{}, notificationRequest(http.MethodPost, "/abc/read", 1))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad id: status = %d, want 400", rec.Code)
	}
}

func TestNotificationHandler_Unauthorized(t *testing.T) {
	for _, tc := range []struct{ method, target string }{
		{http.MethodGet, "/"}, {http.MethodGet, "/unread-count"},
		{http.MethodPost, "/1/read"}, {http.MethodPost, "/read-all"},
	} {
		rec := serveNotification(&fakeNotificationStore{}, notificationRequest(tc.method, tc.target, 0))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: status = %d, want 401", tc.method, tc.target, rec.Code)
		}
	}
}

func TestNotificationHandler_UnreadCount(t *testing.T) {
	rec := serveNotification(&fakeNotificationStore{unread: 7}, notificationRequest(http.MethodGet, "/unread-count", 1))
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"unread_count\":7}\n" {
		t.Fatalf("status %d body %q", rec.Code, rec.Body)
	}
}

func TestNotificationHandler_MarkReadOwnOnly(t *testing.T) {
	store := &fakeNotificationStore{owned: map[int64]int64{5: 42, 6: 99}}
	if rec := serveNotification(store, notificationRequest(http.MethodPost, "/5/read", 42)); rec.Code != http.StatusNoContent {
		t.Fatalf("own: status = %d, want 204", rec.Code)
	}
	if rec := serveNotification(store, notificationRequest(http.MethodPost, "/6/read", 42)); rec.Code != http.StatusNotFound {
		t.Fatalf("someone else's: status = %d, want 404", rec.Code)
	}
}

func TestNotificationHandler_ReadAll(t *testing.T) {
	rec := serveNotification(&fakeNotificationStore{}, notificationRequest(http.MethodPost, "/read-all", 1))
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"updated\":3}\n" {
		t.Fatalf("status %d body %q", rec.Code, rec.Body)
	}
}

func TestNotificationHandler_StoreErrorIs500(t *testing.T) {
	rec := serveNotification(&fakeNotificationStore{err: errors.New("db down")}, notificationRequest(http.MethodGet, "/unread-count", 1))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
