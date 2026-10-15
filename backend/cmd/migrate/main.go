// Command migrate applies backend/migrations/NNN_*.sql with tracking in schema_migrations.
//
//	migrate [-dir migrations] status
//	migrate [-dir migrations] up
//	migrate [-dir migrations] baseline --through NNN
//
// The database comes from DATABASE_URL. Files run through psql (must be on PATH), one at a time,
// under an advisory lock. Run from backend/ on the operator host, outside 07:00–20:00 WIB.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strconv"

	"github.com/jackc/pgx/v5"
)

const usage = "usage: migrate [-dir DIR] status | up | baseline --through NNN"

var throughRe = regexp.MustCompile(`^\d{3}$`)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	err := run(ctx, os.Args[1:], os.Getenv("DATABASE_URL"), os.Stdout, os.Stderr)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, dbURL string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "migrations", "directory holding NNN_*.sql files")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return errors.New(usage)
	}
	cmd, rest := fs.Arg(0), fs.Args()[1:]

	through := -1
	switch cmd {
	case "status", "up":
		if len(rest) > 0 {
			return errors.New(usage)
		}
		if _, err := exec.LookPath("psql"); err != nil {
			return errors.New("psql not found on PATH (migrations are applied with psql -f)")
		}
	case "baseline":
		bfs := flag.NewFlagSet("baseline", flag.ContinueOnError)
		bfs.SetOutput(stderr)
		t := bfs.String("through", "", "mark files numbered <= NNN as applied without running them")
		if err := bfs.Parse(rest); err != nil {
			return err
		}
		if bfs.NArg() > 0 || !throughRe.MatchString(*t) {
			return errors.New(usage)
		}
		through, _ = strconv.Atoi(*t)
	default:
		return errors.New(usage)
	}

	if dbURL == "" {
		return errors.New("DATABASE_URL not set")
	}
	cfg, err := pgx.ParseConfig(dbURL)
	if err != nil {
		return fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	r, err := newRunner(ctx, cfg, *dir, stdout, stderr)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close(context.Background()) }()

	switch cmd {
	case "status":
		return r.Status(ctx)
	case "up":
		return r.Up(ctx)
	default:
		return r.Baseline(ctx, through)
	}
}
