package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"pgregory.net/rapid"
)

// Feature: replenishment-request-enhancements, Property 5: Validasi alasan (reject dan cancel)
//
// Pure, no DB: Reject and Cancel both validate their reason argument (trim,
// 1-500 non-whitespace chars) before ever touching the pool -- BeginTx is the
// first pool call either makes. reasonProbePool's BeginTx returns a sentinel
// error instead of opening a real transaction, so a reason that clears
// validation surfaces errBeginTxProbe (proving validation let it through and
// the service tried to proceed), while every other DBTX method panics:
// neither Reject nor Cancel should ever reach them when the reason itself is
// what's being decided on.

var errBeginTxProbe = errors.New("reasonProbePool: begin tx reached (validation passed)")

type reasonProbePool struct{}

func (reasonProbePool) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	panic("reasonProbePool.Exec: should not be reached by reason validation alone")
}
func (reasonProbePool) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	panic("reasonProbePool.Query: should not be reached by reason validation alone")
}
func (reasonProbePool) QueryRow(context.Context, string, ...interface{}) pgx.Row {
	panic("reasonProbePool.QueryRow: should not be reached by reason validation alone")
}
func (reasonProbePool) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return nil, errBeginTxProbe
}

// reasonGen mixes general random strings with explicit whitespace-only and
// boundary-length (500/501) cases so the property actually exercises the
// edges, not just whatever anyRuneGen happens to roll.
func reasonGen() *rapid.Generator[string] {
	return rapid.OneOf(
		rapid.StringN(0, 600, -1),
		rapid.Custom(func(rt *rapid.T) string {
			n := rapid.IntRange(0, 10).Draw(rt, "wsLen")
			return strings.Repeat(" ", n)
		}),
		rapid.Custom(func(rt *rapid.T) string {
			n := rapid.SampledFrom([]int{1, 500, 501, 502}).Draw(rt, "boundaryLen")
			return strings.Repeat("a", n)
		}),
	)
}

func TestProperty5_ReasonValidation(t *testing.T) {
	svc := NewVendorRequestService(reasonProbePool{})
	ctx := context.Background()
	actor := Actor{UserID: 1, Role: "ATM-SPV"}

	cases := []struct {
		name          string
		invoke        func(reason string) error
		emptySentinel error
		field         string
	}{
		{"Reject", func(reason string) error { _, err := svc.Reject(ctx, actor, 1, reason); return err }, ErrRejectReasonEmpty, "rejection_reason"},
		{"Cancel", func(reason string) error { _, err := svc.Cancel(ctx, actor, 1, reason); return err }, ErrCancelReasonEmpty, "cancellation_reason"},
	}

	rapid.Check(t, func(rt *rapid.T) {
		reason := reasonGen().Draw(rt, "reason")
		trimmedLen := len(strings.TrimSpace(reason))
		wantValid := trimmedLen >= 1 && trimmedLen <= 500

		for _, c := range cases {
			err := c.invoke(reason)
			switch {
			case wantValid:
				if !errors.Is(err, errBeginTxProbe) {
					rt.Fatalf("%s(%q): want validation to pass through to BeginTx, got %v", c.name, reason, err)
				}
			case trimmedLen == 0:
				if !errors.Is(err, c.emptySentinel) {
					rt.Fatalf("%s(%q): want %v for empty/whitespace-only reason, got %v", c.name, reason, c.emptySentinel, err)
				}
			default: // trimmedLen > 500
				var verr *ValidationError
				if !errors.As(err, &verr) || verr.Field != c.field {
					rt.Fatalf("%s(%q): want *ValidationError{Field:%q} for >500 chars, got %v", c.name, reason, c.field, err)
				}
			}
		}
	})
}
