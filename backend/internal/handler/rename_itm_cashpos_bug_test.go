//go:build integration

package handler

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// oldTableNamePattern matches the legacy table name itm_cashpos. It also
// matches itm_cashpos_files since that name contains itm_cashpos as a
// prefix, so one pattern covers both bug-condition names from design.md's
// Glossary (Bug_Condition).
var oldTableNamePattern = regexp.MustCompile(`itm_cashpos`)

// codeExtensionsToScan mirrors the extension filter used by task 4's final
// grep verification (*.go, *.sql, *.py, *.tsx, *.ts).
var codeExtensionsToScan = map[string]bool{
	".go":  true,
	".sql": true,
	".py":  true,
	".tsx": true,
	".ts":  true,
}

// TestBugCondition_TablesRenamedToItmReplenish is Property 1's schema
// check: itm_replenish / itm_replenish_files must exist in
// information_schema.tables. On unfixed code (old itm_cashpos names still
// in place) this FAILS — that failure is the expected, documented baseline
// for task 1. Do not "fix" this test to make it pass before the rename
// migration (task 3.1) actually lands.
func TestBugCondition_TablesRenamedToItmReplenish(t *testing.T) {
	pool := connectForBugConditionTest(t)
	ctx := context.Background()

	for _, table := range []string{"itm_replenish", "itm_replenish_files"} {
		var exists bool
		err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = 'public' AND table_name = $1
			)
		`, table).Scan(&exists)
		if err != nil {
			t.Fatalf("querying information_schema.tables for %s: %v", table, err)
		}
		if !exists {
			t.Errorf("bug condition: table %q does not exist yet (schema still uses itm_cashpos naming)", table)
		}
	}
}

// TestBugCondition_IndexesUseItmReplenishNaming is Property 1's index
// check: every index/constraint on the renamed tables must carry the
// itm_replenish prefix. On unfixed code this FAILS (indexes are still
// itm_cashpos_*) — expected.
func TestBugCondition_IndexesUseItmReplenishNaming(t *testing.T) {
	pool := connectForBugConditionTest(t)
	ctx := context.Background()

	wantIndexes := []string{
		"itm_replenish_pkey",
		"itm_replenish_files_pkey",
		"itm_replenish_files_checksum_uq",
		"itm_replenish_file_idx",
		"itm_replenish_terminal_date_idx",
		"itm_replenish_replenish_date_idx",
		"itm_replenish_branch_code_idx",
	}

	rows, err := pool.Query(ctx, `
		SELECT indexname FROM pg_indexes
		WHERE schemaname = 'public'
		  AND tablename IN ('itm_replenish', 'itm_replenish_files')
	`)
	if err != nil {
		t.Fatalf("querying pg_indexes: %v", err)
	}
	defer rows.Close()

	found := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scanning pg_indexes row: %v", err)
		}
		found[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating pg_indexes: %v", err)
	}

	for _, want := range wantIndexes {
		if !found[want] {
			t.Errorf("bug condition: index/constraint %q not found (schema still uses itm_cashpos naming)", want)
		}
	}
}

// TestBugCondition_NoOldTableNameReferencesInCode is Property 1's code-scan
// check: zero references to itm_cashpos / itm_cashpos_files should remain
// in application source (Go, SQL, Python, TS/TSX). backend/migrations is
// intentionally excluded — 009_itm_cashpos.sql and the new rename migration
// both legitimately contain the old name as historical DDL/rollback
// source; per design.md, existing migration files are never edited in
// place. On unfixed code this FAILS — expected.
func TestBugCondition_NoOldTableNameReferencesInCode(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolving repo root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "backend", "go.mod")); err != nil {
		t.Fatalf("repo root sanity check failed (backend/go.mod not found at %s): %v", repoRoot, err)
	}

	scanDirs := []string{"backend", "scheduler", "frontend"}
	excludeDirNames := map[string]bool{
		"migrations":   true, // historical DDL, legitimately names old tables
		"node_modules": true,
		".git":         true,
		"dist":         true,
		"build":        true,
	}

	// safeLinePatterns matches lines that legitimately contain "itm_cashpos"
	// without being a stale bug-condition reference: historical migration
	// filenames (migration files are never renamed once applied), the
	// Python ETL module's real filename/import (itm_cashpos_etl.py was
	// never scheduled for a rename in task 3.4 — only its SQL string
	// constants were), and "before -> after" documentation notation that
	// names the old table alongside the new one to describe the rename
	// itself. Matching lines are stripped before the bug-condition scan;
	// any *other* itm_cashpos reference in the same file still trips it.
	safeLinePatterns := []*regexp.Regexp{
		regexp.MustCompile(`\d+(?:\.\d+)?_itm_cashpos[\w.]*\.sql`),                  // migration filenames, e.g. 009_itm_cashpos.sql
		regexp.MustCompile(`\d+_rename_itm_cashpos_to_itm_replenish\.sql`),          // the rename migration's own filename
		regexp.MustCompile(`itm_cashpos_etl(?:\.py)?`),                             // Python ETL module filename/import
		regexp.MustCompile(`itm_cashpos(?:_files)?\s*->\s*itm_replenish`),          // "old -> new" doc notation
	}

	// This test file itself necessarily names the old tables (in the regex
	// pattern and comments) in order to test for them — exclude it from
	// its own scan, or it would flag itself forever, even after the real
	// fix lands.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolving this test file's own path via runtime.Caller")
	}

	var matches []string
	for _, dir := range scanDirs {
		root := filepath.Join(repoRoot, dir)
		if _, statErr := os.Stat(root); statErr != nil {
			continue
		}
		walkErr := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if excludeDirNames[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			absPath, absErr := filepath.Abs(path)
			if absErr == nil && strings.EqualFold(filepath.ToSlash(absPath), filepath.ToSlash(thisFile)) {
				return nil
			}
			if !codeExtensionsToScan[strings.ToLower(filepath.Ext(d.Name()))] {
				return nil
			}
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			rel, relErr := filepath.Rel(repoRoot, path)
			if relErr != nil {
				rel = path
			}
			lines := strings.Split(string(content), "\n")
			var remaining []string
			for _, line := range lines {
				safe := false
				for _, pat := range safeLinePatterns {
					if pat.MatchString(line) {
						safe = true
						break
					}
				}
				if !safe {
					remaining = append(remaining, line)
				}
			}
			if oldTableNamePattern.MatchString(strings.Join(remaining, "\n")) {
				matches = append(matches, rel)
			}
			return nil
		})
		if walkErr != nil {
			t.Fatalf("walking %s: %v", root, walkErr)
		}
	}

	if len(matches) > 0 {
		t.Errorf("bug condition: %d file(s) still reference itm_cashpos: %s", len(matches), strings.Join(matches, ", "))
	}
}

// connectForBugConditionTest connects to DATABASE_URL and ensures the
// shared schema is migrated, reusing the same harness pattern as
// integration_test.go's setupHarness/runMigrations.
func connectForBugConditionTest(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connecting to database: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("pinging database: %v", err)
	}

	runMigrations(t, pool)
	return pool
}
