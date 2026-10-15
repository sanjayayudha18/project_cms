//go:build integration

package service

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// be-tech-debt C4.3: vendor order guards not covered by vendor_order_integration_test.go.
func TestIntegration_VendorOrder_DecisionGuards(t *testing.T) {
	f := newVendorOrderFixture(t)
	ctx := context.Background()
	vaultParty := f.party(t, f.cash2, partyRoleVault)

	t.Run("reject reason over 500 chars", func(t *testing.T) {
		var ve *ValidationError
		if _, err := f.orders.Reject(ctx, f.cash2User, vaultParty, strings.Repeat("x", 501)); !errors.As(err, &ve) || ve.Field != "reason" {
			t.Fatalf("err = %v, want ValidationError on reason", err)
		}
	})

	t.Run("vendor role without a users row is not authorized", func(t *testing.T) {
		ghost := f.cash2User
		ghost.UserID = 9_000_000_000
		if _, err := f.orders.Accept(ctx, ghost, vaultParty); !errors.Is(err, ErrNotAuthorized) {
			t.Fatalf("err = %v, want ErrNotAuthorized", err)
		}
	})

	t.Run("no decision on a cancelled request", func(t *testing.T) {
		if _, err := f.pool.Exec(ctx, `UPDATE vendor_requests SET is_canceled = true WHERE id = $1`, f.reqID); err != nil {
			t.Fatalf("cancel: %v", err)
		}
		if _, err := f.orders.Accept(ctx, f.cash2User, vaultParty); !errors.Is(err, ErrInvalidTransition) {
			t.Fatalf("accept: err = %v, want ErrInvalidTransition", err)
		}
		if _, err := f.orders.Reject(ctx, f.cash2User, vaultParty, "vault sedang diaudit"); !errors.Is(err, ErrInvalidTransition) {
			t.Fatalf("reject: err = %v, want ErrInvalidTransition", err)
		}
		var status string
		if err := f.pool.QueryRow(ctx, `SELECT status FROM vendor_request_vendor_parties WHERE id = $1`, vaultParty).Scan(&status); err != nil || status != "pending" {
			t.Fatalf("party status = %q err = %v, want pending", status, err)
		}
	})
}
