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

// TestQueries_NoHardDeleteFromUsers is the "guard di repo: tidak ada path
// SQL DELETE FROM users" from Auth-Local-Lifecycle Task 7. There is no
// runtime code path to test (there's simply no such method), so this test
// guards the invariant at its source: no queries/*.sql file may ever define
// one. A user row must never be physically removed — audit_logs.actor_id
// rows referencing it would dangle otherwise.
func TestQueries_NoHardDeleteFromUsers(t *testing.T) {
	files, err := filepath.Glob("../../queries/*.sql")
	if err != nil {
		t.Fatalf("glob queries/*.sql: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no query files found — check the glob path (expected backend/queries relative to this package)")
	}

	// \busers\b so "user_leaves"/"user_pics" etc. don't false-positive.
	hardDelete := regexp.MustCompile(`(?is)DELETE\s+FROM\s+(public\.)?\busers\b`)

	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		sqlOnly := stripSQLLineComments(content)
		if hardDelete.MatchString(sqlOnly) {
			t.Errorf("%s defines a hard DELETE FROM users — forbidden by Task 7; use DeactivateUser (soft-delete) instead", f)
		}
	}
}
