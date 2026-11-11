// Command extclient is the ClientRegistry CLI for the external user API
// (spec ext-user-api, Req 1, 9.2, 10.4). An operator runs it inside the
// backend-extapi container:
//
//	extclient create --name IDM --scopes users:read,users:write \
//	    --roles ATM-USER,ATM-SPV --cidrs 10.0.0.10/32 --actor <appaccess-user>
//
// Every write and its audit_logs rows run in one transaction, audited as the
// --actor (an active APPACCESS user). A client secret is printed to stdout
// exactly once and never logged, stored, or put in an audit payload.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/cimb-niaga/cms/backend-extapi/internal/config"
	"github.com/cimb-niaga/cms/backend-extapi/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

const usage = `usage: extclient <command> [flags]

commands:
  create     --name --scopes [--roles] [--cidrs] --actor
  rotate     --client-id --actor
  disable    --client-id --actor
  enable     --client-id --actor
  set-grants --client-id [--scopes] [--roles] [--cidrs] --actor
  list

Lists (--scopes, --roles, --cidrs) are comma-separated. Needs DATABASE_URL.
`

// errUsage marks an error caused by bad arguments (exit code 2).
var errUsage = errors.New("usage")

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := run(ctx, os.Args[1:], os.Getenv, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// cli is what one command needs: output streams and a lazy DB connection, so
// flags are validated (and --help works) before any connection is opened.
type cli struct {
	stdout, stderr io.Writer
	getenv         func(string) string
	pool           *pgxpool.Pool
}

func (c *cli) db(ctx context.Context) (*pgxpool.Pool, error) {
	if c.pool != nil {
		return c.pool, nil
	}
	cfg, err := config.LoadCLI(c.getenv)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		// Never print err: a parse error can echo the connection string.
		return nil, errors.New("cannot open database pool (check DATABASE_URL)")
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("cannot reach database: %w", err)
	}
	c.pool = pool
	return pool, nil
}

// inTx runs fn in one primary-pool transaction.
func (c *cli) inTx(ctx context.Context, fn func(tx repository.DBTX) error) error {
	pool, err := c.db(ctx)
	if err != nil {
		return err
	}
	return repository.NewTxRunner(pool).Run(ctx, fn)
}

var commands = map[string]func(context.Context, *cli, []string) error{
	"create":     cmdCreate,
	"rotate":     cmdRotate,
	"disable":    func(ctx context.Context, c *cli, a []string) error { return cmdSetActive(ctx, c, a, false) },
	"enable":     func(ctx context.Context, c *cli, a []string) error { return cmdSetActive(ctx, c, a, true) },
	"set-grants": cmdSetGrants,
	"list":       cmdList,
}

// run returns the process exit code: 0 ok, 1 failure, 2 bad usage.
func run(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprint(stderr, usage)
		return 2
	}
	if args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		_, _ = fmt.Fprint(stdout, usage)
		return 0
	}
	cmd, ok := commands[args[0]]
	if !ok {
		_, _ = fmt.Fprintf(stderr, "extclient: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
	c := &cli{stdout: stdout, stderr: stderr, getenv: getenv}
	defer func() {
		if c.pool != nil {
			c.pool.Close()
		}
	}()
	err := cmd(ctx, c, args[1:])
	switch {
	case err == nil, errors.Is(err, flag.ErrHelp):
		return 0
	case errors.Is(err, errUsage):
		_, _ = fmt.Fprintf(stderr, "extclient %s: %v\n", args[0], err)
		return 2
	default:
		_, _ = fmt.Fprintf(stderr, "extclient %s: %v\n", args[0], err)
		return 1
	}
}

// flagSet returns a FlagSet whose parse errors become errUsage in parse.
func flagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet("extclient "+name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

// parse parses args, rejects positional arguments, and checks that every
// required flag was given a non-blank value.
func parse(fs *flag.FlagSet, args []string, required ...string) error {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("%w: unexpected argument %q", errUsage, fs.Arg(0))
	}
	for _, n := range required {
		if strings.TrimSpace(fs.Lookup(n).Value.String()) == "" {
			return fmt.Errorf("%w: --%s is required", errUsage, n)
		}
	}
	return nil
}

// passed reports which flags were given on the command line.
func passed(fs *flag.FlagSet) map[string]bool {
	m := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { m[f.Name] = true })
	return m
}
