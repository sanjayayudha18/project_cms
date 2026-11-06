//go:build integration

package service

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"pgregory.net/rapid"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/repository"
)

// Property 4, list part (spec vendor-pks task 6.3): for random vendors with
// PKS + director, the vendors list with a pks_status filter equals the
// unfiltered list narrowed to that status; total = rows across all pages;
// INTERNAL vendors match no filter; q by name still works; a disabled
// director is not shown. "today" is fixed through VendorAdminService.now.
// Everything runs in one rolled-back tx. Skips when DATABASE_URL is empty.

const vlpRenewalMonths = 9

var vlpAllStatuses = []string{"aktif", "need_renewal", "expired", "terminated", "belum_diisi"}

type vlpSpec struct {
	internal    bool
	hasPks      bool
	eff, exp    time.Time
	terminated  bool
	hasOldPks   bool
	dirMode     int // 0 none, 1 active, 2 disabled
	disabledVnd bool
}

func vlpAddMonths(d time.Time, n int) time.Time {
	total := d.Year()*12 + int(d.Month()) - 1 + n
	y, m := total/12, time.Month(total%12+1)
	last := time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
	day := d.Day()
	if day > last {
		day = last
	}
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

// vlpStatus is the Req 6.2-6.6 reference model; "" = no status (INTERNAL).
func vlpStatus(s vlpSpec, today time.Time) string {
	switch {
	case s.internal:
		return ""
	case !s.hasPks:
		return "belum_diisi"
	case s.terminated:
		return "terminated"
	case s.exp.Before(today):
		return "expired"
	case s.exp.Before(vlpAddMonths(today, vlpRenewalMonths)):
		return "need_renewal"
	}
	return "aktif"
}

func vlpValidityMonths(eff, exp time.Time) int {
	end := exp.AddDate(0, 0, 1)
	n := 0
	for !vlpAddMonths(eff, n+1).After(end) {
		n++
	}
	return n
}

func vlpDrawSpec(rt *rapid.T, today time.Time, i int) vlpSpec {
	l := func(s string) string { return fmt.Sprintf("%s%d", s, i) }
	s := vlpSpec{
		internal:    rapid.IntRange(0, 5).Draw(rt, l("internal")) == 0,
		hasPks:      rapid.IntRange(0, 5).Draw(rt, l("haspks")) != 0,
		terminated:  rapid.IntRange(0, 4).Draw(rt, l("term")) == 0,
		hasOldPks:   rapid.Bool().Draw(rt, l("old")),
		dirMode:     rapid.IntRange(0, 2).Draw(rt, l("dir")),
		disabledVnd: rapid.IntRange(0, 5).Draw(rt, l("dis")) == 0,
	}
	// Expiry biased around today and today+N months so every status shows up.
	switch rapid.IntRange(0, 2).Draw(rt, l("mode")) {
	case 0:
		s.exp = today.AddDate(0, 0, rapid.IntRange(-730, 1095).Draw(rt, l("exp")))
	case 1:
		s.exp = today.AddDate(0, 0, rapid.IntRange(-2, 2).Draw(rt, l("dt")))
	default:
		s.exp = vlpAddMonths(today, vlpRenewalMonths).AddDate(0, 0, rapid.IntRange(-2, 2).Draw(rt, l("dr")))
	}
	s.eff = s.exp.AddDate(0, 0, -rapid.IntRange(0, 900).Draw(rt, l("len")))
	return s
}

func TestVendorAdminListPks_Property_Integration(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connecting to database: %v", err)
	}
	t.Cleanup(pool.Close)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	var userID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM users ORDER BY id LIMIT 1`).Scan(&userID); err != nil {
		t.Fatalf("load a user: %v", err)
	}

	wib := time.FixedZone("WIB", 7*3600)
	today := time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
	svc := NewVendorAdminService(repository.NewVendorAdminRepository(tx, tx), nil, WithPksRenewalMonths(vlpRenewalMonths))
	svc.now = func() time.Time { return time.Date(2026, 11, 6, 12, 0, 0, 0, wib) }

	runTag := time.Now().UnixNano()
	iter := 0

	rapid.Check(t, func(rt *rapid.T) {
		iter++
		tag := fmt.Sprintf("VLP%d_%d", runTag, iter)
		n := rapid.IntRange(1, 8).Draw(rt, "n")
		specs := make([]vlpSpec, n)
		ids := make([]int64, n)
		for i := range specs {
			specs[i] = vlpDrawSpec(rt, today, i)
			s := specs[i]
			kind := "FLM_VENDOR"
			if s.internal {
				kind = "INTERNAL"
			}
			code := fmt.Sprintf("%s-%02d", tag, i)
			if err := tx.QueryRow(ctx, `INSERT INTO vendors (code, name, kind, is_active, deleted_at)
				VALUES ($1, $1, $2, $3, CASE WHEN $3 THEN NULL ELSE now() END) RETURNING id`,
				code, kind, !s.disabledVnd).Scan(&ids[i]); err != nil {
				rt.Fatalf("insert vendor: %v", err)
			}
			if s.hasOldPks {
				if _, err := tx.Exec(ctx, `INSERT INTO vendor_pks (vendor_id, pks_name, effective_date, expiry_date, created_by, replaced_at)
					VALUES ($1, 'PKS lama', '2000-01-01', '2000-12-31', $2, now())`, ids[i], userID); err != nil {
					rt.Fatalf("insert old pks: %v", err)
				}
			}
			if s.hasPks {
				if _, err := tx.Exec(ctx, `INSERT INTO vendor_pks (vendor_id, pks_name, effective_date, expiry_date, created_by,
					terminated_at, terminated_by, termination_reason)
					VALUES ($1, 'PKS aktif', $2, $3, $4,
					        CASE WHEN $5 THEN now() END, CASE WHEN $5 THEN $4::bigint END, CASE WHEN $5 THEN 'putus' END)`,
					ids[i], s.eff, s.exp, userID, s.terminated); err != nil {
					rt.Fatalf("insert pks: %v", err)
				}
			}
			if s.dirMode != 0 {
				if _, err := tx.Exec(ctx, `INSERT INTO vendor_pics (vendor_id, name, is_director, roles, is_notification_recipient, is_active, deleted_at)
					VALUES ($1, $2, true, '{}', false, $3, CASE WHEN $3 THEN NULL ELSE now() END)`,
					ids[i], fmt.Sprintf("Direktur %d", i), s.dirMode == 1); err != nil {
					rt.Fatalf("insert director: %v", err)
				}
			}
		}

		q := tag
		all := vlpPages(rt, ctx, svc, &q, nil, 3)

		// Unfiltered rows match the reference model (status, months, days, director).
		if len(all) != n {
			rt.Fatalf("unfiltered rows = %d, want %d", len(all), n)
		}
		for i, row := range all {
			s := specs[i]
			if row.ID != ids[i] {
				rt.Fatalf("order: row %d id %d, want %d", i, row.ID, ids[i])
			}
			if want := vlpStatus(s, today); row.PksStatus != want {
				rt.Fatalf("vendor %d (%+v): pks_status %q, want %q", i, s, row.PksStatus, want)
			}
			wantDays := int(s.exp.Sub(today).Hours() / 24)
			if s.hasPks {
				if row.PksID == nil || int(row.PksValidityMonths) != vlpValidityMonths(s.eff, s.exp) || int(row.PksRemainingDays) != wantDays {
					rt.Fatalf("vendor %d: pks %v months %d days %d, want months %d days %d", i, row.PksID,
						row.PksValidityMonths, row.PksRemainingDays, vlpValidityMonths(s.eff, s.exp), wantDays)
				}
			} else if row.PksID != nil || row.PksValidityMonths != 0 || row.PksRemainingDays != 0 {
				rt.Fatalf("vendor %d: no active pks but pks_id=%v months=%d days=%d", i, row.PksID, row.PksValidityMonths, row.PksRemainingDays)
			}
			if s.dirMode == 1 {
				if row.DirectorName == nil || *row.DirectorName != fmt.Sprintf("Direktur %d", i) {
					rt.Fatalf("vendor %d: director %v, want active director", i, row.DirectorName)
				}
			} else if row.DirectorID != nil || row.DirectorName != nil {
				rt.Fatalf("vendor %d: director shown for dirMode %d", i, s.dirMode)
			}
		}

		// Each filter = unfiltered narrowed to that status; total = all pages.
		matched := 0
		for _, st := range vlpAllStatuses {
			st := st
			var want []int64
			for _, row := range all {
				if row.PksStatus == st {
					want = append(want, row.ID)
				}
			}
			got := vlpPages(rt, ctx, svc, &q, &st, 2)
			gotIDs := make([]int64, len(got))
			for i, r := range got {
				gotIDs[i] = r.ID
			}
			if fmt.Sprint(gotIDs) != fmt.Sprint(want) {
				rt.Fatalf("filter %s: ids %v, want %v", st, gotIDs, want)
			}
			total, err := svc.Count(ctx, db.CountVendorsAdminParams{Q: &q, Status: "all", PksStatus: &st})
			if err != nil {
				rt.Fatalf("count: %v", err)
			}
			if int(total) != len(want) {
				rt.Fatalf("filter %s: total %d, want %d", st, total, len(want))
			}
			matched += len(want)
		}
		internal := 0
		for _, s := range specs {
			if s.internal {
				internal++
			}
		}
		if matched != n-internal {
			rt.Fatalf("filters matched %d vendors, want %d (INTERNAL must match none)", matched, n-internal)
		}

		// q by name keeps working.
		one := fmt.Sprintf("%s-%02d", tag, 0)
		rows, err := svc.List(ctx, db.ListVendorsAdminParams{Q: &one, Status: "all", PageLimit: 10})
		if err != nil || len(rows) != 1 || rows[0].ID != ids[0] {
			rt.Fatalf("q=%s: rows %d err %v, want vendor %d only", one, len(rows), err, ids[0])
		}
	})
}

// vlpPages walks every page (size ps) of the list for q and the optional status.
func vlpPages(rt *rapid.T, ctx context.Context, svc *VendorAdminService, q, st *string, ps int64) []db.ListVendorsAdminRow {
	var out []db.ListVendorsAdminRow
	for off := int64(0); ; off += ps {
		rows, err := svc.List(ctx, db.ListVendorsAdminParams{Q: q, Status: "all", PksStatus: st, PageLimit: ps, PageOffset: off})
		if err != nil {
			rt.Fatalf("list: %v", err)
		}
		out = append(out, rows...)
		if int64(len(rows)) < ps {
			break
		}
	}
	return out
}
