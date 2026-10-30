//go:build integration

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend-exq/internal/audit"
	"github.com/cimb-niaga/cms/backend-exq/internal/exportjob"
	"github.com/cimb-niaga/cms/pkg/auth"
	"github.com/cimb-niaga/cms/pkg/csvexport"
	"github.com/cimb-niaga/cms/pkg/middleware"
)

// report-export-migration 6.2 (Properties 4, 5, 6) through the real router: RequireAuth with a real
// TokenService, the real ExportsHandler and Service, real Postgres. Every row carries the marker T62_
// (users, vendor, branch) and is deleted in t.Cleanup; a last cleanup step fails the test when anything
// with the marker is left. Pre-existing rows are never touched: roles are changed only on T62_ users.
// No worker runs here: a job is made "ready" with plain SQL (status succeeded + one chunk).

const (
	t62Like = `T62\_%`
	t62IP   = "10.62.0.1"
)

// t62Roles are all rows of the roles table at the time of writing (14).
var t62Roles = []string{
	"ADMIN", "ADMIN_PARAM", "APPACCESS", "ATM-USER", "ATM-SPV", "BRANCH-ATM-USER", "BRANCH-ATM-SPV",
	"BRANCH-USER", "BRANCH-SPV", "ACM-USER", "ACM-SPV", "VENDOR-USER", "VENDOR-ATM-USER", "VENDOR-VAULT-USER",
}

type t62User struct {
	id    int64
	name  string
	token string
}

type t62Env struct {
	t        *testing.T
	pool     *pgxpool.Pool
	ts       *auth.TokenService
	router   http.Handler
	vendorID int64
	branchID int64
	users    map[string]t62User // by role
}

func newT62Env(t *testing.T) *t62Env {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set -- skipping integration test")
	}
	url = strings.Replace(url, "host.docker.internal", "localhost", 1)
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatalf("parse DATABASE_URL: %v", err)
	}
	cfg.MaxConns = 6
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping db: %v", err)
	}
	e := &t62Env{t: t, pool: pool, users: map[string]t62User{}}
	e.ts = auth.NewTokenService(auth.TokenConfig{
		SecretKey:          []byte("t62-secret-key-of-at-least-32-bytes!"),
		AccessTokenExpiry:  15 * time.Minute,
		SessionMaxLifetime: time.Hour,
	}, nil)
	t.Cleanup(pool.Close)          // runs last
	t.Cleanup(e.assertNoLeftovers) // runs after cleanup
	t.Cleanup(e.cleanup)           // runs first
	e.cleanup()                    // stale rows of a crashed run
	e.seed()
	e.router = e.buildRouter()
	return e
}

func (e *t62Env) buildRouter() http.Handler {
	svc := exportjob.NewService(e.pool, audit.NewWriter(), nil)
	h := NewExportsHandler(svc)
	r := chi.NewRouter()
	r.Route("/api/v1/exports", func(r chi.Router) {
		r.Use(middleware.RequireAuth(e.ts))
		r.Get("/jobs/{jobID}/file", h.Download)
		r.Mount("/", h.Routes())
	})
	return r
}

func (e *t62Env) exec(sql string, args ...any) {
	e.t.Helper()
	if _, err := e.pool.Exec(context.Background(), sql, args...); err != nil {
		e.t.Fatalf("t62 exec: %v\n%s", err, sql)
	}
}

func (e *t62Env) scanInt(sql string, args ...any) int64 {
	e.t.Helper()
	var n int64
	if err := e.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		e.t.Fatalf("t62 query: %v\n%s", err, sql)
	}
	return n
}

func (e *t62Env) seed() {
	for _, role := range t62Roles {
		name := "T62_" + role
		id := e.scanInt(`INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source)
			SELECT id, $1, $1, $1 || '@test.local', true, 'ldap' FROM roles WHERE role = $2 RETURNING id`, name, role)
		e.users[role] = t62User{id: id, name: name, token: e.tokenFor(id, name, role)}
	}
	e.vendorID = e.scanInt(`INSERT INTO vendors (code, name, created_at) VALUES ('T62_V', 'T62 Vendor', '2026-01-01') RETURNING id`)
	e.branchID = e.scanInt(`INSERT INTO vendor_branches (vendor_id, branch_code, branch_name, created_at)
		VALUES ($1, 'T62_BR', 'T62 Branch', '2026-01-01') RETURNING id`, e.vendorID)
}

func (e *t62Env) tokenFor(id int64, name, role string) string {
	tok, _, err := e.ts.GenerateTokenPair(&auth.AuthIdentity{UserID: id, Username: name, Role: role, IsKaryawan: true})
	if err != nil {
		e.t.Fatalf("token for %s: %v", role, err)
	}
	return tok
}

const t62Users = `(SELECT id FROM users WHERE username LIKE 'T62\_%')`

func (e *t62Env) cleanup() {
	steps := []string{
		`DELETE FROM export_job_chunks WHERE export_job_id IN (SELECT id FROM export_jobs WHERE requested_by IN ` + t62Users + `)`,
		`DELETE FROM export_jobs WHERE requested_by IN ` + t62Users,
		`DELETE FROM audit_logs WHERE actor_id IN ` + t62Users,
		`DELETE FROM users WHERE username LIKE 'T62\_%'`,
		`DELETE FROM vendor_branches WHERE branch_code LIKE 'T62\_%'`,
		`DELETE FROM vendors WHERE code LIKE 'T62\_%'`,
	}
	for _, s := range steps {
		if _, err := e.pool.Exec(context.Background(), s); err != nil {
			e.t.Errorf("t62 cleanup: %v\n%s", err, s)
		}
	}
}

func (e *t62Env) assertNoLeftovers() {
	queries := []string{
		`SELECT count(*) FROM users WHERE username LIKE 'T62\_%'`,
		`SELECT count(*) FROM export_jobs WHERE requested_by IN ` + t62Users,
		`SELECT count(*) FROM export_job_chunks WHERE export_job_id NOT IN (SELECT id FROM export_jobs)`,
		`SELECT count(*) FROM audit_logs WHERE actor_id IN ` + t62Users,
		`SELECT count(*) FROM vendor_branches WHERE branch_code LIKE 'T62\_%'`,
		`SELECT count(*) FROM vendors WHERE code LIKE 'T62\_%'`,
	}
	total := int64(0)
	for _, q := range queries {
		n := e.scanInt(q)
		if n != 0 {
			e.t.Errorf("t62 leftover rows: %d from %s", n, q)
		}
		total += n
	}
	e.t.Logf("t62 leftover rows after cleanup: %d", total)
}

// dropJobs removes the jobs (and chunks) of one user so MaxActivePerUser and dedupe never interfere.
func (e *t62Env) dropJobs(userID int64) {
	e.exec(`DELETE FROM export_jobs WHERE requested_by = $1`, userID) // chunks cascade
}

func (e *t62Env) do(method, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.RemoteAddr = t62IP + ":4242"
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

// t62Request is the HTTP request of one Export_Kind.
type t62Request struct {
	kind exportjob.Kind
	path string
}

func (e *t62Env) requests() []t62Request {
	v, b := e.vendorID, e.branchID
	global := csvexport.ImportableNames()[0]
	const base = "/api/v1/exports"
	return []t62Request{
		{exportjob.KindVendor, fmt.Sprintf("%s/vendors/%d/pics", base, v)},
		{exportjob.KindCabang, fmt.Sprintf("%s/vendors/%d/branches/%d/pics", base, v, b)},
		{exportjob.KindHargaPaket, fmt.Sprintf("%s/vendors/%d/package-prices", base, v)},
		{exportjob.KindMasterDataGlobal, base + "/master-data/" + global},
		{exportjob.KindDsrLateDaily, base + "/reports/dsr-late-daily?month=2026-06"},
		{exportjob.KindDsrLateSummary, base + "/reports/dsr-late-summary?month=2026-06"},
		{exportjob.KindReplenishmentAmountMismatch, base + "/reports/replenishment-amount-mismatch?date_from=2026-06-01&date_to=2026-06-30"},
		{exportjob.KindReplenishmentTrips, base + "/reports/replenishment-trips?date_from=2026-06-01&date_to=2026-06-30"},
	}
}

// Property 4: allowed <=> role in KindSpecs[kind].Roles, for every role x every kind, through the router.
func TestReportExportRoleMatrixIntegration(t *testing.T) {
	e := newT62Env(t)

	// Master-data kinds stay ADMIN/ADMIN_PARAM whatever the report lists say.
	for _, k := range []exportjob.Kind{exportjob.KindVendor, exportjob.KindCabang, exportjob.KindHargaPaket, exportjob.KindMasterDataGlobal} {
		got := slices.Clone(exportjob.KindSpecs[k].Roles)
		slices.Sort(got)
		if !slices.Equal(got, []string{"ADMIN", "ADMIN_PARAM"}) {
			t.Fatalf("kind %s roles = %v, want ADMIN/ADMIN_PARAM", k, got)
		}
	}
	reqs := e.requests()
	if len(reqs) != 8 || len(t62Roles) != 14 {
		t.Fatalf("matrix is %d roles x %d kinds, want 14 x 8", len(t62Roles), len(reqs))
	}

	allowedPairs := 0
	for _, role := range t62Roles {
		u := e.users[role]
		for _, rq := range reqs {
			want := slices.Contains(exportjob.KindSpecs[rq.kind].Roles, role)
			t.Run(role+"/"+string(rq.kind), func(t *testing.T) {
				rec := e.do(http.MethodPost, rq.path, u.token)
				jobs := e.scanInt(`SELECT count(*) FROM export_jobs WHERE requested_by = $1`, u.id)
				audits := e.scanInt(`SELECT count(*) FROM audit_logs WHERE actor_id = $1`, u.id)
				if want {
					if rec.Code != http.StatusAccepted || jobs != 1 || audits != 1 {
						t.Errorf("allowed: status=%d jobs=%d audits=%d, want 202/1/1: %s", rec.Code, jobs, audits, rec.Body.String())
					}
				} else if rec.Code != http.StatusForbidden || jobs != 0 || audits != 0 {
					t.Errorf("denied: status=%d jobs=%d audits=%d, want 403/0/0: %s", rec.Code, jobs, audits, rec.Body.String())
				}
				e.dropJobs(u.id)
				e.exec(`DELETE FROM audit_logs WHERE actor_id = $1`, u.id)
			})
			if want {
				allowedPairs++
			}
		}
	}
	// ADMIN/ADMIN_PARAM: 4 master data each; ADMIN also 4 reports; APPACCESS none.
	wantPairs := 0
	for _, rq := range reqs {
		wantPairs += len(exportjob.KindSpecs[rq.kind].Roles)
	}
	if allowedPairs != wantPairs {
		t.Errorf("allowed pairs = %d, want %d", allowedPairs, wantPairs)
	}

	t.Run("no token", func(t *testing.T) {
		for _, rq := range reqs {
			if rec := e.do(http.MethodPost, rq.path, ""); rec.Code != http.StatusUnauthorized {
				t.Errorf("%s without token = %d, want 401", rq.kind, rec.Code)
			}
		}
	})
}

// makeReady turns a queued job into a succeeded one with a single chunk (what the worker would have done).
func (e *t62Env) makeReady(jobID int64, content string, rows int64) {
	e.t.Helper()
	e.exec(`UPDATE export_jobs SET status = 'succeeded', started_at = now(), finished_at = now(), expires_at = now() + interval '10 minutes',
		byte_size = $2, chunk_count = 1, row_count = $3 WHERE id = $1`, jobID, len(content), rows)
	e.exec(`INSERT INTO export_job_chunks (export_job_id, seq, data) VALUES ($1, 0, $2)`, jobID, []byte(content))
}

func (e *t62Env) createJob(u t62User, path string) int64 {
	e.t.Helper()
	rec := e.do(http.MethodPost, path, u.token)
	if rec.Code != http.StatusAccepted {
		e.t.Fatalf("create %s = %d: %s", path, rec.Code, rec.Body.String())
	}
	var b struct{ Data struct{ ID int64 } }
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil || b.Data.ID == 0 {
		e.t.Fatalf("create body: %v: %s", err, rec.Body.String())
	}
	return b.Data.ID
}

func (e *t62Env) setRole(userID int64, role string) {
	e.t.Helper()
	e.exec(`UPDATE users SET role_id = (SELECT id FROM roles WHERE role = $2) WHERE id = $1 AND username LIKE 'T62\_%'`, userID, role)
}

// Property 5: the role is re-read from the DB at download; the token's role no longer matters.
func TestReportExportRoleRecheckAtDownloadIntegration(t *testing.T) {
	e := newT62Env(t)
	u := e.users["ATM-SPV"]
	id := e.createJob(u, "/api/v1/exports/reports/dsr-late-daily?month=2026-06")
	e.makeReady(id, "hello", 1)
	file := fmt.Sprintf("/api/v1/exports/jobs/%d/file", id)

	e.setRole(u.id, "ACM-USER") // stale token still says ATM-SPV
	rec := e.do(http.MethodGet, file, u.token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("download after role change = %d, want 403: %s", rec.Code, rec.Body.String())
	}
	if n := e.scanInt(`SELECT count(*) FROM audit_logs WHERE actor_id = $1 AND action = 'report_exported'`, u.id); n != 0 {
		t.Errorf("report_exported rows after 403 = %d, want 0", n)
	}
	var status string
	if err := e.pool.QueryRow(context.Background(), `SELECT status FROM export_jobs WHERE id = $1`, id).Scan(&status); err != nil || status != "succeeded" {
		t.Errorf("job status after 403 = %q (%v), want succeeded (no claim)", status, err)
	}
	if n := e.scanInt(`SELECT count(*) FROM export_job_chunks WHERE export_job_id = $1`, id); n != 1 {
		t.Errorf("chunks after 403 = %d, want 1", n)
	}

	e.setRole(u.id, "ATM-SPV")
	rec = e.do(http.MethodGet, file, u.token)
	if rec.Code != http.StatusOK || rec.Body.String() != "hello" {
		t.Fatalf("download after role restored = %d %q, want 200 hello", rec.Code, rec.Body.String())
	}
	if n := e.scanInt(`SELECT count(*) FROM audit_logs WHERE actor_id = $1 AND action = 'report_exported'`, u.id); n != 1 {
		t.Errorf("report_exported rows after OK download = %d, want 1", n)
	}
}

// t62AuditRow is one audit_logs row as the Audit Log screen lists it.
type t62AuditRow struct {
	ID         int64
	Action     string
	EntityType string
	EntityID   int64
	After      map[string]any
	IP         *string
}

// listAuditLogs is the SQL text of backend/queries/audit_log.sql ListAuditLogs (the Audit Log screen
// query). backend/internal/db cannot be imported from this module, so the same statement runs raw.
const t62ListAuditLogs = `SELECT id, action, entity_type, entity_id, after, ip
FROM audit_logs
WHERE ($1::bigint       IS NULL OR actor_id    = $1)
  AND ($2::text         IS NULL OR action      = $2)
  AND ($3::text         IS NULL OR entity_type = $3)
  AND ($4::bigint       IS NULL OR entity_id   = $4)
  AND ($5::timestamptz  IS NULL OR created_at >= $5)
  AND ($6::timestamptz  IS NULL OR created_at <= $6)
ORDER BY created_at DESC, id DESC
LIMIT $7::int OFFSET $8::int`

func (e *t62Env) listAuditLogs(actorID int64, action string, entityType *string) []t62AuditRow {
	e.t.Helper()
	rows, err := e.pool.Query(context.Background(), t62ListAuditLogs, actorID, action, entityType, nil, nil, nil, 100, 0)
	if err != nil {
		e.t.Fatalf("ListAuditLogs: %v", err)
	}
	defer rows.Close()
	var out []t62AuditRow
	for rows.Next() {
		var r t62AuditRow
		var after []byte
		if err := rows.Scan(&r.ID, &r.Action, &r.EntityType, &r.EntityID, &after, &r.IP); err != nil {
			e.t.Fatalf("scan audit: %v", err)
		}
		if after != nil {
			if err := json.Unmarshal(after, &r.After); err != nil {
				e.t.Fatalf("audit after json: %v", err)
			}
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		e.t.Fatal(err)
	}
	return out
}

// norm round-trips v through JSON so numbers and nulls compare like the stored jsonb.
func norm(t *testing.T, v map[string]any) map[string]any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// Property 6: the audit rows of a report job keep the legacy shape, and the Audit Log query finds
// legacy (backend/) and new (backend-exq) report_exported rows together.
func TestReportExportAuditShapeIntegration(t *testing.T) {
	e := newT62Env(t)
	u := e.users["ADMIN"]
	vid := e.vendorID
	base := "/api/v1/exports/reports/"
	cases := []struct {
		kind       exportjob.Kind
		query      string
		entityType string
		report     string
		filter     map[string]any
	}{
		{exportjob.KindDsrLateDaily, fmt.Sprintf("dsr-late-daily?month=2026-06&vendor_id=%d&status=late", vid), "dsr_late_report", "daily",
			map[string]any{"month": "2026-06", "vendor_id": vid, "vendor_branch_id": nil, "status": "late"}},
		{exportjob.KindDsrLateSummary, "dsr-late-summary?month=2026-06", "dsr_late_report", "summary",
			map[string]any{"month": "2026-06", "vendor_id": nil, "vendor_branch_id": nil}},
		{exportjob.KindReplenishmentAmountMismatch, "replenishment-amount-mismatch?date_from=2026-06-01&date_to=2026-06-30&classification=late",
			"replenishment_report", "amount_mismatch",
			map[string]any{"date_from": "2026-06-01", "date_to": "2026-06-30", "vendor_id": nil, "region_code": nil, "classification": "late"}},
		{exportjob.KindReplenishmentTrips, fmt.Sprintf("replenishment-trips?date_from=2026-06-01&date_to=2026-06-30&vendor_id=%d", vid),
			"replenishment_report", "trips",
			map[string]any{"date_from": "2026-06-01", "date_to": "2026-06-30", "vendor_id": vid, "region_code": nil}},
	}

	for _, c := range cases {
		t.Run(string(c.kind), func(t *testing.T) {
			id := e.createJob(u, base+c.query)
			wantAfter := map[string]any{
				"export_job_id": id, "export_kind": string(c.kind), "report": c.report, "filter": c.filter,
				"username": u.name, "role": "ADMIN",
			}
			e.checkAudit(t, u, exportjob.ActionExportRequested, c.entityType, norm(t, wantAfter))

			e.makeReady(id, "x", 7)
			rec := e.do(http.MethodGet, fmt.Sprintf("/api/v1/exports/jobs/%d/file", id), u.token)
			if rec.Code != http.StatusOK {
				t.Fatalf("download = %d: %s", rec.Code, rec.Body.String())
			}
			wantAfter["row_count"] = 7
			e.checkAudit(t, u, exportjob.ActionReportExported, c.entityType, norm(t, wantAfter))
		})
	}

	// A row exactly as backend/ RecordExport wrote it (actor, report_exported, entity_id 0, {report, filter}).
	legacyAfter := `{"report":"daily","filter":{"month":"2026-05","vendor_id":null,"vendor_branch_id":null}}`
	e.exec(`INSERT INTO audit_logs (actor_id, action, entity_type, entity_id, after, ip)
		VALUES ($1, 'report_exported', 'dsr_late_report', 0, $2::jsonb, '10.62.0.9')`, u.id, legacyAfter)

	all := e.listAuditLogs(u.id, "report_exported", nil)
	if len(all) != 5 { // 4 new + 1 legacy
		t.Fatalf("ListAuditLogs(report_exported) = %d rows, want 5 (4 new + 1 legacy): %+v", len(all), all)
	}
	dsr := "dsr_late_report"
	rows := e.listAuditLogs(u.id, "report_exported", &dsr)
	if len(rows) != 3 { // legacy + daily + summary
		t.Fatalf("ListAuditLogs(report_exported, dsr_late_report) = %d rows, want 3: %+v", len(rows), rows)
	}
	var sawLegacy, sawNew bool
	for _, r := range rows {
		if r.EntityID != 0 {
			t.Errorf("row %d entity_id = %d, want 0", r.ID, r.EntityID)
		}
		if _, ok := r.After["report"]; !ok {
			t.Errorf("row %d after has no report key: %v", r.ID, r.After)
		}
		if _, ok := r.After["filter"]; !ok {
			t.Errorf("row %d after has no filter key: %v", r.ID, r.After)
		}
		if _, ok := r.After["export_job_id"]; ok {
			sawNew = true
		} else if r.IP != nil && *r.IP == "10.62.0.9" {
			sawLegacy = true
		}
	}
	if !sawLegacy || !sawNew {
		t.Errorf("Audit Log query should return both legacy and new rows: legacy=%v new=%v", sawLegacy, sawNew)
	}
}

// checkAudit finds the single audit row of (user, action) and compares its columns and `after`.
func (e *t62Env) checkAudit(t *testing.T, u t62User, action, entityType string, wantAfter map[string]any) {
	t.Helper()
	var matches []t62AuditRow
	for _, r := range e.listAuditLogs(u.id, action, &entityType) {
		if jid, ok := r.After["export_job_id"].(float64); ok && int64(jid) == int64(wantAfter["export_job_id"].(float64)) {
			matches = append(matches, r)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("%s rows for job %v = %d, want 1", action, wantAfter["export_job_id"], len(matches))
	}
	r := matches[0]
	if r.EntityID != 0 {
		t.Errorf("%s entity_id = %d, want 0", action, r.EntityID)
	}
	if r.IP == nil || *r.IP != t62IP {
		t.Errorf("%s ip = %v, want %s", action, r.IP, t62IP)
	}
	if !reflect.DeepEqual(r.After, wantAfter) {
		t.Errorf("%s after =\n%v\nwant\n%v", action, r.After, wantAfter)
	}
}
