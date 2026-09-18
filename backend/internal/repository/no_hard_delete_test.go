package repository

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// stripSQLLineComments removes `-- ...` line comments before pattern
// matching, so a doc comment that merely mentions "DELETE FROM users" in
// prose (explaining why there isn't one) doesn't false-positive.
func stripSQLLineComments(content []byte) string {
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		if idx := strings.Index(line, "--"); idx != -1 {
			lines[i] = line[:idx]
		}
	}
	return strings.Join(lines, "\n")
}

// TestQueries_NoHardDelete is the "guard di repo: tidak ada path SQL DELETE
// FROM <table>" guarantee, originally Auth-Local-Lifecycle Task 7 for users,
// extended to vendors by the Admin User & Vendor Management spec (Req 5.3,
// 8.3, 14.6), and extended to atms by the Admin ATM Management spec (Req
// 5.3). There is no runtime code path to test for any of these tables
// (there's simply no such method), so this test guards the invariant at its
// source: no queries/*.sql file may ever define one. A user, vendor, or atm
// row must never be physically removed — audit_logs rows (and, for users,
// audit_logs.actor_id; for atms, child rows in atm_denoms/
// atm_vendor_packages via FK CASCADE) referencing it would dangle or
// cascade-delete unexpectedly otherwise. Disable/enable (soft-delete via
// is_active/deleted_at) are the only lifecycle transitions.
func TestQueries_NoHardDelete(t *testing.T) {
	files, err := filepath.Glob("../../queries/*.sql")
	if err != nil {
		t.Fatalf("glob queries/*.sql: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no query files found — check the glob path (expected backend/queries relative to this package)")
	}

	tables := []string{"users", "vendors", "atms"}

	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		sqlOnly := stripSQLLineComments(content)

		for _, table := range tables {
			// \b<table>\b so "user_leaves"/"vendor_branches" etc. don't false-positive.
			hardDelete := regexp.MustCompile(`(?is)DELETE\s+FROM\s+(public\.)?\b` + table + `\b`)
			if hardDelete.MatchString(sqlOnly) {
				t.Errorf("%s defines a hard DELETE FROM %s — forbidden; use the existing soft-delete (Disable/Deactivate) path instead", f, table)
			}
		}
	}
}
