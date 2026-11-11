//go:build integration

// Integration tests for the extclient CLI (spec ext-user-api, task 8.2): every
// command against a real DB, actor and grant rejections, one audit row per
// action without secret or hash (Property 6, CLI side), and a token issued
// before rotate rejected by extauth afterwards (Req 1.4).
//
// The CLI commits for real, so every row is created under a per-run prefix
// (it82-<hex>) and deleted in t.Cleanup. A generated secret is never logged:
// failure messages show stderr only, never stdout.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/cimb-niaga/cms/backend-extapi/internal/extauth"
	"github.com/cimb-niaga/cms/backend-extapi/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type itEnv struct {
	ctx      context.Context
	pool     *pgxpool.Pool
	dbURL    string
	prefix   string // it82-<hex>, unique per run
	actor    string // active APPACCESS
	actorID  int64
	nonAdmin string // active ATM-USER
	inactive string // inactive APPACCESS
}

func newEnv(t *testing.T) *itEnv {
	t.Helper()
	url := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	e := &itEnv{ctx: ctx, pool: pool, dbURL: url, prefix: "it82-" + hex.EncodeToString(b)}
	t.Cleanup(func() { e.cleanup(t); pool.Close() })

	e.actor, e.actorID = e.insertUser(t, "admin", "APPACCESS", true)
	e.nonAdmin, _ = e.insertUser(t, "atm", "ATM-USER", true)
	e.inactive, _ = e.insertUser(t, "off", "APPACCESS", false)
	return e
}

func (e *itEnv) insertUser(t *testing.T, suffix, role string, active bool) (string, int64) {
	t.Helper()
	username := e.prefix + "-" + suffix
	var id int64
	err := e.pool.QueryRow(e.ctx, `
		INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source, is_active)
		SELECT r.id, $1, $1, $1 || '@example.test', true, 'ldap', $3
		FROM roles r WHERE r.role = $2
		RETURNING id`, username, role, active).Scan(&id)
	if err != nil {
		t.Fatalf("insert actor %s (%s): %v", username, role, err)
	}
	return username, id
}

// cleanup deletes only rows this run created: audit rows by its actors, its
// clients, its service accounts (svc-<prefix>...) and its actor users.
func (e *itEnv) cleanup(t *testing.T) {
	like := e.prefix + "%"
	stmts := []string{
		`DELETE FROM audit_logs WHERE actor_id IN (SELECT id FROM users WHERE username LIKE $1)`,
		`DELETE FROM api_clients WHERE name LIKE $1`,
		`DELETE FROM users WHERE username LIKE 'svc-' || $1`,
		`DELETE FROM users WHERE username LIKE $1`,
	}
	for _, s := range stmts {
		if _, err := e.pool.Exec(e.ctx, s, like); err != nil {
			t.Errorf("cleanup %q: %v", s, err)
		}
	}
}

func (e *itEnv) getenv(k string) string {
	if k == "DATABASE_URL" {
		return e.dbURL
	}
	return ""
}

// cli runs one command; stdout may hold a secret, so callers never log it.
func (e *itEnv) cli(args ...string) (code int, stdout, stderr string) {
	var out, errb bytes.Buffer
	code = run(e.ctx, args, e.getenv, &out, &errb)
	return code, out.String(), errb.String()
}

func (e *itEnv) mustCLI(t *testing.T, args ...string) string {
	t.Helper()
	code, out, errOut := e.cli(args...)
	if code != 0 {
		t.Fatalf("extclient %s: exit %d, stderr: %s", args[0], code, errOut)
	}
	return out
}

// field reads "key: value" from CLI output.
func field(t *testing.T, out, key string) string {
	t.Helper()
	for _, l := range strings.Split(out, "\n") {
		if k, v, ok := strings.Cut(l, ":"); ok && strings.TrimSpace(k) == key {
			return strings.TrimSpace(v)
		}
	}
	t.Fatalf("output has no %q line", key)
	return ""
}

type created struct{ name, clientID, secret, svcUsername string }

func (e *itEnv) create(t *testing.T, suffix string, extra ...string) created {
	t.Helper()
	name := e.prefix + "-" + suffix
	args := append([]string{"create", "--name", name, "--scopes", "users:read,users:write", "--actor", e.actor}, extra...)
	out := e.mustCLI(t, args...)
	return created{name: name, clientID: field(t, out, "client_id"), secret: field(t, out, "client_secret"), svcUsername: field(t, out, "service_username")}
}

type auditRow struct {
	actorID       int64
	action        string
	before, after string
	ip            string
}

func (e *itEnv) audits(t *testing.T, entityType string, entityID int64) []auditRow {
	t.Helper()
	rows, err := e.pool.Query(e.ctx, `
		SELECT actor_id, action, COALESCE(before::text, ''), COALESCE(after::text, ''), COALESCE(ip, '')
		FROM audit_logs WHERE entity_type = $1 AND entity_id = $2 ORDER BY id`, entityType, entityID)
	if err != nil {
		t.Fatalf("audits: %v", err)
	}
	defer rows.Close()
	var out []auditRow
	for rows.Next() {
		var a auditRow
		if err := rows.Scan(&a.actorID, &a.action, &a.before, &a.after, &a.ip); err != nil {
			t.Fatal(err)
		}
		out = append(out, a)
	}
	return out
}

func actions(rows []auditRow) []string {
	out := make([]string, len(rows))
	for i, a := range rows {
		out[i] = a.action
	}
	return out
}

func (e *itEnv) client(t *testing.T, clientID string) *extauth.Client {
	t.Helper()
	c, err := repository.NewApiClientRepository(e.pool).GetByClientID(e.ctx, clientID)
	if err != nil || c == nil {
		t.Fatalf("get client: %v (nil=%v)", err, c == nil)
	}
	return c
}

// assertNoSecret checks Property 6 on the CLI side: no audit payload carries
// the secret, the stored hash, or a secret field. Never prints the payload.
func assertNoSecret(t *testing.T, rows []auditRow, secret, hash string) {
	t.Helper()
	for _, a := range rows {
		for _, p := range []string{a.before, a.after} {
			if (secret != "" && strings.Contains(p, secret)) || (hash != "" && strings.Contains(p, hash)) ||
				strings.Contains(p, "secret_hash") || strings.Contains(p, "client_secret") {
				t.Errorf("audit %s payload leaks a secret or hash", a.action)
			}
		}
	}
}

func (e *itEnv) countClients(t *testing.T, name string) int {
	t.Helper()
	var n int
	if err := e.pool.QueryRow(e.ctx, `SELECT count(*) FROM api_clients WHERE name = $1`, name).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestCreate_WritesClientServiceAccountAndAudit(t *testing.T) {
	e := newEnv(t)
	cr := e.create(t, "create", "--roles", "ATM-USER", "--cidrs", "10.20.0.0/16")

	if !strings.HasPrefix(cr.svcUsername, "svc-"+e.prefix) {
		t.Errorf("service_username = %q, want svc-%s...", cr.svcUsername, e.prefix)
	}
	c := e.client(t, cr.clientID)
	if !c.IsActive || c.CredentialVersion != 1 || c.Name != cr.name {
		t.Errorf("client: active=%v ver=%d name=%q", c.IsActive, c.CredentialVersion, c.Name)
	}
	if strings.Join(c.Scopes, ",") != "users:read,users:write" || strings.Join(c.AssignableRoles, ",") != "ATM-USER" ||
		len(c.AllowedCIDRs) != 1 || c.AllowedCIDRs[0].String() != "10.20.0.0/16" {
		t.Errorf("grants: scopes=%v roles=%v cidrs=%v", c.Scopes, c.AssignableRoles, c.AllowedCIDRs)
	}
	if c.SecretHash == cr.secret || !extauth.VerifySecret(c.SecretHash, cr.secret) {
		t.Error("stored hash does not verify the printed secret (or the secret is stored in clear)")
	}

	var svcName, authSource, role string
	var active bool
	var pw *string
	if err := e.pool.QueryRow(e.ctx, `
		SELECT u.username, u.auth_source, r.role, u.is_active, u.password_hash
		FROM users u JOIN roles r ON r.id = u.role_id WHERE u.id = $1`, c.ServiceUserID).
		Scan(&svcName, &authSource, &role, &active, &pw); err != nil {
		t.Fatal(err)
	}
	if svcName != cr.svcUsername || authSource != "service" || role != "EXT-CLIENT" || active || pw != nil {
		t.Errorf("service user: %s %s %s active=%v hasPassword=%v", svcName, authSource, role, active, pw != nil)
	}

	clientAudit := e.audits(t, "api_client", c.ID)
	userAudit := e.audits(t, "user", c.ServiceUserID)
	if got := actions(clientAudit); len(got) != 1 || got[0] != "api_client_created" {
		t.Fatalf("api_client audit = %v, want one api_client_created", got)
	}
	if got := actions(userAudit); len(got) != 1 || got[0] != "user_created" {
		t.Fatalf("user audit = %v, want one user_created", got)
	}
	for _, a := range append(clientAudit, userAudit...) {
		if a.actorID != e.actorID || a.ip != "cli" {
			t.Errorf("%s: actor=%d ip=%q, want %d cli", a.action, a.actorID, a.ip, e.actorID)
		}
	}
	assertNoSecret(t, append(clientAudit, userAudit...), cr.secret, c.SecretHash)

	t.Run("duplicate name rejected", func(t *testing.T) {
		code, _, errOut := e.cli("create", "--name", cr.name, "--scopes", "users:read", "--actor", e.actor)
		if code != 1 || !strings.Contains(errOut, "already exists") {
			t.Errorf("exit %d stderr %q, want 1 already exists", code, errOut)
		}
		if n := e.countClients(t, cr.name); n != 1 {
			t.Errorf("clients named %s = %d, want 1", cr.name, n)
		}
	})
}

func TestCreate_EmptyCIDRsWarns(t *testing.T) {
	e := newEnv(t)
	code, _, errOut := e.cli("create", "--name", e.prefix+"-nocidr", "--scopes", "users:read", "--actor", e.actor)
	if code != 0 || !strings.Contains(errOut, "WARNING: --cidrs is empty") {
		t.Errorf("exit %d, stderr %q; want 0 with the empty-CIDR warning", code, errOut)
	}
}

func TestActorMustBeActiveAppAccess(t *testing.T) {
	e := newEnv(t)
	cr := e.create(t, "actor")
	for _, tc := range []struct{ name, actor string }{
		{"non-APPACCESS", e.nonAdmin},
		{"inactive-APPACCESS", e.inactive},
		{"unknown-user", e.prefix + "-nobody"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			name := e.prefix + "-rej-" + tc.name
			cmds := [][]string{
				{"create", "--name", name, "--scopes", "users:read", "--actor", tc.actor},
				{"rotate", "--client-id", cr.clientID, "--actor", tc.actor},
				{"disable", "--client-id", cr.clientID, "--actor", tc.actor},
				{"enable", "--client-id", cr.clientID, "--actor", tc.actor},
				{"set-grants", "--client-id", cr.clientID, "--scopes", "users:read", "--actor", tc.actor},
			}
			for _, args := range cmds {
				code, _, errOut := e.cli(args...)
				if code != 1 || !strings.Contains(errOut, "is not an active APPACCESS user") {
					t.Errorf("%s: exit %d stderr %q, want 1 actor rejected", args[0], code, errOut)
				}
			}
			if n := e.countClients(t, name); n != 0 {
				t.Errorf("rejected create left %d clients", n)
			}
		})
	}
	c := e.client(t, cr.clientID)
	if c.CredentialVersion != 1 || !c.IsActive || len(c.Scopes) != 2 {
		t.Errorf("client changed by rejected actors: ver=%d active=%v scopes=%v", c.CredentialVersion, c.IsActive, c.Scopes)
	}
	if got := actions(e.audits(t, "api_client", c.ID)); len(got) != 1 {
		t.Errorf("audit = %v, want only the create", got)
	}
}

func TestPrivilegedRolesAndOpenCIDRRejected(t *testing.T) {
	e := newEnv(t)
	cr := e.create(t, "grants")
	bad := []struct{ name, flag, value string }{
		{"ADMIN", "--roles", "ATM-USER,ADMIN"},
		{"APPACCESS", "--roles", "APPACCESS"},
		{"ADMIN_PARAM", "--roles", "ADMIN_PARAM"},
		{"EXT-CLIENT", "--roles", "EXT-CLIENT"},
		{"ipv4-slash0", "--cidrs", "0.0.0.0/0"},
		{"ipv6-slash0", "--cidrs", "10.0.0.1/32,::/0"},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			name := e.prefix + "-bad-" + tc.name
			code, _, _ := e.cli("create", "--name", name, "--scopes", "users:read", tc.flag, tc.value, "--actor", e.actor)
			if code != 2 {
				t.Errorf("create: exit %d, want 2", code)
			}
			if n := e.countClients(t, name); n != 0 {
				t.Errorf("create left %d clients", n)
			}
			code, _, _ = e.cli("set-grants", "--client-id", cr.clientID, tc.flag, tc.value, "--actor", e.actor)
			if code != 2 {
				t.Errorf("set-grants: exit %d, want 2", code)
			}
		})
	}
	t.Run("unknown role", func(t *testing.T) {
		code, _, errOut := e.cli("set-grants", "--client-id", cr.clientID, "--roles", "NO-SUCH-ROLE-IT82", "--actor", e.actor)
		if code != 1 || !strings.Contains(errOut, "does not exist") {
			t.Errorf("exit %d stderr %q, want 1 role does not exist", code, errOut)
		}
	})
	c := e.client(t, cr.clientID)
	if len(c.AssignableRoles) != 0 || len(c.AllowedCIDRs) != 0 {
		t.Errorf("grants changed by rejected calls: roles=%v cidrs=%v", c.AssignableRoles, c.AllowedCIDRs)
	}
	if got := actions(e.audits(t, "api_client", c.ID)); len(got) != 1 {
		t.Errorf("audit = %v, want only the create", got)
	}
}

func TestSetGrants_ReplacesListsAndAudits(t *testing.T) {
	e := newEnv(t)
	cr := e.create(t, "setg", "--roles", "ATM-USER", "--cidrs", "10.0.0.0/8")
	c := e.client(t, cr.clientID)

	out := e.mustCLI(t, "set-grants", "--client-id", cr.clientID, "--scopes", "users:read", "--cidrs", "192.168.1.0/24,10.9.9.9/32", "--actor", e.actor)
	if !strings.Contains(out, "grants updated") {
		t.Errorf("stdout = %q", out)
	}
	got := e.client(t, cr.clientID)
	if strings.Join(got.Scopes, ",") != "users:read" || strings.Join(got.AssignableRoles, ",") != "ATM-USER" || len(got.AllowedCIDRs) != 2 {
		t.Errorf("after set-grants: scopes=%v roles=%v (kept) cidrs=%v", got.Scopes, got.AssignableRoles, got.AllowedCIDRs)
	}

	_, _, errOut := e.cli("set-grants", "--client-id", cr.clientID, "--cidrs", "", "--roles", "", "--actor", e.actor)
	if !strings.Contains(errOut, "WARNING: --cidrs is empty") {
		t.Errorf("clearing CIDRs should warn, stderr %q", errOut)
	}
	got = e.client(t, cr.clientID)
	if len(got.AssignableRoles) != 0 || len(got.AllowedCIDRs) != 0 {
		t.Errorf("after clear: roles=%v cidrs=%v", got.AssignableRoles, got.AllowedCIDRs)
	}

	rows := e.audits(t, "api_client", c.ID)
	want := []string{"api_client_created", "api_client_grants_updated", "api_client_grants_updated"}
	if strings.Join(actions(rows), ",") != strings.Join(want, ",") {
		t.Fatalf("audit = %v, want %v", actions(rows), want)
	}
	if !strings.Contains(rows[1].before, "10.0.0.0/8") || !strings.Contains(rows[1].after, "192.168.1.0/24") {
		t.Errorf("grants audit before/after wrong: %s -> %s", rows[1].before, rows[1].after)
	}
	assertNoSecret(t, rows, cr.secret, c.SecretHash)
}

func TestDisableEnable_TogglesAndAuditsOncePerChange(t *testing.T) {
	e := newEnv(t)
	cr := e.create(t, "toggle")
	id := e.client(t, cr.clientID).ID

	steps := []struct {
		cmd        string
		wantActive bool
		wantOut    string
	}{
		{"disable", false, "disabled"},
		{"disable", false, "already disabled"},
		{"enable", true, "enabled"},
		{"enable", true, "already enabled"},
	}
	for _, s := range steps {
		out := e.mustCLI(t, s.cmd, "--client-id", cr.clientID, "--actor", e.actor)
		if !strings.Contains(out, s.wantOut) {
			t.Errorf("%s: stdout %q, want %q", s.cmd, out, s.wantOut)
		}
		if got := e.client(t, cr.clientID).IsActive; got != s.wantActive {
			t.Errorf("%s: is_active = %v, want %v", s.cmd, got, s.wantActive)
		}
	}
	rows := e.audits(t, "api_client", id)
	want := []string{"api_client_created", "api_client_disabled", "api_client_enabled"}
	if strings.Join(actions(rows), ",") != strings.Join(want, ",") {
		t.Fatalf("audit = %v, want %v (no-op calls write nothing)", actions(rows), want)
	}
	for _, a := range rows {
		if a.actorID != e.actorID {
			t.Errorf("%s by %d, want %d", a.action, a.actorID, e.actorID)
		}
	}

	code, _, errOut := e.cli("disable", "--client-id", "crown_aaaaaaaaaaaaaaaaaaaaaaaaaa", "--actor", e.actor)
	if code != 1 || !strings.Contains(errOut, "not found") {
		t.Errorf("unknown client: exit %d stderr %q", code, errOut)
	}
}

// requireClient runs extauth.RequireClient against the real api_clients row.
func requireClient(t *testing.T, e *itEnv, tokens *extauth.TokenService, rate *extauth.APIRate, token string) int {
	t.Helper()
	h := extauth.RequireClient(tokens, repository.NewApiClientRepository(e.pool), rate)(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	req := httptest.NewRequest(http.MethodGet, "/api/ext/v1/users", nil)
	req.RemoteAddr = "192.0.2.10:5555"
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

func TestRotate_NewSecretAndOldTokenRejected(t *testing.T) {
	e := newEnv(t)
	cr := e.create(t, "rotate", "--cidrs", "192.0.2.0/24")
	before := e.client(t, cr.clientID)

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	rate := extauth.NewAPIRate(rdb, 1000)
	tokens := extauth.NewTokenService([]byte("it82-test-key-0123456789abcdef0123456789"), "crown-ext-it82", 5*time.Minute)

	oldToken, _, err := tokens.Issue(*before, []string{extauth.ScopeUsersRead})
	if err != nil {
		t.Fatal(err)
	}
	if code := requireClient(t, e, tokens, rate, oldToken); code != http.StatusOK {
		t.Fatalf("token before rotate: %d, want 200", code)
	}

	out := e.mustCLI(t, "rotate", "--client-id", cr.clientID, "--actor", e.actor)
	newSecret := field(t, out, "client_secret")
	if field(t, out, "credential_version") != "2" {
		t.Error("rotate did not report credential_version 2")
	}
	after := e.client(t, cr.clientID)
	if after.CredentialVersion != 2 {
		t.Errorf("credential_version = %d, want 2", after.CredentialVersion)
	}
	if newSecret == cr.secret || !extauth.VerifySecret(after.SecretHash, newSecret) || extauth.VerifySecret(after.SecretHash, cr.secret) {
		t.Error("after rotate the new secret must verify and the old one must not")
	}

	if code := requireClient(t, e, tokens, rate, oldToken); code != http.StatusUnauthorized {
		t.Errorf("token from before rotate: %d, want 401", code)
	}
	newToken, _, err := tokens.Issue(*after, []string{extauth.ScopeUsersRead})
	if err != nil {
		t.Fatal(err)
	}
	if code := requireClient(t, e, tokens, rate, newToken); code != http.StatusOK {
		t.Errorf("token after rotate: %d, want 200", code)
	}

	rows := e.audits(t, "api_client", after.ID)
	if got := actions(rows); len(got) != 2 || got[1] != "api_client_secret_rotated" || rows[1].actorID != e.actorID {
		t.Fatalf("audit = %v, want create + one api_client_secret_rotated by the actor", got)
	}
	if !strings.Contains(rows[1].before, `"credential_version": 1`) || !strings.Contains(rows[1].after, `"credential_version": 2`) {
		t.Errorf("rotate audit versions: %s -> %s", rows[1].before, rows[1].after)
	}
	assertNoSecret(t, rows, cr.secret, before.SecretHash)
	assertNoSecret(t, rows, newSecret, after.SecretHash)

	t.Run("disabled client token rejected", func(t *testing.T) {
		e.mustCLI(t, "disable", "--client-id", cr.clientID, "--actor", e.actor)
		if code := requireClient(t, e, tokens, rate, newToken); code != http.StatusUnauthorized {
			t.Errorf("token of disabled client: %d, want 401", code)
		}
	})
}

func TestList_ShowsClientAndServiceAccount(t *testing.T) {
	e := newEnv(t)
	cr := e.create(t, "list")
	svcID := e.client(t, cr.clientID).ServiceUserID
	out := e.mustCLI(t, "list")
	if strings.Contains(out, cr.secret) {
		t.Fatal("list output contains the client secret")
	}
	var f []string
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, cr.clientID) {
			f = strings.Fields(l)
		}
	}
	if len(f) != 9 || f[1] != cr.name || f[2] != "true" || f[3] != "1" ||
		f[7] != strconv.FormatInt(svcID, 10) || f[8] != cr.svcUsername {
		t.Errorf("list row fields = %v", f)
	}
}
