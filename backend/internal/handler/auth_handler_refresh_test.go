package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	pkgauth "github.com/cimb-niaga/cms/pkg/auth"
)

type fakeProfileRepo struct{ pkgauth.UserRepository }

func (fakeProfileRepo) GetUserProfile(_ context.Context, id int64) (*pkgauth.UserRecord, error) {
	return &pkgauth.UserRecord{ID: id, Username: "u", Role: "ATM-USER"}, nil
}

func newRefreshHandler(life time.Duration) (*AuthHandler, *pkgauth.TokenService) {
	ts := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: life,
	}, noopBlacklist{})
	return &AuthHandler{tokenService: ts, userRepo: fakeProfileRepo{}}, ts
}

func doRefreshDirect(h *AuthHandler, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: token})
	rec := httptest.NewRecorder()
	h.Refresh(rec, req)
	return rec
}

func refreshCookie(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == "refresh_token" {
			return c
		}
	}
	return nil
}

func TestRefresh_InheritsDeadlineAndCookieMaxAge(t *testing.T) {
	h, ts := newRefreshHandler(time.Hour)
	_, rt, _ := ts.RotateTokenPair(&pkgauth.AuthIdentity{UserID: 1, Username: "u", Role: "ATM-USER"}, time.Now().Add(30*time.Minute))

	rec := doRefreshDirect(h, rt)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	c := refreshCookie(rec)
	if c == nil || c.MaxAge < 1790 || c.MaxAge > 1800 {
		t.Fatalf("cookie = %+v, want Max-Age ~1800", c)
	}
}

func TestRefresh_SessionExpired_401AndClearsCookie(t *testing.T) {
	h, _ := newRefreshHandler(time.Hour)
	expiredSvc := pkgauth.NewTokenService(pkgauth.TokenConfig{
		SecretKey:          []byte("test-secret-minimum-32-bytes-long!!"),
		SessionMaxLifetime: -time.Second,
	}, noopBlacklist{})
	_, rt, _ := expiredSvc.GenerateTokenPair(&pkgauth.AuthIdentity{UserID: 1, Username: "u", Role: "ATM-USER"})

	rec := doRefreshDirect(h, rt)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if c := refreshCookie(rec); c == nil || c.MaxAge != -1 {
		t.Fatalf("cookie = %+v, want Max-Age=-1", c)
	}
}
