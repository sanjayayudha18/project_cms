package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/jackc/pgx/v5"
)

// lockKey is hashed by Postgres (hashtext) into the session advisory lock held for a whole run,
// so two runs against the same database never apply files at the same time.
const lockKey = "cms.schema_migrations"

// schema_migrations is created by the runner itself, not by a migration file (spec S2).
const createTableSQL = `CREATE TABLE IF NOT EXISTS schema_migrations (
	version    text PRIMARY KEY,
	checksum   text NOT NULL,
	applied_at timestamptz NOT NULL DEFAULT now(),
	applied_by text NOT NULL,
	baseline   boolean NOT NULL DEFAULT false
)`

// fileRe matches the top-level migration files. Exactly three digits, so name order == numeric order.
var fileRe = regexp.MustCompile(`^\d{3}_.*\.sql$`)

type migration struct {
	Version  string // file name, e.g. 029_vendor_request_send.sql
	Path     string
	Checksum string // SHA-256 hex of the file bytes
}

type record struct {
	Checksum string
	Baseline bool
}

type runner struct {
	conn   *pgx.Conn
	cfg    *pgx.ConnConfig
	dir    string
	stdout io.Writer
	stderr io.Writer
}

// newRunner connects, takes the advisory lock (blocking while another run holds it) and makes
// sure schema_migrations exists. Close releases the lock.
func newRunner(ctx context.Context, cfg *pgx.ConnConfig, dir string, stdout, stderr io.Writer) (*runner, error) {
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock(hashtext($1))`, lockKey); err != nil {
		_ = conn.Close(ctx)
		return nil, fmt.Errorf("advisory lock: %w", err)
	}
	if _, err := conn.Exec(ctx, createTableSQL); err != nil {
		_ = conn.Close(ctx)
		return nil, fmt.Errorf("create schema_migrations: %w", err)
	}
	return &runner{conn: conn, cfg: cfg, dir: dir, stdout: stdout, stderr: stderr}, nil
}

// Close ends the session, which also releases the advisory lock.
func (r *runner) Close(ctx context.Context) error {
	return r.conn.Close(ctx)
}

// Status prints every file (and every recorded version without a file) with its state.
// It returns an error when an applied file's checksum changed.
func (r *runner) Status(ctx context.Context) error {
	files, applied, err := r.load(ctx)
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(r.stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "VERSION\tSTATE\tCHECKSUM\tBASELINE")
	seen := make(map[string]bool, len(files))
	nApplied, nPending := 0, 0
	for _, f := range files {
		seen[f.Version] = true
		rec, ok := applied[f.Version]
		if !ok {
			nPending++
			_, _ = fmt.Fprintf(tw, "%s\tpending\t-\t-\n", f.Version)
			continue
		}
		nApplied++
		sum := "ok"
		if rec.Checksum != f.Checksum {
			sum = "MISMATCH"
		}
		_, _ = fmt.Fprintf(tw, "%s\tapplied\t%s\t%t\n", f.Version, sum, rec.Baseline)
	}
	for _, v := range sortedKeys(applied) {
		if !seen[v] {
			_, _ = fmt.Fprintf(tw, "%s\tapplied\tno file\t%t\n", v, applied[v].Baseline)
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(r.stdout, "%d applied, %d pending\n", nApplied, nPending)
	return checkChecksums(files, applied)
}

// Up applies every file not recorded yet (lower numbers that appear later included), in order,
// one psql run per file. The row is written only after psql succeeds; the first failure stops.
func (r *runner) Up(ctx context.Context) error {
	files, applied, err := r.load(ctx)
	if err != nil {
		return err
	}
	if err := checkChecksums(files, applied); err != nil {
		return err
	}
	n := 0
	for _, f := range files {
		if _, ok := applied[f.Version]; ok {
			continue
		}
		_, _ = fmt.Fprintf(r.stdout, "applying %s\n", f.Version)
		if err := r.psql(ctx, f.Path); err != nil {
			return fmt.Errorf("apply %s: %w", f.Version, err)
		}
		if _, err := r.conn.Exec(ctx,
			`INSERT INTO schema_migrations (version, checksum, applied_by) VALUES ($1, $2, current_user)`,
			f.Version, f.Checksum); err != nil {
			return fmt.Errorf("record %s (its SQL already ran; insert the row by hand): %w", f.Version, err)
		}
		n++
	}
	_, _ = fmt.Fprintf(r.stdout, "%d applied\n", n)
	return nil
}

// Baseline records every unrecorded file numbered <= through as applied without running it.
func (r *runner) Baseline(ctx context.Context, through int) error {
	files, applied, err := r.load(ctx)
	if err != nil {
		return err
	}
	if err := checkChecksums(files, applied); err != nil {
		return err
	}
	n := 0
	for _, f := range files {
		num, _ := strconv.Atoi(f.Version[:3]) // fileRe guarantees three digits
		if _, ok := applied[f.Version]; ok || num > through {
			continue
		}
		if _, err := r.conn.Exec(ctx,
			`INSERT INTO schema_migrations (version, checksum, applied_by, baseline) VALUES ($1, $2, current_user, true)`,
			f.Version, f.Checksum); err != nil {
			return fmt.Errorf("baseline %s: %w", f.Version, err)
		}
		_, _ = fmt.Fprintf(r.stdout, "baselined %s\n", f.Version)
		n++
	}
	_, _ = fmt.Fprintf(r.stdout, "%d baselined\n", n)
	return nil
}

func (r *runner) load(ctx context.Context) ([]migration, map[string]record, error) {
	files, err := readMigrations(r.dir)
	if err != nil {
		return nil, nil, err
	}
	rows, err := r.conn.Query(ctx, `SELECT version, checksum, baseline FROM schema_migrations`)
	if err != nil {
		return nil, nil, fmt.Errorf("read schema_migrations: %w", err)
	}
	defer rows.Close()
	applied := map[string]record{}
	for rows.Next() {
		var v string
		var rec record
		if err := rows.Scan(&v, &rec.Checksum, &rec.Baseline); err != nil {
			return nil, nil, fmt.Errorf("scan schema_migrations: %w", err)
		}
		applied[v] = rec
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("read schema_migrations: %w", err)
	}
	return files, applied, nil
}

// psql runs one file exactly like the old manual step: the file controls its own BEGIN/COMMIT
// (006 uses CREATE INDEX CONCURRENTLY, which cannot run inside a transaction).
func (r *runner) psql(ctx context.Context, path string) error {
	//nolint:gosec // G204: fixed binary and flags; path comes from readMigrations (own dir listing, fileRe), not user input.
	cmd := exec.CommandContext(ctx, "psql", "-v", "ON_ERROR_STOP=1", "-X", "-q", "-f", path)
	cmd.Env = psqlEnv(os.Environ(), r.cfg)
	cmd.Stdout, cmd.Stderr = r.stdout, r.stderr
	return cmd.Run()
}

// readMigrations lists NNN_*.sql at the top level of dir (archives/ and other files ignored),
// sorted by name, with the SHA-256 of each file.
func readMigrations(dir string) ([]migration, error) {
	entries, err := os.ReadDir(dir) // sorted by file name
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}
	var out []migration
	for _, e := range entries {
		if e.IsDir() || !fileRe.MatchString(e.Name()) {
			continue
		}
		path := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(path) //nolint:gosec // G304: path is a file name from our own os.ReadDir listing of dir.
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		sum := sha256.Sum256(b)
		out = append(out, migration{Version: e.Name(), Path: path, Checksum: hex.EncodeToString(sum[:])})
	}
	return out, nil
}

func checkChecksums(files []migration, applied map[string]record) error {
	var bad []string
	for _, f := range files {
		if rec, ok := applied[f.Version]; ok && rec.Checksum != f.Checksum {
			bad = append(bad, f.Version)
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("checksum mismatch, file changed after it was applied (nothing run): %s", strings.Join(bad, ", "))
	}
	return nil
}

// psqlEnv passes the connection to psql through PG* variables, so the password never appears
// in argv. Later entries win in os/exec, so these override inherited PG* values; other
// inherited ones (e.g. PGSSLROOTCERT) still apply.
// ponytail: only the first host is passed; multi-host DATABASE_URLs are not supported.
func psqlEnv(base []string, cfg *pgx.ConnConfig) []string {
	env := make([]string, 0, len(base)+6)
	env = append(env, base...)
	return append(env,
		"PGHOST="+cfg.Host,
		"PGPORT="+strconv.Itoa(int(cfg.Port)),
		"PGUSER="+cfg.User,
		"PGPASSWORD="+cfg.Password,
		"PGDATABASE="+cfg.Database,
		"PGSSLMODE="+sslMode(cfg),
	)
}

// sslMode reverses pgconn's sslmode → TLS config mapping (pgconn/config.go configTLS).
func sslMode(cfg *pgx.ConnConfig) string {
	plainFallback, tlsFallback := false, false
	for _, fb := range cfg.Fallbacks {
		if fb.TLSConfig == nil {
			plainFallback = true
		} else {
			tlsFallback = true
		}
	}
	tc := cfg.TLSConfig
	switch {
	case tc == nil && tlsFallback:
		return "allow"
	case tc == nil:
		return "disable"
	case plainFallback:
		return "prefer"
	case tc.VerifyPeerCertificate != nil:
		return "verify-ca"
	case tc.InsecureSkipVerify:
		return "require"
	default:
		return "verify-full"
	}
}

func sortedKeys(m map[string]record) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
