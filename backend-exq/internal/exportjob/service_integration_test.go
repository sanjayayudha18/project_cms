//go:build integration

package exportjob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend-exq/internal/audit"
	"github.com/cimb-niaga/cms/backend-exq/internal/repository"
)

// task 11.3 (Properties 13, 14, 16, Req 7.5, 7.6, 8.x, 9.4, 9.5) on real Postgres.
// Everything inserted carries the unique tag s11<hex> (users, vendor, branch code) and is
// removed in t.Cleanup, even on failure. No Worker is started: jobs stay queued unless a
// test sets them to succeeded directly in SQL.

const s11IP = "10.11.12.13"

type s11Env struct {
	t        *testing.T
	pool     *pgxpool.Pool
	svc      *Service
	tag      string
	vendorID int64
	branchID int64
	// a second vendor + branch, to prove cross-vendor branch lookups are 404
	vendor2ID, branch2ID int64
}

type s11FailAudit struct{}

func (s11FailAudit) Write(context.Context, repository.DBTX, audit.Entry) error {
	return errors.New("s11: injected audit failure")
}

func s11NewEnv(t *testing.T) *s11Env {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set -- skipping integration test")
	}
	url = strings.Replace(url, "host.docker.internal", "localhost", 1)
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatalf("parse DATABASE_URL: %v", err)
	}
	cfg.MaxConns = 24
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping db: %v", err)
	}
	e := &s11Env{t: t, pool: pool, tag: fmt.Sprintf("s11%x", time.Now().UnixNano()%0xffffffffff)}
	e.svc = NewService(pool, audit.NewWriter(), nil)

	// Registered before anything is inserted; LIFO order puts pool.Close last.
	t.Cleanup(pool.Close)
	t.Cleanup(e.cleanup)

	e.vendorID, e.branchID = e.seedVendor("a")
	e.vendor2ID, e.branch2ID = e.seedVendor("b")
	return e
}

// cleanup deletes in reverse FK order, by tag, so a half-built fixture is removed too.
func (e *s11Env) cleanup() {
	ctx := context.Background()
	like := e.tag + "-%"
	steps := []struct{ name, sql string }{
		{"audit_logs", `DELETE FROM audit_logs WHERE actor_id IN (SELECT id FROM users WHERE username LIKE $1)`},
		{"export_job_chunks", `DELETE FROM export_job_chunks WHERE export_job_id IN (SELECT id FROM export_jobs WHERE requested_by IN (SELECT id FROM users WHERE username LIKE $1))`},
		{"export_jobs", `DELETE FROM export_jobs WHERE requested_by IN (SELECT id FROM users WHERE username LIKE $1)`},
		{"users", `DELETE FROM users WHERE username LIKE $1`},
		{"vendor_branches", `DELETE FROM vendor_branches WHERE vendor_id IN (SELECT id FROM vendors WHERE code LIKE $1)`},
		{"vendors", `DELETE FROM vendors WHERE code LIKE $1`},
	}
	for _, s := range steps {
		arg := like
		if s.name == "vendors" || s.name == "vendor_branches" {
			arg = strings.ToUpper(e.tag) + "%"
		}
		if _, err := e.pool.Exec(ctx, s.sql, arg); err != nil {
			e.t.Errorf("s11 cleanup %s: %v", s.name, err)
		}
	}
}

func (e *s11Env) seedVendor(suffix string) (vendorID, branchID int64) {
	e.t.Helper()
	ctx := context.Background()
	code := strings.ToUpper(e.tag) + strings.ToUpper(suffix)
	if err := e.pool.QueryRow(ctx,
		`INSERT INTO vendors (code, name) VALUES ($1, $1) RETURNING id`, code).Scan(&vendorID); err != nil {
		e.t.Fatalf("seed vendor: %v", err)
	}
	if err := e.pool.QueryRow(ctx,
		`INSERT INTO vendor_branches (vendor_id, branch_code, branch_name) VALUES ($1, $2, $2) RETURNING id`,
		vendorID, code+"BR").Scan(&branchID); err != nil {
		e.t.Fatalf("seed branch: %v", err)
	}
	return vendorID, branchID
}

func (e *s11Env) user(name, role string) Actor {
	e.t.Helper()
	username := e.tag + "-" + name
	var id int64
	if err := e.pool.QueryRow(context.Background(),
		`INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source)
		 SELECT id, $1, $1, $1 || '@test.local', true, 'ldap' FROM roles WHERE role = $2 RETURNING id`,
		username, role).Scan(&id); err != nil {
		e.t.Fatalf("seed user %s/%s: %v", name, role, err)
	}
	return Actor{UserID: id, Username: username, Role: role, IP: s11IP}
}

func (e *s11Env) n(sql string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		e.t.Fatalf("count: %v", err)
	}
	return n
}

func (e *s11Env) auditCount(actor int64, action string) int {
	e.t.Helper()
	return e.n(`SELECT count(*) FROM audit_logs WHERE actor_id = $1 AND action = $2`, actor, action)
}

func (e *s11Env) jobCount(actor int64) int {
	e.t.Helper()
	return e.n(`SELECT count(*) FROM export_jobs WHERE requested_by = $1`, actor)
}

func (e *s11Env) activeCount(actor int64) int {
	e.t.Helper()
	return e.n(`SELECT count(*) FROM export_jobs WHERE requested_by = $1 AND status IN ('queued','running')`, actor)
}

func (e *s11Env) status(id int64) string {
	e.t.Helper()
	var s string
	if err := e.pool.QueryRow(context.Background(), `SELECT status FROM export_jobs WHERE id = $1`, id).Scan(&s); err != nil {
		e.t.Fatalf("status: %v", err)
	}
	return s
}

// setStatus moves a job straight to a ready-shaped status (satisfies export_jobs_ready_chk);
// succeeded also gets one chunk so the download path has bytes to guard.
func (e *s11Env) setStatus(id int64, status string) {
	e.t.Helper()
	ctx := context.Background()
	if _, err := e.pool.Exec(ctx,
		`UPDATE export_jobs SET status = $2, row_count = 3, byte_size = 5, chunk_count = 1,
		        finished_at = now(), expires_at = now() + interval '10 minutes' WHERE id = $1`, id, status); err != nil {
		e.t.Fatalf("set status: %v", err)
	}
	if status == "succeeded" {
		if _, err := e.pool.Exec(ctx,
			`INSERT INTO export_job_chunks (export_job_id, seq, data) VALUES ($1, 0, $2) ON CONFLICT DO NOTHING`, id, []byte("a,b\n1\n")); err != nil {
			e.t.Fatalf("insert chunk: %v", err)
		}
	}
}

func (e *s11Env) params(kind Kind, entity string, vendor, branch *int64, status, level string) Params {
	e.t.Helper()
	p, err := ParseParams(kind, entity, vendor, branch, status, level)
	if err != nil {
		e.t.Fatalf("ParseParams: %v", err)
	}
	return p
}

func s11Ptr(v int64) *int64 { return &v }

// createVendor creates a vendor-kind job for entity/status.
func (e *s11Env) createVendor(a Actor, entity, status string) (Job, bool, error) {
	return e.svc.Create(context.Background(), a, e.params(KindVendor, entity, s11Ptr(e.vendorID), nil, status, ""))
}

type s11AuditRow struct {
	Action, EntityType string
	EntityID           int64
	IP                 *string
	After              map[string]any
}

// viewerRows reads audit rows with the ListAuditLogs filter shape
// (backend/queries/audit_log.sql: actor_id + action, newest first), read-only.
func (e *s11Env) viewerRows(actor int64, action string) []s11AuditRow {
	e.t.Helper()
	rows, err := e.pool.Query(context.Background(),
		`SELECT id, actor_id, action, entity_type, entity_id, before, after, ip, created_at
		 FROM audit_logs
		 WHERE ($1::bigint IS NULL OR actor_id = $1) AND ($2::text IS NULL OR action = $2)
		 ORDER BY created_at DESC, id DESC LIMIT 100 OFFSET 0`, actor, action)
	if err != nil {
		e.t.Fatalf("viewer query: %v", err)
	}
	defer rows.Close()
	var out []s11AuditRow
	for rows.Next() {
		var (
			id, actorID int64
			before      []byte
			after       []byte
			ts          time.Time
			r           s11AuditRow
		)
		if err := rows.Scan(&id, &actorID, &r.Action, &r.EntityType, &r.EntityID, &before, &after, &r.IP, &ts); err != nil {
			e.t.Fatalf("viewer scan: %v", err)
		}
		if actorID != actor {
			e.t.Fatalf("viewer returned actor %d, want %d", actorID, actor)
		}
		if before != nil {
			e.t.Errorf("audit row %d: before must be NULL", id)
		}
		if err := json.Unmarshal(after, &r.After); err != nil {
			e.t.Fatalf("after json: %v", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		e.t.Fatal(err)
	}
	return out
}

// Property 13: a job is visible only to its owner (Req 7.5).
func TestS11Property13Ownership(t *testing.T) {
	e := s11NewEnv(t)
	ctx := context.Background()
	a := e.user("own-a", "ADMIN")
	b := e.user("own-b", "ADMIN_PARAM")

	job, created, err := e.createVendor(a, "vendor", "")
	if err != nil || !created {
		t.Fatalf("A create: created=%v err=%v", created, err)
	}
	e.setStatus(job.ID, "succeeded")

	if _, err := e.svc.Get(ctx, b, job.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("B Get: want ErrNotFound, got %v", err)
	}
	if _, err := e.svc.StartDownload(ctx, b, job.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("B StartDownload: want ErrNotFound, got %v", err)
	}
	jobs, err := e.svc.List(ctx, b, 50)
	if err != nil {
		t.Fatal(err)
	}
	for _, j := range jobs {
		if j.ID == job.ID || j.RequestedBy != b.UserID {
			t.Errorf("List(B) leaked job %d (owner %d)", j.ID, j.RequestedBy)
		}
	}
	if got := e.status(job.ID); got != "succeeded" {
		t.Errorf("job status after B attempts = %s, want succeeded", got)
	}
	if n := e.auditCount(b.UserID, ActionMasterDataExported); n != 0 {
		t.Errorf("B has %d master_data_exported rows, want 0", n)
	}
	if n := e.auditCount(a.UserID, ActionMasterDataExported); n != 0 {
		t.Errorf("A has %d master_data_exported rows after only B's attempts, want 0", n)
	}
	// The owner still sees and downloads it.
	if _, err := e.svc.Get(ctx, a, job.ID); err != nil {
		t.Errorf("A Get: %v", err)
	}
	if _, err := e.svc.StartDownload(ctx, a, job.ID); err != nil {
		t.Errorf("A StartDownload: %v", err)
	}
}

// Property 14: at most MaxActivePerUser active jobs, identical params dedupe to one job,
// under concurrency, with the unique index never surfacing as ErrInternal (Req 9.4, 9.5).
func TestS11Property14LimitsAndDedupe(t *testing.T) {
	e := s11NewEnv(t)
	ctx := context.Background()
	u := e.user("lim", "ADMIN")

	// 10 distinct param sets x 3 concurrent requests each.
	type req struct{ entity, status string }
	var sets []req
	for _, ent := range []string{"vendor", "branches", "vaults", "pics", "dsr-location-maps"} {
		sets = append(sets, req{ent, "active"}, req{ent, "all"})
	}
	const dup = 3
	type res struct {
		set     int
		job     Job
		created bool
		err     error
	}
	results := make([]res, 0, len(sets)*dup)
	var mu sync.Mutex
	var wg sync.WaitGroup
	start := make(chan struct{})
	stopSample := make(chan struct{})
	sampleDone := make(chan int)
	go func() { // never more than MaxActivePerUser active at any sampled instant
		maxSeen := 0
		for {
			select {
			case <-stopSample:
				sampleDone <- maxSeen
				return
			default:
				var n int
				if err := e.pool.QueryRow(ctx,
					`SELECT count(*) FROM export_jobs WHERE requested_by = $1 AND status IN ('queued','running')`,
					u.UserID).Scan(&n); err == nil && n > maxSeen {
					maxSeen = n
				}
			}
		}
	}()
	for i, s := range sets {
		for d := 0; d < dup; d++ {
			wg.Add(1)
			go func(i int, s req) {
				defer wg.Done()
				<-start
				j, c, err := e.createVendor(u, s.entity, s.status)
				mu.Lock()
				results = append(results, res{i, j, c, err})
				mu.Unlock()
			}(i, s)
		}
	}
	close(start)
	wg.Wait()
	close(stopSample)
	if maxSeen := <-sampleDone; maxSeen > MaxActivePerUser {
		t.Fatalf("sampled %d active jobs, limit %d", maxSeen, MaxActivePerUser)
	}

	idBySet := map[int]int64{}
	created := map[int64]int{}
	var rejected int
	for _, r := range results {
		switch {
		case r.err == nil:
			if prev, ok := idBySet[r.set]; ok && prev != r.job.ID {
				t.Errorf("set %d got two ids %d and %d", r.set, prev, r.job.ID)
			}
			idBySet[r.set] = r.job.ID
			if r.created {
				created[r.job.ID]++
			}
		case errors.Is(r.err, ErrTooManyActive):
			rejected++
		default:
			t.Errorf("unexpected error (internal/500?): %v", r.err)
		}
	}
	if got := e.activeCount(u.UserID); got != MaxActivePerUser {
		t.Errorf("active jobs = %d, want exactly %d", got, MaxActivePerUser)
	}
	if got := e.jobCount(u.UserID); got != MaxActivePerUser {
		t.Errorf("total jobs = %d, want %d (429 must create nothing)", got, MaxActivePerUser)
	}
	if len(created) != MaxActivePerUser {
		t.Errorf("created ids = %d, want %d", len(created), MaxActivePerUser)
	}
	for id, c := range created {
		if c != 1 {
			t.Errorf("job %d reported created %d times", id, c)
		}
	}
	if rejected == 0 {
		t.Error("no request was rejected with ErrTooManyActive")
	}
	if got := e.auditCount(u.UserID, ActionExportRequested); got != MaxActivePerUser {
		t.Errorf("export_requested rows = %d, want %d", got, MaxActivePerUser)
	}

	// At the limit: identical params still dedupe (200 path), a new one is 429, nothing changes.
	var someID int64
	var someSet req
	for i, id := range idBySet {
		someID, someSet = id, sets[i]
		break
	}
	j, c, err := e.createVendor(u, someSet.entity, someSet.status)
	if err != nil || c || j.ID != someID {
		t.Errorf("dedupe at limit: id=%d created=%v err=%v, want id=%d created=false", j.ID, c, err, someID)
	}
	var fresh req
	for i, s := range sets {
		if _, ok := idBySet[i]; !ok {
			fresh = s
			break
		}
	}
	if _, _, err := e.createVendor(u, fresh.entity, fresh.status); !errors.Is(err, ErrTooManyActive) {
		t.Errorf("4th distinct: want ErrTooManyActive, got %v", err)
	}
	if e.jobCount(u.UserID) != MaxActivePerUser || e.auditCount(u.UserID, ActionExportRequested) != MaxActivePerUser {
		t.Error("dedupe / 429 changed jobs or audit")
	}
}

// Property 16: audit completeness and content (Req 8.1-8.6, 8.9).
func TestS11Property16AuditCompleteness(t *testing.T) {
	e := s11NewEnv(t)
	ctx := context.Background()
	a := e.user("aud", "ADMIN")

	vid, bid := s11Ptr(e.vendorID), s11Ptr(e.branchID)
	global := entitiesFor(KindMasterDataGlobal)[0]
	cases := []struct {
		name       string
		p          Params
		wantType   string
		wantEntity int64
	}{
		{"vendor", e.params(KindVendor, "vendor", vid, nil, "", ""), "vendor", e.vendorID},
		{"cabang", e.params(KindCabang, "vaults", vid, bid, "", ""), "vendor_branch", e.branchID},
		{"harga_paket branch", e.params(KindHargaPaket, "package-prices", vid, bid, "", ""), "vendor_branch", e.branchID},
		{"harga_paket vendor", e.params(KindHargaPaket, "package-prices", vid, nil, "", ""), "vendor", e.vendorID},
		{"master_data_global", e.params(KindMasterDataGlobal, global, nil, nil, "", ""), "master_data", 0},
	}
	ids := map[int64]int{} // job id -> case index
	for i, c := range cases {
		job, created, err := e.svc.Create(ctx, a, c.p)
		if err != nil || !created {
			t.Fatalf("%s create: created=%v err=%v", c.name, created, err)
		}
		ids[job.ID] = i
		e.setStatus(job.ID, "succeeded") // frees an active slot (limit 3)
	}
	if got := e.auditCount(a.UserID, ActionExportRequested); got != len(cases) {
		t.Fatalf("export_requested = %d, want %d (== new jobs)", got, len(cases))
	}
	if got := e.auditCount(a.UserID, ActionMasterDataExported); got != 0 {
		t.Fatalf("master_data_exported before any download = %d, want 0", got)
	}
	for id := range ids {
		if _, err := e.svc.StartDownload(ctx, a, id); err != nil {
			t.Fatalf("StartDownload %d: %v", id, err)
		}
	}
	// A second claim of the same job is rejected (409) and adds nothing.
	for id := range ids {
		if _, err := e.svc.StartDownload(ctx, a, id); !errors.Is(err, ErrNotReady) {
			t.Errorf("second StartDownload %d: want ErrNotReady, got %v", id, err)
		}
	}
	if got := e.auditCount(a.UserID, ActionMasterDataExported); got != len(cases) {
		t.Fatalf("master_data_exported = %d, want %d (== successful StartDownload)", got, len(cases))
	}

	for _, action := range []string{ActionExportRequested, ActionMasterDataExported} {
		rows := e.viewerRows(a.UserID, action)
		if len(rows) != len(cases) {
			t.Fatalf("viewer %s rows = %d, want %d", action, len(rows), len(cases))
		}
		seen := map[int64]bool{}
		for _, r := range rows {
			jid := int64(r.After["export_job_id"].(float64))
			ci, ok := ids[jid]
			if !ok {
				t.Errorf("%s: unexpected export_job_id %d", action, jid)
				continue
			}
			seen[jid] = true
			c := cases[ci]
			if r.EntityType != c.wantType || r.EntityID != c.wantEntity {
				t.Errorf("%s/%s: entity %s/%d, want %s/%d", action, c.name, r.EntityType, r.EntityID, c.wantType, c.wantEntity)
			}
			if r.IP == nil || *r.IP != s11IP {
				t.Errorf("%s/%s: ip = %v, want %s", action, c.name, r.IP, s11IP)
			}
			if r.After["username"] != a.Username || r.After["role"] != "ADMIN" {
				t.Errorf("%s/%s: username/role = %v/%v", action, c.name, r.After["username"], r.After["role"])
			}
			if r.After["export_kind"] != string(c.p.Kind) || r.After["export_entity"] != c.p.Entity {
				t.Errorf("%s/%s: kind/entity = %v/%v", action, c.name, r.After["export_kind"], r.After["export_entity"])
			}
			_, hasRows := r.After["row_count"]
			if (action == ActionMasterDataExported) != hasRows {
				t.Errorf("%s/%s: row_count presence = %v", action, c.name, hasRows)
			}
		}
		if len(seen) != len(cases) {
			t.Errorf("%s: covered %d distinct jobs, want %d", action, len(seen), len(cases))
		}
	}
}

// Rejected requests (401/403/404/409/410/429) write no audit and create no job.
func TestS11Property16RejectionsWriteNothing(t *testing.T) {
	e := s11NewEnv(t)
	ctx := context.Background()
	u := e.user("rej", "ADMIN")
	plain := e.user("rej-plain", "ATM-USER")

	var first Job
	for i, ent := range []string{"vendor", "branches", "vaults"} {
		j, _, err := e.createVendor(u, ent, "")
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = j
		}
	}
	reqBefore := e.auditCount(u.UserID, ActionExportRequested)
	jobsBefore := e.jobCount(u.UserID)
	if reqBefore != 3 || jobsBefore != 3 {
		t.Fatalf("setup: audit=%d jobs=%d, want 3/3", reqBefore, jobsBefore)
	}
	plainBefore := e.n(`SELECT count(*) FROM audit_logs WHERE actor_id = $1`, plain.UserID)

	vp := e.params(KindVendor, "vendor", s11Ptr(e.vendorID), nil, "", "")
	unknownVendor := e.params(KindVendor, "vendor", s11Ptr(e.vendorID+1_000_000_000), nil, "", "")
	crossBranch := e.params(KindCabang, "vaults", s11Ptr(e.vendorID), s11Ptr(e.branch2ID), "", "")
	fresh := e.params(KindVendor, "pics", s11Ptr(e.vendorID), nil, "", "")

	checks := []struct {
		name string
		want error
		run  func() error
	}{
		{"401 create", ErrUnauthenticated, func() error { _, _, err := e.svc.Create(ctx, Actor{}, vp); return err }},
		{"401 download", ErrUnauthenticated, func() error { _, err := e.svc.StartDownload(ctx, Actor{}, first.ID); return err }},
		{"403 create", ErrForbidden, func() error { _, _, err := e.svc.Create(ctx, plain, vp); return err }},
		{"404 vendor", ErrNotFound, func() error { _, _, err := e.svc.Create(ctx, u, unknownVendor); return err }},
		{"404 foreign branch", ErrNotFound, func() error { _, _, err := e.svc.Create(ctx, u, crossBranch); return err }},
		{"404 job", ErrNotFound, func() error { _, err := e.svc.StartDownload(ctx, u, 999_999_999_999); return err }},
		{"429 create", ErrTooManyActive, func() error { _, _, err := e.svc.Create(ctx, u, fresh); return err }},
		{"409 download queued", ErrNotReady, func() error { _, err := e.svc.StartDownload(ctx, u, first.ID); return err }},
	}
	run := func() {
		for _, c := range checks {
			if err := c.run(); !errors.Is(err, c.want) {
				t.Errorf("%s: want %v, got %v", c.name, c.want, err)
			}
		}
	}
	run()
	e.setStatus(first.ID, "downloaded")
	if _, err := e.svc.StartDownload(ctx, u, first.ID); !errors.Is(err, ErrGone) {
		t.Errorf("410 download: want ErrGone, got %v", err)
	}
	// 403 on download for a plain user is covered in the role-revocation test; here 403 create only.

	if got := e.auditCount(u.UserID, ActionExportRequested); got != reqBefore {
		t.Errorf("export_requested = %d after rejections, want %d", got, reqBefore)
	}
	if got := e.auditCount(u.UserID, ActionMasterDataExported); got != 0 {
		t.Errorf("master_data_exported = %d after rejections, want 0", got)
	}
	if got := e.jobCount(u.UserID); got != jobsBefore {
		t.Errorf("jobs = %d after rejections, want %d", got, jobsBefore)
	}
	if got := e.jobCount(plain.UserID); got != 0 {
		t.Errorf("forbidden user got %d jobs", got)
	}
	if got := e.n(`SELECT count(*) FROM audit_logs WHERE actor_id = $1`, plain.UserID); got != plainBefore {
		t.Errorf("forbidden user audit rows changed %d -> %d", plainBefore, got)
	}
}

// Audit failure leaves no job and releases no bytes (Req 8.1, 8.2).
func TestS11Property16AuditFailure(t *testing.T) {
	e := s11NewEnv(t)
	ctx := context.Background()
	u := e.user("fail", "ADMIN")
	bad := *e.svc
	bad.audit = s11FailAudit{}

	// Create: audit fails -> no job row, no audit row.
	_, created, err := bad.Create(ctx, u, e.params(KindVendor, "vendor", s11Ptr(e.vendorID), nil, "", ""))
	if !errors.Is(err, ErrInternal) || created {
		t.Errorf("Create with failing audit: created=%v err=%v, want ErrInternal", created, err)
	}
	if n := e.jobCount(u.UserID); n != 0 {
		t.Errorf("jobs after failed-audit Create = %d, want 0", n)
	}
	if n := e.n(`SELECT count(*) FROM audit_logs WHERE actor_id = $1`, u.UserID); n != 0 {
		t.Errorf("audit rows = %d, want 0", n)
	}
	// The same request succeeds with the real writer afterwards (nothing stuck behind the lock/index).
	job, created, err := e.createVendor(u, "vendor", "")
	if err != nil || !created {
		t.Fatalf("retry create: created=%v err=%v", created, err)
	}

	// StartDownload: audit fails -> claim rolled back, no Job returned (so no chunk is ever opened).
	e.setStatus(job.ID, "succeeded")
	got, err := bad.StartDownload(ctx, u, job.ID)
	if !errors.Is(err, ErrInternal) || got.ID != 0 {
		t.Errorf("StartDownload with failing audit: job=%d err=%v, want zero Job + ErrInternal", got.ID, err)
	}
	if s := e.status(job.ID); s != "succeeded" {
		t.Errorf("status after failed-audit download = %s, want succeeded (claim rolled back)", s)
	}
	if n := e.auditCount(u.UserID, ActionMasterDataExported); n != 0 {
		t.Errorf("master_data_exported = %d, want 0", n)
	}
	if n := e.n(`SELECT count(*) FROM export_job_chunks WHERE export_job_id = $1`, job.ID); n != 1 {
		t.Errorf("chunks = %d, want 1 (untouched)", n)
	}
	// And the real writer still lets the owner download once.
	if _, err := e.svc.StartDownload(ctx, u, job.ID); err != nil {
		t.Errorf("StartDownload after rollback: %v", err)
	}
	if n := e.auditCount(u.UserID, ActionMasterDataExported); n != 1 {
		t.Errorf("master_data_exported = %d, want 1", n)
	}
}

// Role revoked (or user deactivated) after Create -> StartDownload 403 (Req 7.6); List/Get still work (D23).
func TestS11RoleRevokedAfterCreate(t *testing.T) {
	e := s11NewEnv(t)
	ctx := context.Background()
	u := e.user("revoke", "ADMIN")
	job, _, err := e.createVendor(u, "vendor", "")
	if err != nil {
		t.Fatal(err)
	}
	e.setStatus(job.ID, "succeeded")

	if _, err := e.pool.Exec(ctx,
		`UPDATE users SET role_id = (SELECT id FROM roles WHERE role = 'ATM-USER') WHERE id = $1`, u.UserID); err != nil {
		t.Fatal(err)
	}
	// The Actor still carries the stale ADMIN role: the service must re-read it from the DB.
	if _, err := e.svc.StartDownload(ctx, u, job.ID); !errors.Is(err, ErrForbidden) {
		t.Errorf("role revoked: want ErrForbidden, got %v", err)
	}
	if s := e.status(job.ID); s != "succeeded" {
		t.Errorf("status after 403 = %s, want succeeded", s)
	}
	if _, err := e.svc.Get(ctx, u, job.ID); err != nil {
		t.Errorf("Get after revoke: %v", err)
	}
	if jobs, err := e.svc.List(ctx, u, 10); err != nil || len(jobs) != 1 {
		t.Errorf("List after revoke: %d jobs, err=%v", len(jobs), err)
	}

	// Role restored but account deactivated: still 403.
	if _, err := e.pool.Exec(ctx,
		`UPDATE users SET role_id = (SELECT id FROM roles WHERE role = 'ADMIN'), is_active = false WHERE id = $1`, u.UserID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.StartDownload(ctx, u, job.ID); !errors.Is(err, ErrForbidden) {
		t.Errorf("deactivated: want ErrForbidden, got %v", err)
	}
	if n := e.auditCount(u.UserID, ActionMasterDataExported); n != 0 {
		t.Errorf("master_data_exported = %d after 403s, want 0", n)
	}
}
