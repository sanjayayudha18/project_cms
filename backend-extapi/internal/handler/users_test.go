package handler

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/cimb-niaga/cms/backend-extapi/internal/extuser"
	"github.com/cimb-niaga/cms/pkg/auth"
	"github.com/cimb-niaga/cms/pkg/userrules"
)

func TestWriteServiceErrorStatusMapping(t *testing.T) {
	for _, c := range []struct {
		name   string
		err    error
		status int
		code   string
		field  string
	}{
		{"validation", &auth.ValidationError{Field: "email", Message: "format salah"}, 422, "validation_error", "email"},
		{"wrapped validation", fmt.Errorf("x: %w", &auth.ValidationError{Field: "role", Message: "m"}), 422, "validation_error", "role"},
		{"reference", &userrules.ReferenceError{Field: "role", Message: "tidak ada"}, 400, "invalid_reference", ""},
		{"conflict", &userrules.ConflictError{Field: "email", Message: "sudah dipakai"}, 409, "conflict", ""},
		{"self supervision", extuser.ErrSelfSupervision, 400, "bad_request", ""},
		{"role not allowed", extuser.ErrRoleNotAllowed, 403, "role_not_allowed", ""},
		{"not found", extuser.ErrUserNotFound, 404, "not_found", ""},
		{"other", errors.New("pq: relation users broke at 10.7.7.7"), 500, "internal_error", ""},
	} {
		w := httptest.NewRecorder()
		writeServiceError(w, httptest.NewRequest("GET", "/x", nil), c.err)
		var body apiError
		mustJSON(t, w.Body.Bytes(), &body)
		if w.Code != c.status || body.Error != c.code || body.Field != c.field || w.Header().Get("Content-Type") != jsonCT {
			t.Errorf("%s: %d %+v", c.name, w.Code, body)
		}
		if c.status == 500 && strings.Contains(w.Body.String(), "10.7.7.7") {
			t.Errorf("%s: 5xx leaks the cause: %s", c.name, w.Body)
		}
	}
}

func TestUserCRUDThroughRouter(t *testing.T) {
	e := newEnv(t)
	tok := e.tokenFor(t, "")

	// Create → 201, Location, actor built from the verified client.
	w := e.api(http.MethodPost, usersPath, `{"username":"alice","full_name":"Alice A","email":"alice@example.id","role":"ATM-USER","employee_id":"E100","supervisor_id":3,"approval_level":2}`, tok)
	if w.Code != 201 || w.Header().Get("Location") != usersPath+"/7" {
		t.Fatalf("create: %d %v %s", w.Code, w.Header(), w.Body)
	}
	a := e.users.actor
	if a.ServiceUserID != serviceUID || a.ClientID != e.id || a.IP != testIP || len(a.AssignableRoles) != 1 || a.AssignableRoles[0] != "ATM-USER" {
		t.Errorf("actor: %+v", a)
	}
	cr := e.users.cr
	if cr.Username != "alice" || cr.Role != "ATM-USER" || cr.EmployeeID != "E100" || cr.SupervisorID == nil || *cr.SupervisorID != 3 || cr.ApprovalLevel == nil || *cr.ApprovalLevel != 2 {
		t.Errorf("create request: %+v", cr)
	}

	// Wire shape: exactly the ExtUser fields, nothing internal.
	var m map[string]any
	mustJSON(t, w.Body.Bytes(), &m)
	if len(m) != 11 {
		t.Errorf("wire fields: %v", m)
	}
	for _, banned := range []string{"auth_source", "deleted_at", "password", "password_hash"} {
		if _, ok := m[banned]; ok {
			t.Errorf("wire exposes %s", banned)
		}
	}
	if m["created_at"] != "2026-11-01T03:00:00Z" {
		t.Errorf("created_at: %v", m["created_at"])
	}

	// Update → 200
	w = e.api(http.MethodPut, usersPath+"/7", `{"full_name":"Alice B","email":"a@x.id","role":"ATM-USER"}`, tok)
	if w.Code != 200 || e.users.id != 7 || e.users.ur.FullName != "Alice B" {
		t.Errorf("update: %d %s %+v", w.Code, w.Body, e.users.ur)
	}
	// Disable → 204, empty body
	w = e.api(http.MethodDelete, usersPath+"/7", "", tok)
	if w.Code != 204 || w.Body.Len() != 0 || e.users.calls[len(e.users.calls)-1] != "disable" {
		t.Errorf("disable: %d %q", w.Code, w.Body)
	}
	// Enable → 200
	w = e.api(http.MethodPost, usersPath+"/7/enable", "", tok)
	if w.Code != 200 || e.users.calls[len(e.users.calls)-1] != "enable" {
		t.Errorf("enable: %d %s", w.Code, w.Body)
	}
	// Get / find by username → 200
	if w = e.api(http.MethodGet, usersPath+"/7", "", tok); w.Code != 200 || !strings.Contains(w.Body.String(), `"alice"`) {
		t.Errorf("get: %d %s", w.Code, w.Body)
	}
	if w = e.api(http.MethodGet, usersPath+"?username="+url.QueryEscape("alice"), "", tok); w.Code != 200 || e.users.name != "alice" {
		t.Errorf("find: %d %s %q", w.Code, w.Body, e.users.name)
	}
}

func TestUserServiceErrorsOverHTTP(t *testing.T) {
	e := newEnv(t)
	tok := e.tokenFor(t, "")
	e.users.err = extuser.ErrUserNotFound
	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, usersPath + "/7", ""},
		{http.MethodGet, usersPath + "?username=ghost", ""},
		{http.MethodPut, usersPath + "/7", `{"full_name":"x"}`},
		{http.MethodDelete, usersPath + "/7", ""},
		{http.MethodPost, usersPath + "/7/enable", ""},
		{http.MethodPost, usersPath, `{"username":"x"}`},
	} {
		w := e.api(c.method, c.path, c.body, tok)
		if w.Code != 404 || w.Header().Get("Location") != "" {
			t.Errorf("%s %s: %d %s", c.method, c.path, w.Code, w.Body)
		}
	}
}

func TestUserRequestValidation(t *testing.T) {
	e := newEnv(t)
	tok := e.tokenFor(t, "")
	e.users.calls = nil
	for _, c := range []struct {
		name, method, path, body, ct string
		status                       int
		contains                     string
	}{
		{"id not a number", "GET", usersPath + "/abc", "", "", 400, "positive integer"},
		{"id zero", "GET", usersPath + "/0", "", "", 400, "positive integer"},
		{"id negative", "DELETE", usersPath + "/-5", "", "", 400, "positive integer"},
		{"id overflow", "POST", usersPath + "/99999999999999999999/enable", "", "", 400, "positive integer"},
		{"update bad id", "PUT", usersPath + "/x", `{}`, jsonCT, 400, "positive integer"},
		{"username missing", "GET", usersPath, "", "", 400, "username"},
		{"username blank", "GET", usersPath + "?username=%20%20", "", "", 400, "username"},
		{"username too long", "GET", usersPath + "?username=" + strings.Repeat("a", 101), "", "", 400, "username"},
		{"create not json", "POST", usersPath, `username=a`, "text/plain", 415, "application/json"},
		{"create no content type", "POST", usersPath, `{}`, "", 415, "application/json"},
		{"update not json", "PUT", usersPath + "/7", `{}`, formCT, 415, "application/json"},
		{"unknown field", "POST", usersPath, `{"bogus":1}`, jsonCT, 400, "unknown field"},
		{"password field refused", "POST", usersPath, `{"username":"a","password":"x"}`, jsonCT, 400, "unknown field"},
		{"wrong type", "POST", usersPath, `{"username":5}`, jsonCT, 400, `invalid type for field "username"`},
		{"malformed json", "POST", usersPath, `{"username":`, jsonCT, 400, "malformed JSON body"},
		{"trailing data", "POST", usersPath, `{} {}`, jsonCT, 400, "single JSON object"},
		{"empty body", "POST", usersPath, ``, jsonCT, 400, "malformed JSON body"},
	} {
		r := req(c.method, c.path, c.body, c.ct, withBearer(tok))
		w := e.serve(r)
		var body apiError
		mustJSON(t, w.Body.Bytes(), &body)
		if w.Code != c.status || !strings.Contains(body.Message, c.contains) {
			t.Errorf("%s: %d %+v", c.name, w.Code, body)
		}
	}
	if len(e.users.calls) != 0 {
		t.Errorf("service called for invalid input: %v", e.users.calls)
	}
}

func TestUserBodyTooLarge(t *testing.T) {
	e := newEnv(t)
	tok := e.tokenFor(t, "")
	body := `{"full_name":"` + strings.Repeat("a", maxUserBody) + `"}`
	w := e.api(http.MethodPost, usersPath, body, tok)
	if w.Code != 400 || strings.Contains(w.Body.String(), "aaaa") {
		t.Fatalf("%d %.200s", w.Code, w.Body)
	}
}

func TestInternalErrorBodyHasNoCause(t *testing.T) {
	e := newEnv(t)
	tok := e.tokenFor(t, "")
	e.users.err = errors.New("pq: deadlock detected on users at db-primary.internal")
	w := e.api(http.MethodGet, usersPath+"/7", "", tok)
	if w.Code != 500 || strings.Contains(w.Body.String(), "deadlock") || !strings.Contains(w.Body.String(), "internal_error") {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestHandlerWithoutClientContextIs500(t *testing.T) {
	h := NewUserHandler(&fakeUsers{})
	for name, fn := range map[string]http.HandlerFunc{"create": h.Create, "update": h.Update, "disable": h.Disable, "enable": h.Enable} {
		r := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		fn(w, r)
		if w.Code != 500 || !strings.Contains(w.Body.String(), "internal_error") {
			t.Errorf("%s: %d %s", name, w.Code, w.Body)
		}
	}
}
