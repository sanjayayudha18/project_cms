package repository

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Property 12: no backend-extapi query hard-deletes users, api_clients or audit_logs.
var guardedTables = []string{"users", "api_clients", "audit_logs", "roles"}

func stripSQLLineComments(content []byte) string {
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		if idx := strings.Index(line, "--"); idx != -1 {
			lines[i] = line[:idx]
		}
	}
	return strings.Join(lines, "\n")
}

func hardDeleteRe(table string) *regexp.Regexp {
	return regexp.MustCompile(`(?is)DELETE\s+FROM\s+(public\.)?\b` + table + `\b`)
}

func TestQueries_NoHardDelete(t *testing.T) {
	files, err := filepath.Glob("../../queries/*.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("glob queries/*.sql: %v (found %d)", err, len(files))
	}
	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		sqlOnly := stripSQLLineComments(content)
		for _, table := range guardedTables {
			if hardDeleteRe(table).MatchString(sqlOnly) {
				t.Errorf("%s defines a hard DELETE FROM %s — forbidden; use is_active/deleted_at", f, table)
			}
		}
	}
}

// Keeps the scan above from passing vacuously.
func TestNoHardDeletePattern_ActuallyFires(t *testing.T) {
	for _, table := range guardedTables {
		re := hardDeleteRe(table)
		for _, sql := range []string{
			"DELETE FROM " + table + " WHERE id = $1;",
			"delete from public." + table + " where id = $1",
			"DELETE\n  FROM   " + table,
		} {
			if !re.MatchString(stripSQLLineComments([]byte(sql))) {
				t.Errorf("pattern failed to flag %q", sql)
			}
		}
		if re.MatchString(stripSQLLineComments([]byte("-- DELETE FROM " + table + "\nSELECT 1;"))) {
			t.Errorf("pattern must ignore a commented-out mention for %s", table)
		}
		if re.MatchString("DELETE FROM " + table + "_archive WHERE id = $1") {
			t.Errorf("pattern must not flag look-alike %s_archive", table)
		}
	}
}
