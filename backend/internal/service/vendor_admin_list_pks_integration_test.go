//go:build integration

package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"pgregory.net/rapid"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/repository"
)

// Properties 6 and 7 (spec vendor-pks-type task 5.2). For random vendors with a PKS per active type:
//   - P6: the list filtered by (pks_type, pks_status) equals the unfiltered list narrowed to the status
//     shown for that type, and Count = rows across all pages.
//   - P7: PksSummaries gives one entry per active type ordered by code (belum_diisi + nil days when empty),
//     INTERNAL vendors get none and match no filter, an inactive type never becomes an entry.
//
// "today" is fixed through VendorAdminService.now. Runs in one rolled-back tx; skips without DATABASE_URL.

const vlpRenewalMonths = 9

var vlpAllStatuses = []string{"aktif", "need_renewal", "expired", "terminated", "belum_diisi"}

// vlpPks is one drawn PKS of a vendor for one type.
type vlpPks struct {
	has        bool
	eff, exp   time.Time
	terminated bool
	hasOld     bool // a replaced (history) PKS of the same type must be ignored
}

type vlpSpec struct {
	internal    bool
	disabledVnd bool
	pks         []vlpPks // index = position in the active types ordered by code
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

// vlpStatus is the reference model of vendor_pks_status() for one type; "" = INTERNAL (no status).
func vlpStatus(internal bool, p vlpPks, today time.Time) string {
	switch {
	case internal:
		return ""
	case !p.has:
		return "belum_diisi"
	case p.terminated:
		return "terminated"
	case p.exp.Before(today):
		return "expired"
	case p.exp.Before(vlpAddMonths(today, vlpRenewalMonths)):
		return "need_renewal"
	}
	return "aktif"
}

func vlpDrawPks(rt *rapid.T, today time.Time, label string) vlpPks {
	p := vlpPks{
		has:        rapid.IntRange(0, 5).Draw(rt, label+"has") != 0,
		terminated: rapid.IntRange(0, 4).Draw(rt, label+"term") == 0,
		hasOld:     rapid.Bool().Draw(rt, label+"old"),
	}
	// Expiry biased around today and today+N months so every status shows up.
	switch rapid.IntRange(0, 2).Draw(rt, label+"mode") {
	case 0:
		p.exp = today.AddDate(0, 0, rapid.IntRange(-730, 1095).Draw(rt, label+"exp"))
	case 1:
		p.exp = today.AddDate(0, 0, rapid.IntRange(-2, 2).Draw(rt, label+"dt"))
	default:
		p.exp = vlpAddMonths(today, vlpRenewalMonths).AddDate(0, 0, rapid.IntRange(-2, 2).Draw(rt, label+"dr"))
	}
	p.eff = p.exp.AddDate(0, 0, -rapid.IntRange(0, 900).Draw(rt, label+"len"))
	return p
}

type vlpType struct {
	id   int64
	code string
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

	// Active types (ordered by code) are what the list may show; an inactive one must never appear.
	var inactiveID int64
	inactiveCode := fmt.Sprintf("ZI%d", time.Now().UnixNano()%10_000_000)
	if err := tx.QueryRow(ctx, `INSERT INTO vendor_pks_types (code, name, is_active) VALUES ($1, $1, false) RETURNING id`,
		inactiveCode).Scan(&inactiveID); err != nil {
		t.Fatalf("insert inactive type: %v", err)
	}
	var types []vlpType
	trows, err := tx.Query(ctx, `SELECT id, code FROM vendor_pks_types WHERE is_active ORDER BY code`)
	if err != nil {
		t.Fatalf("load types: %v", err)
	}
	for trows.Next() {
		var ty vlpType
		if err := trows.Scan(&ty.id, &ty.code); err != nil {
			t.Fatalf("scan type: %v", err)
		}
		types = append(types, ty)
	}
	trows.Close()
	if len(types) < 2 {
		t.Fatalf("need >= 2 active pks types (FLM, CIT seeded by 043), got %d", len(types))
	}

	wib := time.FixedZone("WIB", 7*3600)
	today := time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
	svc := NewVendorAdminService(repository.NewVendorAdminRepository(tx, tx), nil, WithPksRenewalMonths(vlpRenewalMonths))
	svc.now = func() time.Time { return time.Date(2026, 11, 6, 12, 0, 0, 0, wib) }

	// Type filter validation: unknown and inactive type are rejected, active resolves (case-insensitive).
	if _, err := svc.ResolvePksFilterType(ctx, "NOPE"); !errors.Is(err, ErrPksFilterType) {
		t.Fatalf("unknown type: err = %v, want ErrPksFilterType", err)
	}
	if _, err := svc.ResolvePksFilterType(ctx, inactiveCode); !errors.Is(err, ErrPksFilterType) {
		t.Fatalf("inactive type: err = %v, want ErrPksFilterType", err)
	}
	if id, err := svc.ResolvePksFilterType(ctx, " "+types[0].code+" "); err != nil || id != types[0].id {
		t.Fatalf("active type: id %d err %v, want %d", id, err, types[0].id)
	}

	runTag := time.Now().UnixNano()
	iter := 0

	rapid.Check(t, func(rt *rapid.T) {
		iter++
		tag := fmt.Sprintf("VLP%d_%d", runTag, iter)
		n := rapid.IntRange(1, 8).Draw(rt, "n")
		specs := make([]vlpSpec, n)
		ids := make([]int64, n)
		for i := range specs {
			s := vlpSpec{
				internal:    rapid.IntRange(0, 5).Draw(rt, fmt.Sprintf("internal%d", i)) == 0,
				disabledVnd: rapid.IntRange(0, 5).Draw(rt, fmt.Sprintf("dis%d", i)) == 0,
			}
			for k := range types {
				s.pks = append(s.pks, vlpDrawPks(rt, today, fmt.Sprintf("v%dt%d", i, k)))
			}
			specs[i] = s
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
			for k, p := range s.pks {
				wod := fmt.Sprintf("WOD-%s-%d-%d", tag, i, k)
				if p.hasOld {
					if _, err := tx.Exec(ctx, `INSERT INTO vendor_pks (vendor_id, pks_type_id, wod_number, pks_name, effective_date, expiry_date, created_by, replaced_at)
						VALUES ($1, $2, $3, 'PKS lama', '2000-01-01', '2000-12-31', $4, now())`,
						ids[i], types[k].id, wod+"-old", userID); err != nil {
						rt.Fatalf("insert old pks: %v", err)
					}
				}
				if p.has {
					if _, err := tx.Exec(ctx, `INSERT INTO vendor_pks (vendor_id, pks_type_id, wod_number, pks_name, effective_date, expiry_date, created_by,
						terminated_at, terminated_by, termination_reason)
						VALUES ($1, $2, $3, 'PKS aktif', $4, $5, $6,
						        CASE WHEN $7 THEN now() END, CASE WHEN $7 THEN $6::bigint END, CASE WHEN $7 THEN 'putus' END)`,
						ids[i], types[k].id, wod, p.eff, p.exp, userID, p.terminated); err != nil {
						rt.Fatalf("insert pks: %v", err)
					}
				}
			}
			// A PKS on the inactive type must never surface as an entry or a column.
			if _, err := tx.Exec(ctx, `INSERT INTO vendor_pks (vendor_id, pks_type_id, wod_number, pks_name, effective_date, expiry_date, created_by)
				VALUES ($1, $2, $3, 'PKS type nonaktif', '2026-01-01', '2027-12-31', $4)`,
				ids[i], inactiveID, fmt.Sprintf("WOD-%s-%d-inactive", tag, i), userID); err != nil {
				rt.Fatalf("insert pks on inactive type: %v", err)
			}
		}

		q := tag
		all := vlpPages(rt, ctx, svc, db.ListVendorsAdminParams{Q: &q, Status: "all"}, 3)
		if len(all) != n {
			rt.Fatalf("unfiltered rows = %d, want %d", len(all), n)
		}
		allIDs := make([]int64, n)
		for i, row := range all {
			if row.ID != ids[i] {
				rt.Fatalf("order: row %d id %d, want %d", i, row.ID, ids[i])
			}
			allIDs[i] = row.ID
		}

		// P7: summaries — one entry per active type ordered by code; INTERNAL none.
		sums, err := svc.PksSummaries(ctx, allIDs)
		if err != nil {
			rt.Fatalf("PksSummaries: %v", err)
		}
		for i, s := range specs {
			got := sums[ids[i]]
			if s.internal {
				if len(got) != 0 {
					rt.Fatalf("vendor %d INTERNAL: %d entries, want 0", i, len(got))
				}
				continue
			}
			if len(got) != len(types) {
				rt.Fatalf("vendor %d: %d entries, want %d (one per active type): %+v", i, len(got), len(types), got)
			}
			if !sort.SliceIsSorted(got, func(a, b int) bool { return got[a].PksType < got[b].PksType }) {
				rt.Fatalf("vendor %d: entries not ordered by type code: %+v", i, got)
			}
			for k, e := range got {
				p := s.pks[k]
				if e.PksType != types[k].code {
					rt.Fatalf("vendor %d entry %d: type %q, want %q (inactive type %q must not appear)", i, k, e.PksType, types[k].code, inactiveCode)
				}
				want := vlpStatus(false, p, today)
				if e.PksStatus != want {
					rt.Fatalf("vendor %d type %s (%+v): status %q, want %q", i, e.PksType, p, e.PksStatus, want)
				}
				if !p.has {
					if e.RemainingDays != nil {
						rt.Fatalf("vendor %d type %s: belum_diisi must have nil remaining_days, got %d", i, e.PksType, *e.RemainingDays)
					}
				} else if wantDays := int32(p.exp.Sub(today).Hours() / 24); e.RemainingDays == nil || *e.RemainingDays != wantDays {
					rt.Fatalf("vendor %d type %s: remaining_days %v, want %d", i, e.PksType, e.RemainingDays, wantDays)
				}
			}
		}

		// P6: for each (type, status), filter = unfiltered narrowed to the status shown for that type.
		for k, ty := range types {
			tid := ty.id
			matched := 0
			for _, st := range vlpAllStatuses {
				st := st
				var want []int64
				for i := range specs {
					if len(sums[ids[i]]) > 0 && sums[ids[i]][k].PksStatus == st {
						want = append(want, ids[i])
					}
				}
				got := vlpPages(rt, ctx, svc, db.ListVendorsAdminParams{Q: &q, Status: "all", PksTypeID: &tid, PksStatus: &st}, 2)
				gotIDs := make([]int64, len(got))
				for i, r := range got {
					gotIDs[i] = r.ID
				}
				if fmt.Sprint(gotIDs) != fmt.Sprint(want) {
					rt.Fatalf("filter %s/%s: ids %v, want %v", ty.code, st, gotIDs, want)
				}
				total, err := svc.Count(ctx, db.CountVendorsAdminParams{Q: &q, Status: "all", PksTypeID: &tid, PksStatus: &st})
				if err != nil {
					rt.Fatalf("count: %v", err)
				}
				if int(total) != len(want) {
					rt.Fatalf("filter %s/%s: total %d, want %d", ty.code, st, total, len(want))
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
				rt.Fatalf("type %s: filters matched %d vendors, want %d (INTERNAL must match none)", ty.code, matched, n-internal)
			}
		}

		// kind filter, q-by-name and disabled-vendor visibility keep working with the type join in place.
		kindTotal := 0
		for _, kd := range []string{"FLM_VENDOR", "INTERNAL"} {
			kd := kd
			var want []int64
			for i, s := range specs {
				if s.internal == (kd == "INTERNAL") {
					want = append(want, ids[i])
				}
			}
			got := vlpPages(rt, ctx, svc, db.ListVendorsAdminParams{Q: &q, Status: "all", Kind: &kd}, 50)
			gotIDs := make([]int64, len(got))
			for i, r := range got {
				gotIDs[i] = r.ID
			}
			if fmt.Sprint(gotIDs) != fmt.Sprint(want) {
				rt.Fatalf("kind %s: ids %v, want %v", kd, gotIDs, want)
			}
			total, err := svc.Count(ctx, db.CountVendorsAdminParams{Q: &q, Status: "all", Kind: &kd})
			if err != nil || int(total) != len(want) {
				rt.Fatalf("kind %s: total %d err %v, want %d", kd, total, err, len(want))
			}
			kindTotal += len(want)
		}
		if kindTotal != n {
			rt.Fatalf("kinds cover %d vendors, want %d", kindTotal, n)
		}
		one := fmt.Sprintf("%s-%02d", tag, 0)
		byName, err := svc.List(ctx, db.ListVendorsAdminParams{Q: &one, Status: "all", PageLimit: 10})
		if err != nil || len(byName) != 1 || byName[0].ID != ids[0] {
			rt.Fatalf("q=%s: rows %d err %v, want vendor %d only", one, len(byName), err, ids[0])
		}
	})
}

// vlpPages walks every page (size ps) of the list for the given filters.
func vlpPages(rt *rapid.T, ctx context.Context, svc *VendorAdminService, base db.ListVendorsAdminParams, ps int64) []db.ListVendorsAdminRow {
	var out []db.ListVendorsAdminRow
	for off := int64(0); ; off += ps {
		arg := base
		arg.PageLimit, arg.PageOffset = ps, off
		rows, err := svc.List(ctx, arg)
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
