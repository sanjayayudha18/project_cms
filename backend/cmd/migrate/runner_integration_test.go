//go:build integration

package main

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// tempDB creates an empty database next to DATABASE_URL's and drops it after the test.
// It returns the pgx config and a URL for the new database.
func tempDB(t *testing.T) (*pgx.ConnConfig, string) {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}
	if _, err := exec.LookPath("psql"); err != nil {
		t.Fatal("psql not on PATH")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	name := fmt.Sprintf("cms_migrate_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		_ = admin.Close(ctx)
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("drop database %s: %v", name, err)
		}
		_ = admin.Close(ctx)
	})

	cfg, err := pgx.ParseConfig(dbURL)
	if err != nil {
		t.Fatalf("parse DATABASE_URL: %v", err)
	}
	cfg.Database = name
	u, err := url.Parse(dbURL)
	if err != nil {
		t.Fatalf("DATABASE_URL is not a URL: %v", err)
	}
	u.Path = "/" + name
	return cfg, u.String()
}

func writeSQL(t *testing.T, dir, name, sql string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(sql), 0o600); err != nil {
		t.Fatal(err)
	}
}

// migrate opens a runner, calls fn and closes it (releasing the lock); output goes to out.
func migrate(t *testing.T, cfg *pgx.ConnConfig, dir string, out *bytes.Buffer, fn func(*runner) error) error {
	t.Helper()
	ctx := context.Background()
	r, err := newRunner(ctx, cfg, dir, out, out)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close(ctx) }()
	return fn(r)
}

func up(r *runner) error     { return r.Up(context.Background()) }
func status(r *runner) error { return r.Status(context.Background()) }

func queryInt(t *testing.T, cfg *pgx.ConnConfig, sql string) int {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = conn.Close(ctx) }()
	var n int
	if err := conn.QueryRow(ctx, sql).Scan(&n); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return n
}

func tableExists(t *testing.T, cfg *pgx.ConnConfig, table string) bool {
	t.Helper()
	return queryInt(t, cfg, fmt.Sprintf(`SELECT count(*) FROM pg_tables WHERE schemaname = 'public' AND tablename = '%s'`, table)) == 1
}

func TestUpAppliesAllThenIsNoop(t *testing.T) {
	cfg, dbURL := tempDB(t)
	dir := t.TempDir()
	writeSQL(t, dir, "001_a.sql", "BEGIN;\nCREATE TABLE a (id int);\nCOMMIT;\n")
	writeSQL(t, dir, "002_seed.sql", "INSERT INTO a VALUES (1);\n")
	// CONCURRENTLY fails inside a transaction: proves files run as-is, like migration 006.
	writeSQL(t, dir, "003_idx.sql", "CREATE INDEX CONCURRENTLY a_id_idx ON a (id);\n")

	var out bytes.Buffer
	if err := migrate(t, cfg, dir, &out, up); err != nil {
		t.Fatalf("up: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "3 applied") {
		t.Errorf("first up output = %q; want 3 applied", out.String())
	}
	if n := queryInt(t, cfg, `SELECT count(*) FROM schema_migrations WHERE NOT baseline AND applied_by = current_user`); n != 3 {
		t.Errorf("recorded rows = %d; want 3", n)
	}

	out.Reset()
	if err := migrate(t, cfg, dir, &out, up); err != nil {
		t.Fatalf("second up: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "0 applied") || strings.Contains(out.String(), "applying") {
		t.Errorf("second up output = %q; want no-op", out.String())
	}
	if n := queryInt(t, cfg, `SELECT count(*) FROM a`); n != 1 {
		t.Errorf("rows in a = %d; want 1 (seed ran once)", n)
	}

	// Same through the CLI entry point.
	var cli bytes.Buffer
	if err := run(context.Background(), []string{"-dir", dir, "status"}, dbURL, &cli, &cli); err != nil {
		t.Fatalf("run status: %v\n%s", err, cli.String())
	}
	if !strings.Contains(cli.String(), "3 applied, 0 pending") {
		t.Errorf("status output = %q", cli.String())
	}
}

func TestChecksumMismatchStopsBeforeRunningAnything(t *testing.T) {
	cfg, _ := tempDB(t)
	dir := t.TempDir()
	writeSQL(t, dir, "001_a.sql", "CREATE TABLE a (id int);\n")
	var out bytes.Buffer
	if err := migrate(t, cfg, dir, &out, up); err != nil {
		t.Fatalf("up: %v\n%s", err, out.String())
	}

	writeSQL(t, dir, "001_a.sql", "CREATE TABLE a (id int); -- edited\n")
	writeSQL(t, dir, "002_b.sql", "CREATE TABLE b (id int);\n")
	out.Reset()
	err := migrate(t, cfg, dir, &out, up)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") || !strings.Contains(err.Error(), "001_a.sql") {
		t.Fatalf("up err = %v; want checksum mismatch on 001_a.sql", err)
	}
	if tableExists(t, cfg, "b") {
		t.Error("002_b.sql ran despite the mismatch")
	}

	out.Reset()
	if err := migrate(t, cfg, dir, &out, status); err == nil {
		t.Error("status err = nil; want checksum mismatch")
	}
	if !strings.Contains(out.String(), "MISMATCH") || !strings.Contains(out.String(), "1 applied, 1 pending") {
		t.Errorf("status output = %q", out.String())
	}
}

func TestBaselineMarksWithoutRunningSQL(t *testing.T) {
	cfg, _ := tempDB(t)
	dir := t.TempDir()
	writeSQL(t, dir, "001_boom.sql", "SELECT 1/0;\n")
	writeSQL(t, dir, "002_boom.sql", "SELECT 1/0;\n")
	writeSQL(t, dir, "003_c.sql", "CREATE TABLE c (id int);\n")

	var out bytes.Buffer
	if err := migrate(t, cfg, dir, &out, func(r *runner) error { return r.Baseline(context.Background(), 2) }); err != nil {
		t.Fatalf("baseline: %v\n%s", err, out.String())
	}
	if n := queryInt(t, cfg, `SELECT count(*) FROM schema_migrations WHERE baseline`); n != 2 {
		t.Errorf("baseline rows = %d; want 2", n)
	}
	if tableExists(t, cfg, "c") {
		t.Error("baseline ran 003 (> through)")
	}

	// up must only run 003: running 001/002 would fail with division by zero.
	out.Reset()
	if err := migrate(t, cfg, dir, &out, up); err != nil {
		t.Fatalf("up after baseline: %v\n%s", err, out.String())
	}
	if !tableExists(t, cfg, "c") || !strings.Contains(out.String(), "1 applied") {
		t.Errorf("up after baseline output = %q; want only 003 applied", out.String())
	}
}

func TestLowerNumberAddedLaterIsApplied(t *testing.T) {
	cfg, _ := tempDB(t)
	dir := t.TempDir()
	writeSQL(t, dir, "001_a.sql", "CREATE TABLE a (id int);\n")
	writeSQL(t, dir, "003_c.sql", "CREATE TABLE c (id int);\n")
	var out bytes.Buffer
	if err := migrate(t, cfg, dir, &out, up); err != nil {
		t.Fatalf("up: %v\n%s", err, out.String())
	}

	writeSQL(t, dir, "002_b.sql", "CREATE TABLE b (id int);\n") // like 022 created after 031
	out.Reset()
	if err := migrate(t, cfg, dir, &out, up); err != nil {
		t.Fatalf("up: %v\n%s", err, out.String())
	}
	if !tableExists(t, cfg, "b") || !strings.Contains(out.String(), "applying 002_b.sql") {
		t.Errorf("late 002 not applied; output = %q", out.String())
	}
}

func TestFailureStopsAndIsNotRecorded(t *testing.T) {
	cfg, _ := tempDB(t)
	dir := t.TempDir()
	writeSQL(t, dir, "001_a.sql", "CREATE TABLE a (id int);\n")
	writeSQL(t, dir, "002_bad.sql", "CREATE TABLE nope (id no_such_type);\n")
	writeSQL(t, dir, "003_c.sql", "CREATE TABLE c (id int);\n")

	var out bytes.Buffer
	err := migrate(t, cfg, dir, &out, up)
	if err == nil || !strings.Contains(err.Error(), "002_bad.sql") {
		t.Fatalf("up err = %v; want failure on 002_bad.sql", err)
	}
	if !strings.Contains(out.String(), "no_such_type") {
		t.Errorf("psql stderr not forwarded; output = %q", out.String())
	}
	if n := queryInt(t, cfg, `SELECT count(*) FROM schema_migrations`); n != 1 {
		t.Errorf("recorded rows = %d; want 1 (only 001)", n)
	}
	if tableExists(t, cfg, "c") {
		t.Error("003 ran after 002 failed")
	}
}

func TestConcurrentUpAppliesOnce(t *testing.T) {
	cfg, _ := tempDB(t)
	dir := t.TempDir()
	// The sleep keeps the first run inside the lock while the second one starts.
	writeSQL(t, dir, "001_slow.sql", "SELECT pg_sleep(1);\nCREATE TABLE once_t (id int);\n")
	writeSQL(t, dir, "002_seed.sql", "INSERT INTO once_t VALUES (1);\n")

	var wg sync.WaitGroup
	var errs [2]error
	var outs [2]bytes.Buffer
	for i := range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = migrate(t, cfg, dir, &outs[i], up)
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("run %d: %v\n%s", i, err, outs[i].String())
		}
	}
	got := outs[0].String() + "|" + outs[1].String()
	if !strings.Contains(got, "2 applied") || !strings.Contains(got, "0 applied") {
		t.Errorf("outputs = %q; want one run applying 2 and the other 0", got)
	}
	if n := queryInt(t, cfg, `SELECT count(*) FROM once_t`); n != 1 {
		t.Errorf("rows in once_t = %d; want 1", n)
	}
	if n := queryInt(t, cfg, `SELECT count(*) FROM schema_migrations`); n != 2 {
		t.Errorf("recorded rows = %d; want 2", n)
	}
}
