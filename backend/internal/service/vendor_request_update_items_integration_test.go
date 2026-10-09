//go:build integration

package service

import (
	"context"
	"errors"
	"testing"
)

// UpdateItems guards (Req 4): payload validation first, then the request must
// exist, still be draft, and only its creator may edit the items.
func TestIntegration_UpdateItems_Guards(t *testing.T) {
	pool, vendorID, createdBy, terminalID := setupNumberGeneratorHarness(t)
	seedDmaaRows(t, pool, terminalID, 50000)
	svc := NewVendorRequestService(pool)
	ctx := context.Background()
	actor := Actor{UserID: createdBy, Role: "ADMIN"}
	items := []ItemInput{{TerminalID: terminalID, PeriodePred: jakartaCalendarDate(0), Denom: 50000, AmountReplenish: 1000000}}

	created, err := svc.Create(ctx, actor, manualCreateInput(vendorID, terminalID, 50000))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := svc.UpdateItems(ctx, actor, created.ID, nil); !errors.Is(err, ErrEmptyItems) {
		t.Fatalf("empty items: want ErrEmptyItems, got %v", err)
	}
	if _, err := svc.UpdateItems(ctx, actor, -1, items); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing request: want ErrNotFound, got %v", err)
	}
	if _, err := svc.UpdateItems(ctx, Actor{UserID: createdBy + 1_000_000, Role: "ADMIN"}, created.ID, items); !errors.Is(err, ErrNotCreator) {
		t.Fatalf("other user: want ErrNotCreator, got %v", err)
	}
	if _, err := svc.Submit(ctx, actor, created.ID); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := svc.UpdateItems(ctx, actor, created.ID, items); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("not draft: want ErrInvalidTransition, got %v", err)
	}
}
