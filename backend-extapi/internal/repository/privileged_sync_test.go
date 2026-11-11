package repository

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/cimb-niaga/cms/pkg/userrules"
)

// The CHECK on api_clients.assignable_roles in migration 039 must list exactly
// userrules.PrivilegedRoles (D12); otherwise the DB and the Go guard drift apart.
func TestMigration039CheckMatchesPrivilegedRoles(t *testing.T) {
	sql, err := os.ReadFile("../../../backend/migrations/039_ext_api_clients.sql")
	if err != nil {
		t.Fatalf("read migration 039: %v", err)
	}
	m := regexp.MustCompile(`(?s)NOT\s*\(\s*assignable_roles\s*&&\s*ARRAY\[([^\]]*)\]`).FindSubmatch(sql)
	if m == nil {
		t.Fatal("assignable_roles CHECK not found in migration 039")
	}
	var got []string
	for _, p := range strings.Split(string(m[1]), ",") {
		got = append(got, strings.Trim(strings.TrimSpace(p), "'"))
	}
	want := slices.Clone(userrules.PrivilegedRoles)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("migration 039 CHECK roles %v != userrules.PrivilegedRoles %v", got, want)
	}
}
