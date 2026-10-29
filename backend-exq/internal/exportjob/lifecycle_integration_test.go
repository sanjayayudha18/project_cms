//go:build integration

package exportjob

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cimb-niaga/cms/backend-exq/internal/audit"
	"github.com/cimb-niaga/cms/backend-exq/internal/repository"
)

// task 11.2 (Properties 10, 11, 12, 15) on real Postgres, with the real Worker, Janitor and Service.
// Everything inserted carries the unique tag w11<hex> (users, vendor, branch code) and is removed in
// t.Cleanup, even on failure. Time is never waited for: started_at / download_started_at / expires_at
// are moved into the past with SQL. Tests run sequentially; a Worker claims ANY queued job, so nothing
// else may use export_jobs on this DB while they run, and no test leaves a queued job behind.

func w11Rng(t *testing.T) *rand.Rand {
	t.Helper()
	seed := uint64(time.Now().UnixNano())
	if v := os.Getenv("W11_SEED"); v != "" {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			t.Fatalf("W11_SEED: %v", err)
		}
		seed = n
	}
	t.Logf("seed=%d (rerun with W11_SEED=%d)", seed, seed)
	return rand.New(rand.NewPCG(seed, 0x5711))
}

type w11Env struct {
	t        *testing.T
	pool     *pgxpool.Pool
	svc      *Service
	tag      string
	vendorID int64
	users    []Actor
}

func w11NewEnv(t *testing.T, nUsers int) *w11Env {
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
	cfg.MaxConns = 30
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping db: %v", err)
	}
	e := &w11Env{t: t, pool: pool, tag: fmt.Sprintf("w11%x", time.Now().UnixNano()%0xffffffffff)}
	e.svc = NewService(pool, audit.NewWriter(), nil)
	t.Cleanup(pool.Close) // LIFO: runs after e.cleanup
	t.Cleanup(e.cleanup)

	code := strings.ToUpper(e.tag)
	if err := pool.QueryRow(ctx, `INSERT INTO vendors (code, name) VALUES ($1, $1) RETURNING id`, code).Scan(&e.vendorID); err != nil {
		t.Fatalf("seed vendor: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO vendor_branches (vendor_id, branch_code, branch_name) VALUES ($1, $2, $2)`, e.vendorID, code+"BR"); err != nil {
		t.Fatalf("seed branch: %v", err)
	}
	for i := range nUsers {
		e.users = append(e.users, e.user(fmt.Sprintf("u%d", i)))
	}
	return e
}

// cleanup deletes in reverse FK order, by tag, so a half-built fixture is removed too.
func (e *w11Env) cleanup() {
	ctx := context.Background()
	like := e.tag + "-%"
	up := strings.ToUpper(e.tag) + "%"
	steps := []struct{ name, sql, arg string }{
		{"audit_logs", `DELETE FROM audit_logs WHERE actor_id IN (SELECT id FROM users WHERE username LIKE $1)`, like},
		{"export_job_chunks", `DELETE FROM export_job_chunks WHERE export_job_id IN (SELECT id FROM export_jobs WHERE requested_by IN (SELECT id FROM users WHERE username LIKE $1))`, like},
		{"export_jobs", `DELETE FROM export_jobs WHERE requested_by IN (SELECT id FROM users WHERE username LIKE $1)`, like},
		{"users", `DELETE FROM users WHERE username LIKE $1`, like},
		{"vendor_branches", `DELETE FROM vendor_branches WHERE vendor_id IN (SELECT id FROM vendors WHERE code LIKE $1)`, up},
		{"vendors", `DELETE FROM vendors WHERE code LIKE $1`, up},
	}
	for _, s := range steps {
		if _, err := e.pool.Exec(ctx, s.sql, s.arg); err != nil {
			e.t.Errorf("w11 cleanup %s: %v", s.name, err)
		}
	}
}

func (e *w11Env) user(name string) Actor {
	e.t.Helper()
	username := e.tag + "-" + name
	var id int64
	if err := e.pool.QueryRow(context.Background(),
		`INSERT INTO users (role_id, username, full_name, email, is_karyawan, auth_source)
		 SELECT id, $1, $1, $1 || '@test.local', true, 'ldap' FROM roles WHERE role = 'ADMIN' RETURNING id`,
		username).Scan(&id); err != nil {
		e.t.Fatalf("seed user %s: %v", name, err)
	}
	return Actor{UserID: id, Username: username, Role: "ADMIN", IP: "10.11.12.14"}
}

func (e *w11Env) actorByID(id int64) Actor {
	for _, u := range e.users {
		if u.UserID == id {
			return u
		}
	}
	e.t.Fatalf("unknown user %d", id)
	return Actor{}
}

func (e *w11Env) userIDs() []int64 {
	ids := make([]int64, len(e.users))
	for i, u := range e.users {
		ids[i] = u.UserID
	}
	return ids
}

func (e *w11Env) count(sql string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		e.t.Fatalf("count: %v", err)
	}
	return n
}

func (e *w11Env) exec(sql string, args ...any) {
	e.t.Helper()
	if _, err := e.pool.Exec(context.Background(), sql, args...); err != nil {
		e.t.Fatalf("exec: %v", err)
	}
}

func (e *w11Env) chunks(id int64) int {
	return e.count(`SELECT count(*) FROM export_job_chunks WHERE export_job_id = $1`, id)
}

func (e *w11Env) status(id int64) string {
	e.t.Helper()
	var s string
	if err := e.pool.QueryRow(context.Background(), `SELECT status FROM export_jobs WHERE id = $1`, id).Scan(&s); err != nil {
		e.t.Fatalf("status: %v", err)
	}
	return s
}

func (e *w11Env) fileBytes(id int64) []byte {
	e.t.Helper()
	rows, err := e.pool.Query(context.Background(), `SELECT data FROM export_job_chunks WHERE export_job_id = $1 ORDER BY seq`, id)
	if err != nil {
		e.t.Fatalf("chunks: %v", err)
	}
	defer rows.Close()
	var out []byte
	for rows.Next() {
		var d []byte
		if err := rows.Scan(&d); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, d...)
	}
	if err := rows.Err(); err != nil {
		e.t.Fatal(err)
	}
	return out
}

// ---- fake producer (controlled success / failure / shutdown) -------------------------------

const (
	w11OK int32 = iota
	w11Big
	w11Fail
	w11FailBig
	w11Shutdown
)

var w11Entities = []string{"vendor", "branches", "vaults", "pics", "dsr-location-maps"}

type w11Prod struct {
	header []string
	mode   *atomic.Int32
	cancel func() // set after the worker exists
}

func (p w11Prod) Header() []string { return p.header }

func (p w11Prod) row(i int, big bool) []string {
	cells := make([]string, len(p.header))
	cells[0] = strconv.Itoa(i)
	if big {
		cells[0] = strings.Repeat("x", 1000) + strconv.Itoa(i)
	}
	return cells
}

func (p w11Prod) Produce(ctx context.Context, _ Querier, _ Params, _ time.Time, emit func([]string) error) error {
	m := p.mode.Load()
	big := m == w11Big || m == w11FailBig
	n := 3
	if big {
		n = 2500 // ~2.5 MB -> 3 chunks
	}
	for i := range n {
		if err := emit(p.row(i, big)); err != nil {
			return err
		}
	}
	switch m {
	case w11Fail, w11FailBig:
		return errors.New("w11 injected producer failure")
	case w11Shutdown:
		p.cancel()
		<-ctx.Done()
		return ctx.Err()
	}
	return nil
}

var w11Cfg = WorkerConfig{Workers: 1, PollInterval: 10 * time.Millisecond, JanitorInterval: time.Second}

// w11Worker builds a real Worker (NewWorker, real pools) whose producers are the fake ones.
func w11Worker(pool *pgxpool.Pool, mode *atomic.Int32) *Worker {
	hdr := DefaultProducers()[KindVendor]["vendor"].Header()
	prod := w11Prod{header: hdr, mode: mode}
	producers := map[Kind]map[string]Producer{KindVendor: {}}
	for _, ent := range w11Entities {
		producers[KindVendor][ent] = &prod
	}
	w := NewWorker(pool, pool, producers, w11Cfg)
	prod.cancel = w.cancelJobs
	// producers hold &prod, so the cancel set above is visible to them
	return w
}

func (e *w11Env) create(a Actor, entity, status string) (Job, bool, error) {
	e.t.Helper()
	vid := e.vendorID
	p, err := ParseParams(KindVendor, entity, &vid, nil, status, "")
	if err != nil {
		e.t.Fatalf("ParseParams: %v", err)
	}
	return e.svc.Create(context.Background(), a, p)
}

func (e *w11Env) loadJob(id int64) Job {
	e.t.Helper()
	row, err := repository.New(e.pool).GetExportJobForUser(context.Background(), repository.GetExportJobForUserParams{
		ID: id, RequestedBy: e.ownerOf(id),
	})
	if err != nil {
		e.t.Fatalf("load job %d: %v", id, err)
	}
	return jobFromRow(row)
}

func (e *w11Env) ownerOf(id int64) int64 {
	var u int64
	if err := e.pool.QueryRow(context.Background(), `SELECT requested_by FROM export_jobs WHERE id = $1`, id).Scan(&u); err != nil {
		e.t.Fatalf("owner: %v", err)
	}
	return u
}

func (e *w11Env) tick() {
	NewJanitor(e.pool, time.Minute).tick(context.Background())
}

// ---- Property 10 ----------------------------------------------------------------------------

type w11Snap struct {
	status      string
	chunks      int
	chunkCount  *int32
	requestedBy int64
}

func (e *w11Env) snap() map[int64]w11Snap {
	e.t.Helper()
	rows, err := e.pool.Query(context.Background(),
		`SELECT j.id, j.status, j.chunk_count, j.requested_by,
		        (SELECT count(*) FROM export_job_chunks c WHERE c.export_job_id = j.id)
		 FROM export_jobs j WHERE j.requested_by = ANY($1)`, e.userIDs())
	if err != nil {
		e.t.Fatalf("snap: %v", err)
	}
	defer rows.Close()
	out := map[int64]w11Snap{}
	for rows.Next() {
		var id int64
		var s w11Snap
		if err := rows.Scan(&id, &s.status, &s.chunkCount, &s.requestedBy, &s.chunks); err != nil {
			e.t.Fatal(err)
		}
		out[id] = s
	}
	if err := rows.Err(); err != nil {
		e.t.Fatal(err)
	}
	return out
}

type w11Edge [2]string

func w11Edges(pairs ...w11Edge) map[w11Edge]bool {
	m := map[w11Edge]bool{}
	for _, p := range pairs {
		m[p] = true
	}
	return m
}

// allowed status changes per operation: the "Transisi status dan penjaganya" table, restricted to
// what one operation can do (a janitor tick may recover and expire a download in one pass).
var w11Allowed = map[string]map[w11Edge]bool{
	"create":       w11Edges(),
	"claim":        w11Edges(w11Edge{"queued", "running"}),
	"run_ok":       w11Edges(w11Edge{"running", "succeeded"}),
	"run_fail":     w11Edges(w11Edge{"running", "failed"}),
	"run_shutdown": w11Edges(w11Edge{"running", "queued"}),
	"run_lost": w11Edges(w11Edge{"running", "failed"}, w11Edge{"downloading", "succeeded"},
		w11Edge{"downloading", "expired"}, w11Edge{"succeeded", "expired"}), // includes a full janitor tick
	"start":    w11Edges(w11Edge{"succeeded", "downloading"}),
	"complete": w11Edges(w11Edge{"downloading", "downloaded"}),
	"abort":    w11Edges(w11Edge{"downloading", "succeeded"}),
	"age":      w11Edges(),
	"tick": w11Edges(w11Edge{"running", "failed"}, w11Edge{"downloading", "succeeded"},
		w11Edge{"downloading", "expired"}, w11Edge{"succeeded", "expired"}),
}

func (e *w11Env) checkOp(op string, prev, cur map[int64]w11Snap, trace []string) {
	e.t.Helper()
	fail := func(format string, a ...any) {
		e.t.Helper()
		e.t.Fatalf("op %q: %s\nlast ops: %s", op, fmt.Sprintf(format, a...), strings.Join(trace[max(0, len(trace)-8):], " | "))
	}
	changed := 0
	for id, c := range cur {
		p, existed := prev[id]
		switch {
		case !existed:
			if op != "create" || c.status != "queued" {
				fail("job %d appeared as %s", id, c.status)
			}
		case p.status != c.status:
			changed++
			if !w11Allowed[op][w11Edge{p.status, c.status}] {
				fail("job %d: illegal transition %s -> %s", id, p.status, c.status)
			}
		}
		switch c.status {
		case "failed", "downloaded", "expired", "queued", "running":
			if c.chunks != 0 {
				fail("job %d is %s but has %d chunks", id, c.status, c.chunks)
			}
		case "succeeded", "downloading":
			if c.chunkCount == nil || int(*c.chunkCount) != c.chunks {
				fail("job %d is %s with chunk_count=%v but %d stored chunks", id, c.status, c.chunkCount, c.chunks)
			}
		}
	}
	for id := range prev {
		if _, ok := cur[id]; !ok {
			fail("job %d vanished", id)
		}
	}
	if op != "tick" && op != "run_lost" && changed > 1 {
		fail("%d jobs changed status in one operation", changed)
	}
}

// ids of jobs in the given statuses, sorted (deterministic for a seed).
func w11IDs(s map[int64]w11Snap, statuses ...string) []int64 {
	var out []int64
	for id, v := range s {
		if slices.Contains(statuses, v.status) {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

func w11Pick(rng *rand.Rand, ids []int64) (int64, bool) {
	if len(ids) == 0 {
		return 0, false
	}
	return ids[rng.IntN(len(ids))], true
}

func TestW11Property10Transitions(t *testing.T) {
	e := w11NewEnv(t, 4)
	rng := w11Rng(t)
	ctx := context.Background()
	statuses := []string{"active", "disabled", "all"}
	var trace []string

	for seq := range 6 {
		for step := range 45 {
			prev := e.snap()
			op, detail := "", ""
			var opErr error
			switch k := rng.IntN(12); k {
			case 0, 1:
				op = "create"
				a := e.users[rng.IntN(len(e.users))]
				ent, st := w11Entities[rng.IntN(len(w11Entities))], statuses[rng.IntN(len(statuses))]
				_, _, err := e.create(a, ent, st)
				if err != nil && !errors.Is(err, ErrTooManyActive) {
					t.Fatalf("create: %v", err)
				}
				detail = fmt.Sprintf("%s/%s", ent, st)
			case 2:
				op = "claim"
				w := w11Worker(e.pool, new(atomic.Int32))
				job, ok, err := w.store.ClaimNext(ctx)
				if err != nil {
					t.Fatalf("claim: %v", err)
				}
				detail = fmt.Sprintf("ok=%v id=%d", ok, job.ID)
			case 3, 4, 5, 6:
				op = [...]string{3: "run_ok", 4: "run_fail", 5: "run_shutdown", 6: "run_lost"}[k]
				id, ok := w11Pick(rng, w11IDs(prev, "running"))
				if !ok {
					op, detail = "noop", "no running job"
					break
				}
				job := e.loadJob(id)
				mode := w11OK
				switch op {
				case "run_ok":
					if rng.IntN(3) == 0 {
						mode = w11Big
					}
				case "run_fail":
					mode = w11Fail
					if rng.IntN(2) == 0 {
						mode = w11FailBig
					}
				case "run_shutdown":
					mode = w11Shutdown
				case "run_lost":
					// the janitor fails the job while this (stale) run is in flight
					e.exec(`UPDATE export_jobs SET started_at = now() - interval '11 minutes' WHERE id = $1`, id)
					e.tick()
				}
				m := new(atomic.Int32)
				m.Store(mode)
				w11Worker(e.pool, m).run(job)
				detail = fmt.Sprintf("id=%d mode=%d", id, mode)
			case 7:
				op = "start"
				ids := w11IDs(prev, "succeeded")
				if len(ids) == 0 || rng.IntN(4) == 0 {
					ids = w11IDs(prev, "queued", "running", "succeeded", "failed", "downloading", "downloaded", "expired")
				}
				id, ok := w11Pick(rng, ids)
				if !ok {
					op = "noop"
					break
				}
				_, opErr = e.svc.StartDownload(ctx, e.actorByID(prev[id].requestedBy), id)
				detail = fmt.Sprintf("id=%d was=%s err=%v", id, prev[id].status, opErr)
				cur := e.status(id)
				if opErr == nil && !(prev[id].status == "succeeded" && cur == "downloading") {
					t.Fatalf("StartDownload ok but %s -> %s", prev[id].status, cur)
				}
				if opErr != nil && (!(errors.Is(opErr, ErrNotReady) || errors.Is(opErr, ErrGone)) || cur != prev[id].status) {
					t.Fatalf("StartDownload err=%v status %s -> %s", opErr, prev[id].status, cur)
				}
			case 8, 9:
				op = [...]string{8: "complete", 9: "abort"}[k]
				ids := w11IDs(prev, "downloading")
				if len(ids) == 0 || rng.IntN(4) == 0 {
					ids = w11IDs(prev, "queued", "running", "succeeded", "failed", "downloading", "downloaded", "expired")
				}
				id, ok := w11Pick(rng, ids)
				if !ok {
					op = "noop"
					break
				}
				if op == "complete" {
					opErr = e.svc.CompleteDownload(ctx, id)
				} else {
					opErr = e.svc.AbortDownload(ctx, id)
				}
				detail = fmt.Sprintf("id=%d was=%s err=%v", id, prev[id].status, opErr)
				if (opErr == nil) != (prev[id].status == "downloading") || (opErr != nil && !errors.Is(opErr, ErrNotDownloading)) {
					t.Fatalf("%s on %s job: err=%v", op, prev[id].status, opErr)
				}
			case 10:
				op = "age"
				switch rng.IntN(3) {
				case 0:
					id, _ := w11Pick(rng, w11IDs(prev, "running"))
					e.exec(`UPDATE export_jobs SET started_at = now() - interval '11 minutes' WHERE id = $1 AND status = 'running'`, id)
					detail = fmt.Sprintf("running %d", id)
				case 1:
					id, _ := w11Pick(rng, w11IDs(prev, "downloading"))
					e.exec(`UPDATE export_jobs SET download_started_at = now() - interval '6 minutes' WHERE id = $1 AND status = 'downloading'`, id)
					detail = fmt.Sprintf("downloading %d", id)
				default:
					id, _ := w11Pick(rng, w11IDs(prev, "succeeded", "downloading"))
					e.exec(`UPDATE export_jobs SET expires_at = now() - interval '1 second' WHERE id = $1 AND status IN ('succeeded','downloading')`, id)
					detail = fmt.Sprintf("expire %d", id)
				}
			default:
				op = "tick"
				e.tick()
			}
			trace = append(trace, fmt.Sprintf("s%d.%d %s(%s)", seq, step, op, detail))
			if op == "noop" {
				continue
			}
			e.checkOp(op, prev, e.snap(), trace)
		}
	}
	// Leave nothing claimable behind even if cleanup were skipped.
	e.exec(`UPDATE export_jobs SET status = 'failed', finished_at = now(), error_message = 'w11 done' WHERE requested_by = ANY($1) AND status IN ('queued','running')`, e.userIDs())
	e.exec(`DELETE FROM export_job_chunks WHERE export_job_id IN (SELECT id FROM export_jobs WHERE requested_by = ANY($1))`, e.userIDs())
}

// ---- Property 11 ----------------------------------------------------------------------------

// succeeded creates a job through the Service and runs it through the real Worker code path.
func (e *w11Env) succeeded(a Actor, entity string, mode int32) Job {
	e.t.Helper()
	job, created, err := e.create(a, entity, "all")
	if err != nil || !created {
		e.t.Fatalf("create: created=%v err=%v", created, err)
	}
	m := new(atomic.Int32)
	m.Store(mode)
	w := w11Worker(e.pool, m)
	claimed, ok, err := w.store.ClaimNext(context.Background())
	if err != nil || !ok || claimed.ID != job.ID {
		e.t.Fatalf("claim: ok=%v id=%d want %d err=%v", ok, claimed.ID, job.ID, err)
	}
	w.run(claimed)
	if s := e.status(job.ID); s != "succeeded" {
		e.t.Fatalf("job %d status %s, want succeeded", job.ID, s)
	}
	return e.loadJob(job.ID)
}

func w11Read(t *testing.T, svc *Service, id int64, n int64, maxChunks int) ([]byte, int) {
	t.Helper()
	var out []byte
	got := 0
	for data, err := range svc.OpenChunks(context.Background(), id, n) {
		if err != nil {
			t.Fatalf("OpenChunks: %v", err)
		}
		out = append(out, data...)
		got++
		if maxChunks > 0 && got >= maxChunks {
			break
		}
	}
	return out, got
}

func TestW11Property11SingleDownload(t *testing.T) {
	e := w11NewEnv(t, 0)
	rng := w11Rng(t)
	ctx := context.Background()

	for round := range 5 {
		a := e.user(fmt.Sprintf("dl%d", round))
		e.users = append(e.users, a)
		mode := w11Big
		if round%2 == 1 {
			mode = w11OK
		}
		job := e.succeeded(a, w11Entities[rng.IntN(len(w11Entities))], mode)
		want := e.fileBytes(job.ID)
		wantChunks := e.chunks(job.ID)
		if job.ChunkCount == nil || int(*job.ChunkCount) != wantChunks || wantChunks == 0 {
			t.Fatalf("round %d: chunk_count=%v stored=%d", round, job.ChunkCount, wantChunks)
		}

		// concurrent claims: exactly one wins
		n := 2 + rng.IntN(9)
		var wg sync.WaitGroup
		start := make(chan struct{})
		errs := make([]error, n)
		jobs := make([]Job, n)
		for i := range n {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				jobs[i], errs[i] = e.svc.StartDownload(ctx, a, job.ID)
			}()
		}
		close(start)
		wg.Wait()
		winner := -1
		for i, err := range errs {
			switch {
			case err == nil:
				if winner >= 0 {
					t.Fatalf("round %d: two StartDownload winners (%d and %d of %d)", round, winner, i, n)
				}
				winner = i
			case !errors.Is(err, ErrNotReady) && !errors.Is(err, ErrGone):
				t.Fatalf("round %d: loser %d err=%v, want ErrNotReady/ErrGone", round, i, err)
			}
		}
		if winner < 0 {
			t.Fatalf("round %d: no winner among %d: %v", round, n, errs)
		}
		if jobs[winner].Status != StatusDownloading {
			t.Fatalf("round %d: winner job status %s", round, jobs[winner].Status)
		}
		if got := e.count(`SELECT count(*) FROM audit_logs WHERE actor_id = $1 AND action = $2`, a.UserID, ActionMasterDataExported); got != 1 {
			t.Fatalf("round %d: %d download audit rows, want 1", round, got)
		}
		body, _ := w11Read(t, e.svc, job.ID, *jobs[winner].ChunkCount, 0)
		if !bytes.Equal(body, want) {
			t.Fatalf("round %d: downloaded %d bytes != stored %d bytes", round, len(body), len(want))
		}
		if err := e.svc.CompleteDownload(ctx, job.ID); err != nil {
			t.Fatalf("round %d: complete: %v", round, err)
		}
		if s := e.status(job.ID); s != "downloaded" || e.chunks(job.ID) != 0 {
			t.Fatalf("round %d: after complete status=%s chunks=%d", round, s, e.chunks(job.ID))
		}
		if _, err := e.svc.StartDownload(ctx, a, job.ID); !errors.Is(err, ErrGone) {
			t.Fatalf("round %d: download after complete: %v, want ErrGone", round, err)
		}
		if err := e.svc.CompleteDownload(ctx, job.ID); !errors.Is(err, ErrNotDownloading) {
			t.Fatalf("round %d: second complete: %v, want ErrNotDownloading", round, err)
		}

		// interrupted download: abort leaves chunks intact and a re-download works
		b := e.user(fmt.Sprintf("ab%d", round))
		e.users = append(e.users, b)
		job2 := e.succeeded(b, w11Entities[rng.IntN(len(w11Entities))], mode)
		want2 := e.fileBytes(job2.ID)
		d1, err := e.svc.StartDownload(ctx, b, job2.ID)
		if err != nil {
			t.Fatalf("round %d: start job2: %v", round, err)
		}
		cut := 1 + rng.IntN(int(*d1.ChunkCount)) // cut after 1..all chunks: "interrupted at a random point"
		_, _ = w11Read(t, e.svc, job2.ID, *d1.ChunkCount, cut)
		if err := e.svc.AbortDownload(ctx, job2.ID); err != nil {
			t.Fatalf("round %d: abort: %v", round, err)
		}
		if s := e.status(job2.ID); s != "succeeded" || e.chunks(job2.ID) != int(*d1.ChunkCount) || !bytes.Equal(e.fileBytes(job2.ID), want2) {
			t.Fatalf("round %d: after abort status=%s chunks=%d/%d", round, s, e.chunks(job2.ID), *d1.ChunkCount)
		}
		d2, err := e.svc.StartDownload(ctx, b, job2.ID)
		if err != nil {
			t.Fatalf("round %d: re-download start: %v", round, err)
		}
		body2, _ := w11Read(t, e.svc, job2.ID, *d2.ChunkCount, 0)
		if !bytes.Equal(body2, want2) {
			t.Fatalf("round %d: re-download differs", round)
		}
		if err := e.svc.CompleteDownload(ctx, job2.ID); err != nil {
			t.Fatalf("round %d: complete job2: %v", round, err)
		}
		if got := e.count(`SELECT count(*) FROM audit_logs WHERE actor_id = $1 AND action = $2`, b.UserID, ActionMasterDataExported); got != 2 {
			t.Fatalf("round %d: %d audit rows for abort+redownload, want 2 (one per claim)", round, got)
		}
		if e.chunks(job2.ID) != 0 {
			t.Fatalf("round %d: chunks left after final complete", round)
		}
	}
}

// ---- Property 12 ----------------------------------------------------------------------------

type w11Seed struct {
	status                      string
	startedAgo, dlAgo, expiryIn *int // seconds; expiryIn negative = already past
	chunks                      int
}

var w11Hash atomic.Int64

func (e *w11Env) insertJob(userID int64, s w11Seed) int64 {
	e.t.Helper()
	ready := s.status == "succeeded" || s.status == "downloading" || s.status == "downloaded"
	var byteSize, chunkCount any
	if ready {
		byteSize, chunkCount = int64(10*s.chunks), int32(s.chunks)
	}
	var id int64
	err := e.pool.QueryRow(context.Background(), `
		INSERT INTO export_jobs (requested_by, kind, entity, vendor_id, vendor_code, status_filter, params_hash, file_name,
		                         status, row_count, byte_size, chunk_count, started_at, finished_at, expires_at, download_started_at, error_message)
		VALUES ($1, 'vendor', 'vendor', $2, $3, 'all', $4, 'w11.csv', $5, 1, $6, $7,
		        CASE WHEN $8::int IS NULL THEN NULL ELSE now() - make_interval(secs => $8::int) END,
		        CASE WHEN $5 IN ('queued','running') THEN NULL ELSE now() END,
		        CASE WHEN $9::int IS NULL THEN NULL ELSE now() + make_interval(secs => $9::int) END,
		        CASE WHEN $10::int IS NULL THEN NULL ELSE now() - make_interval(secs => $10::int) END,
		        CASE WHEN $5 = 'failed' THEN 'w11 seeded failure' END)
		RETURNING id`,
		userID, e.vendorID, strings.ToUpper(e.tag), fmt.Sprintf("%s-%d", e.tag, w11Hash.Add(1)), s.status,
		byteSize, chunkCount, s.startedAgo, s.expiryIn, s.dlAgo).Scan(&id)
	if err != nil {
		e.t.Fatalf("insert job %+v: %v", s, err)
	}
	for seq := range s.chunks {
		e.exec(`INSERT INTO export_job_chunks (export_job_id, seq, data) VALUES ($1, $2, $3)`, id, seq, []byte("a,b\n"))
	}
	return id
}

func w11I(v int) *int { return &v }

func (e *w11Env) rowJSON(id int64) string {
	var s string
	if err := e.pool.QueryRow(context.Background(), `SELECT to_jsonb(j)::text FROM export_jobs j WHERE id = $1`, id).Scan(&s); err != nil {
		e.t.Fatalf("row json: %v", err)
	}
	return s
}

func TestW11Property12JanitorWindows(t *testing.T) {
	e := w11NewEnv(t, 1)
	rng := w11Rng(t)
	uid := e.users[0].UserID

	type exp struct {
		id        int64
		name      string
		want      string // final status
		wantChunk int    // -1 = unchanged
		untouched bool
		before    string
	}
	future := func() *int { return w11I(120 + rng.IntN(3000)) }
	past := func() *int { return w11I(-(1 + rng.IntN(3000))) }
	staleRun := func() *int { return w11I(601 + rng.IntN(5000)) }
	freshRun := func() *int { return w11I(rng.IntN(580)) }
	staleDL := func() *int { return w11I(301 + rng.IntN(3000)) }
	freshDL := func() *int { return w11I(rng.IntN(280)) }

	for round := range 6 {
		var all []exp
		add := func(name string, s w11Seed, want string, wantChunk int, untouched bool) {
			id := e.insertJob(uid, s)
			x := exp{id: id, name: name, want: want, wantChunk: wantChunk, untouched: untouched}
			if untouched {
				x.before = e.rowJSON(id)
			}
			all = append(all, x)
		}
		for range rng.IntN(4) {
			add("running-stale", w11Seed{status: "running", startedAgo: staleRun(), chunks: 1 + rng.IntN(3)}, "failed", 0, false)
		}
		for range rng.IntN(4) {
			c := rng.IntN(3)
			add("running-fresh", w11Seed{status: "running", startedAgo: freshRun(), chunks: c}, "running", c, true)
		}
		for range rng.IntN(4) {
			c := 1 + rng.IntN(3)
			add("downloading-stale", w11Seed{status: "downloading", dlAgo: staleDL(), expiryIn: future(), chunks: c}, "succeeded", c, false)
		}
		for range rng.IntN(4) {
			c := 1 + rng.IntN(3)
			add("downloading-fresh", w11Seed{status: "downloading", dlAgo: freshDL(), expiryIn: future(), chunks: c}, "downloading", c, true)
		}
		for range rng.IntN(3) { // lease not over: expiry applies only to succeeded
			c := 1 + rng.IntN(3)
			add("downloading-fresh-past-expiry", w11Seed{status: "downloading", dlAgo: freshDL(), expiryIn: past(), chunks: c}, "downloading", c, true)
		}
		for range rng.IntN(4) { // recovered and expired in the same tick
			add("downloading-stale-past-expiry", w11Seed{status: "downloading", dlAgo: staleDL(), expiryIn: past(), chunks: 1 + rng.IntN(3)}, "expired", 0, false)
		}
		for range rng.IntN(4) {
			add("succeeded-past-expiry", w11Seed{status: "succeeded", expiryIn: past(), chunks: 1 + rng.IntN(3)}, "expired", 0, false)
		}
		for range rng.IntN(4) {
			c := 1 + rng.IntN(3)
			add("succeeded-valid", w11Seed{status: "succeeded", expiryIn: future(), chunks: c}, "succeeded", c, true)
		}
		for _, st := range []string{"queued", "failed", "downloaded", "expired"} {
			if rng.IntN(2) == 0 {
				add(st, w11Seed{status: st, expiryIn: w11I(0)}, st, 0, true)
			}
		}

		e.tick()

		for _, x := range all {
			if got := e.status(x.id); got != x.want {
				t.Fatalf("round %d: %s (job %d): status %s, want %s", round, x.name, x.id, got, x.want)
			}
			c := e.chunks(x.id)
			if x.wantChunk >= 0 && c != x.wantChunk {
				t.Fatalf("round %d: %s (job %d): %d chunks, want %d", round, x.name, x.id, c, x.wantChunk)
			}
			if x.untouched && e.rowJSON(x.id) != x.before {
				t.Fatalf("round %d: %s (job %d) row changed:\n before %s\n after  %s", round, x.name, x.id, x.before, e.rowJSON(x.id))
			}
			if x.name == "running-stale" {
				var msg *string
				if err := e.pool.QueryRow(context.Background(), `SELECT error_message FROM export_jobs WHERE id = $1`, x.id).Scan(&msg); err != nil || msg == nil || *msg != MsgTimeout {
					t.Fatalf("round %d: timed-out job %d error_message=%v err=%v, want %q", round, x.id, msg, err, MsgTimeout)
				}
			}
		}
		// A second tick changes nothing (idempotent).
		before := map[int64]string{}
		for _, x := range all {
			before[x.id] = e.rowJSON(x.id)
		}
		e.tick()
		for _, x := range all {
			if e.rowJSON(x.id) != before[x.id] {
				t.Fatalf("round %d: %s (job %d) changed on the second tick", round, x.name, x.id)
			}
		}
	}
	// Fresh running / queued jobs must not stay claimable or running after the test.
	e.exec(`UPDATE export_jobs SET status = 'failed', finished_at = now(), error_message = 'w11 done' WHERE requested_by = ANY($1) AND status IN ('queued','running')`, e.userIDs())
}

// ---- Property 15 ----------------------------------------------------------------------------

type w11CountStore struct {
	pgStore
	mu     *sync.Mutex
	claims map[int64]int
}

func (s w11CountStore) ClaimNext(ctx context.Context) (Job, bool, error) {
	j, ok, err := s.pgStore.ClaimNext(ctx)
	if ok {
		s.mu.Lock()
		s.claims[j.ID]++
		s.mu.Unlock()
	}
	return j, ok, err
}

// w11FailBranches runs the real producer, except entity "branches" which fails.
type w11FailBranches struct{ inner Producer }

func (p w11FailBranches) Header() []string { return p.inner.Header() }
func (p w11FailBranches) Produce(ctx context.Context, q Querier, pr Params, asOf time.Time, emit func([]string) error) error {
	if pr.Entity == "branches" {
		return errors.New("w11 injected failure")
	}
	return p.inner.Produce(ctx, q, pr, asOf, emit)
}

func TestW11Property15ClaimOnce(t *testing.T) {
	e := w11NewEnv(t, 4)
	rng := w11Rng(t)

	producers := map[Kind]map[string]Producer{KindVendor: {}}
	for ent, p := range DefaultProducers()[KindVendor] {
		producers[KindVendor][ent] = w11FailBranches{inner: p}
	}

	for round := range 4 {
		n := 1 + rng.IntN(50)
		wa := 1 + rng.IntN(7)
		wb := 1 + rng.IntN(8-wa)
		entityOf := map[int64]string{}
		for range n {
			ent := w11Entities[rng.IntN(len(w11Entities))]
			s := w11Seed{status: "queued"}
			id := e.insertJob(e.users[rng.IntN(len(e.users))].UserID, s)
			e.exec(`UPDATE export_jobs SET entity = $2 WHERE id = $1`, id, ent)
			entityOf[id] = ent
		}

		counter := &sync.Mutex{}
		claims := map[int64]int{}
		mk := func(workers int) *Worker {
			cfg := w11Cfg
			cfg.Workers = workers
			st := w11CountStore{pgStore: pgStore{pool: e.pool, q: repository.New(e.pool)}, mu: counter, claims: claims}
			return newWorker(st, pgReader{pool: e.pool}, producers, cfg)
		}
		w1, w2 := mk(wa), mk(wb)
		ctx, cancel := context.WithCancel(context.Background())
		var run sync.WaitGroup
		for _, w := range []*Worker{w1, w2} {
			run.Add(1)
			go func() { defer run.Done(); w.Run(ctx) }()
		}

		deadline := time.Now().Add(90 * time.Second)
		for e.count(`SELECT count(*) FROM export_jobs WHERE requested_by = ANY($1) AND status IN ('queued','running')`, e.userIDs()) > 0 {
			if time.Now().After(deadline) {
				cancel()
				w1.Wait(time.Second)
				w2.Wait(time.Second)
				run.Wait()
				t.Fatalf("round %d: jobs still active after 90s (n=%d workers=%d+%d)", round, n, wa, wb)
			}
			time.Sleep(20 * time.Millisecond)
		}
		cancel()
		w1.Wait(5 * time.Second)
		w2.Wait(5 * time.Second)
		run.Wait()

		counter.Lock()
		for id, ent := range entityOf {
			if claims[id] != 1 {
				t.Fatalf("round %d: job %d claimed %d times (n=%d workers=%d+%d)", round, id, claims[id], n, wa, wb)
			}
			want := "succeeded"
			if ent == "branches" {
				want = "failed"
			}
			if got := e.status(id); got != want {
				t.Fatalf("round %d: job %d (%s) ended %s, want %s", round, id, ent, got, want)
			}
			if e.count(`SELECT count(*) FROM export_jobs WHERE id = $1 AND started_at IS NOT NULL AND finished_at IS NOT NULL`, id) != 1 {
				t.Fatalf("round %d: job %d missing started_at/finished_at", round, id)
			}
			c := e.chunks(id)
			if want == "failed" && c != 0 || want == "succeeded" && c == 0 {
				t.Fatalf("round %d: job %d (%s) has %d chunks", round, id, want, c)
			}
		}
		if len(claims) != n {
			t.Fatalf("round %d: %d distinct claims, want %d (stray claim of a foreign job?)", round, len(claims), n)
		}
		counter.Unlock()
		t.Logf("round %d: n=%d workers=%d+%d ok", round, n, wa, wb)
	}
}
