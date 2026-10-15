//go:build integration

package service

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// replenishment-realisasi S3 on real Postgres: holidays admin (FR1) -- one active row per date,
// enable refused on a taken date, audit written with every change, import skips taken dates, ADMIN
// only. Uses dates in 2098 and removes its rows afterwards.
func TestIntegration_HolidayAdmin(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var admin int64
	if err := pool.QueryRow(ctx, `SELECT u.id FROM users u JOIN roles r ON r.id = u.role_id WHERE r.role = 'ADMIN' AND u.is_active ORDER BY u.id LIMIT 1`).Scan(&admin); err != nil {
		t.Skipf("need an active ADMIN user: %v", err)
	}
	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM audit_logs WHERE entity_type = 'holiday' AND actor_id = $1 AND created_at > now() - interval '10 minutes'
			AND (entity_id IN (SELECT id FROM holidays WHERE EXTRACT(YEAR FROM holiday_date) = 2098) OR action = 'holiday_imported')`, admin)
		_, _ = pool.Exec(c, `DELETE FROM holidays WHERE EXTRACT(YEAR FROM holiday_date) = 2098`)
	})
	svc := NewHolidayAdminService(pool, pool)
	a := Actor{UserID: admin, Role: "ADMIN"}
	d := time.Date(2098, 12, 25, 0, 0, 0, 0, time.UTC)

	if _, err := svc.Create(ctx, Actor{UserID: admin, Role: "ATM-SPV"}, d, "Natal"); !errors.Is(err, ErrNotAuthorized) {
		t.Fatalf("ATM-SPV create: want ErrNotAuthorized, got %v", err)
	}
	h, err := svc.Create(ctx, a, d, "  Natal  ")
	if err != nil || h.Name != "Natal" || !h.IsActive {
		t.Fatalf("create = %+v, %v", h, err)
	}
	if _, err := svc.Create(ctx, a, d, "Natal lagi"); !errors.Is(err, ErrHolidayDateConflict) {
		t.Fatalf("duplicate active date: want ErrHolidayDateConflict, got %v", err)
	}
	if _, err := svc.SetActive(ctx, a, h.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetActive(ctx, a, h.ID, false); !errors.Is(err, ErrHolidayStatusUnchanged) {
		t.Fatalf("disable twice: want ErrHolidayStatusUnchanged, got %v", err)
	}
	// the date is free again: a new row may take it, and the old one can no longer be enabled.
	h2, err := svc.Create(ctx, a, d, "Natal (ganti)")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetActive(ctx, a, h.ID, true); !errors.Is(err, ErrHolidayDateConflict) {
		t.Fatalf("enable on a taken date: want ErrHolidayDateConflict, got %v", err)
	}
	if v, err := svc.UpdateName(ctx, a, h2.ID, "Hari Raya Natal"); err != nil || v.Name != "Hari Raya Natal" {
		t.Fatalf("update = %+v, %v", v, err)
	}
	if _, err := svc.UpdateName(ctx, a, -1, "x"); !errors.Is(err, ErrHolidayNotFound) {
		t.Fatalf("update missing: want ErrHolidayNotFound, got %v", err)
	}

	res, err := svc.Import(ctx, a, strings.NewReader("holiday_date,name\n2098-12-25,Natal\n2098-12-26,Cuti bersama\n"))
	if err != nil || res.Created != 1 || len(res.Skipped) != 1 || res.Skipped[0] != "2098-12-25" {
		t.Fatalf("import = %+v, %v", res, err)
	}
	var ve *ValidationError
	if _, err := svc.Import(ctx, a, strings.NewReader("holiday_date,name\n2098-12-27,A\nbad,B\n")); !errors.As(err, &ve) {
		t.Fatalf("bad file: want ValidationError, got %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM holidays WHERE holiday_date = '2098-12-27'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("a rejected file must add nothing, got %d rows", n)
	}

	list, err := svc.List(ctx, a, ptrInt32(2098), "active", 1, 50)
	if err != nil || list.Total != 2 {
		t.Fatalf("active 2098 = %+v, %v; want 2", list, err)
	}
	var audits int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE entity_type = 'holiday' AND actor_id = $1
		AND created_at > now() - interval '10 minutes'`, admin).Scan(&audits); err != nil || audits != 5 {
		t.Fatalf("audit rows = %d, %v; want one per successful change (5)", audits, err)
	}
}

func ptrInt32(v int32) *int32 { return &v }
