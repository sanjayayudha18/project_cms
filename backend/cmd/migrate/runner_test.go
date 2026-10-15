package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestSSLModeRoundTrip(t *testing.T) {
	for _, mode := range []string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full"} {
		t.Run(mode, func(t *testing.T) {
			cfg, err := pgx.ParseConfig("postgres://u:p@db.example:5432/cms?sslmode=" + mode)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := sslMode(cfg); got != mode {
				t.Errorf("sslMode = %q; want %q", got, mode)
			}
		})
	}
}

func TestPsqlEnvCarriesConnectionAndOverridesInherited(t *testing.T) {
	cfg, err := pgx.ParseConfig("postgres://cms:s3cret@db.example:6543/cmsdb?sslmode=disable")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	env := psqlEnv([]string{"PATH=/bin", "PGHOST=stale"}, cfg)

	got := map[string]string{} // last value wins, like os/exec
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		got[k] = v
	}
	want := map[string]string{
		"PATH": "/bin", "PGHOST": "db.example", "PGPORT": "6543", "PGUSER": "cms",
		"PGPASSWORD": "s3cret", "PGDATABASE": "cmsdb", "PGSSLMODE": "disable",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q; want %q", k, got[k], v)
		}
	}
}

func TestReadMigrationsFiltersAndSorts(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"010_b.sql", "002_a.sql", "README.md", "02_short.sql", "003_x.sql.bak"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("SELECT 1;\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, "archives"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "archives", "001_old.sql"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	files, err := readMigrations(dir)
	if err != nil {
		t.Fatalf("readMigrations: %v", err)
	}
	var names []string
	for _, f := range files {
		names = append(names, f.Version)
	}
	if strings.Join(names, ",") != "002_a.sql,010_b.sql" {
		t.Fatalf("files = %v; want [002_a.sql 010_b.sql]", names)
	}
	if files[0].Checksum != files[1].Checksum || len(files[0].Checksum) != 64 {
		t.Errorf("checksum = %q; want 64 hex chars, equal for equal content", files[0].Checksum)
	}
}

func TestRunRejectsBadArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"no command", nil, "usage"},
		{"unknown command", []string{"down"}, "usage"},
		{"baseline without through", []string{"baseline"}, "usage"},
		{"baseline bad through", []string{"baseline", "--through", "29"}, "usage"},
		{"baseline extra arg", []string{"baseline", "--through", "029", "x"}, "usage"},
		{"no database url", []string{"baseline", "--through", "029"}, "DATABASE_URL not set"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out strings.Builder
			err := run(context.Background(), tt.args, "", &out, &out)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v; want containing %q", err, tt.want)
			}
		})
	}
}

func TestRunParseErrorDoesNotLeakPassword(t *testing.T) {
	var out strings.Builder
	err := run(context.Background(), []string{"baseline", "--through", "001"},
		"postgres://u:topsecret@host:notaport/db", &out, &out)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if strings.Contains(err.Error(), "topsecret") || strings.Contains(out.String(), "topsecret") {
		t.Errorf("password leaked: %v / %s", err, out.String())
	}
}
