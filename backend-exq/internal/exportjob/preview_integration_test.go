//go:build integration

package exportjob

import (
	"context"
	"errors"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/cimb-niaga/cms/backend-exq/internal/repository"
)

// Task 3.3 on real Postgres. Reuses the p11 fixture (preview of the same tagged rows the
// export tests use; created and removed by newP11Fix, dev data is only read).

func p33Previewer(f *p11Fix, producers map[Kind]map[string]Producer) *Previewer {
	return &Previewer{scope: repository.New(f.pool), reader: pgReader{pool: f.pool}, producers: producers, now: time.Now}
}

var p33Admin = Actor{UserID: 1, Username: "p33", Role: "ADMIN"}

// p33Cases lists every scope the UI can ask for, per kind and entity.
func p33Cases() []struct {
	kind Kind
	q    p11Q
} {
	ip := func(i int) *int { return &i }
	prods := DefaultProducers()
	entities := func(k Kind) []string {
		var out []string
		for e := range prods[k] {
			out = append(out, e)
		}
		sort.Strings(out)
		return out
	}
	var out []struct {
		kind Kind
		q    p11Q
	}
	add := func(k Kind, q p11Q) {
		out = append(out, struct {
			kind Kind
			q    p11Q
		}{k, q})
	}
	for _, e := range entities(KindMasterDataGlobal) {
		add(KindMasterDataGlobal, p11Q{entity: e})
	}
	for _, e := range entities(KindVendor) {
		for v := range 2 {
			add(KindVendor, p11Q{entity: e, vendor: ip(v)})
		}
	}
	for _, e := range entities(KindCabang) {
		for _, vb := range [][2]int{{0, 0}, {0, 1}, {1, 2}, {1, 0}} { // {1,0}: cabang of another vendor
			add(KindCabang, p11Q{entity: e, vendor: ip(vb[0]), branch: ip(vb[1])})
		}
	}
	for v := range 2 {
		branches := []*int{nil, ip(2)}
		if v == 0 {
			branches = []*int{nil, ip(0), ip(1)}
		}
		for _, b := range branches {
			for _, lvl := range []string{"vendor", "branch", "atm", "all"} {
				add(KindHargaPaket, p11Q{entity: "package-prices", vendor: ip(v), branch: b, level: lvl})
			}
		}
	}
	return out
}

// Property 2: Count == number of rows Produce emits, in one snapshot.
func TestP33CountEqualsProducedRows(t *testing.T) {
	f := newP11Fix(t)
	ctx := context.Background()
	asOfs := []time.Time{time.Now(), time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)}
	entities := map[string]bool{}
	combos := 0
	for _, c := range p33Cases() {
		for _, st := range []string{"active", "disabled", "all"} {
			for _, asOf := range asOfs {
				q := c.q
				q.status, q.asOf = st, asOf
				var p Params
				if c.kind == KindMasterDataGlobal {
					var err error
					if p, err = ParseParams(c.kind, q.entity, nil, nil, st, ""); err != nil {
						t.Fatal(err)
					}
				} else {
					p = p11Params(t, c.kind, q, f)
				}
				pp, ok := DefaultProducers()[c.kind][q.entity].(PreviewProducer)
				if !ok {
					t.Fatalf("%s/%s is not a PreviewProducer", c.kind, q.entity)
				}
				snap, rollback, err := pgReader{pool: f.pool}.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				total, cerr := pp.Count(ctx, snap, p, asOf)
				var produced int64
				perr := pp.Produce(ctx, snap, p, asOf, func([]string) error { produced++; return nil })
				rollback()
				if cerr != nil || perr != nil {
					t.Fatalf("%s/%s %+v: count err=%v produce err=%v", c.kind, q.entity, q, cerr, perr)
				}
				if total != produced {
					t.Fatalf("%s/%s status=%s level=%q v=%v b=%v: Count=%d, Produce emitted %d",
						c.kind, q.entity, st, q.level, q.vendor, q.branch, total, produced)
				}
				if c.kind != KindMasterDataGlobal { // scoped to fixture rows: the model must agree too
					want := len(f.expect(q, c.kind == KindVendor && q.entity == "vendor"))
					if int64(want) != total {
						t.Errorf("%s/%s status=%s level=%q v=%v b=%v: Count=%d, model=%d",
							c.kind, q.entity, st, q.level, q.vendor, q.branch, total, want)
					}
				}
				entities[q.entity] = true
				combos++
			}
		}
	}
	t.Logf("Property 2: %d combinations (kind x entity x scope x status x level x asOf), %d distinct entity names", combos, len(entities))
}

func p33PicIDs(t *testing.T, f *p11Fix, kind Kind, v int, b *int) []int64 {
	t.Helper()
	pv := new(Previewer)
	*pv = *p33Previewer(f, DefaultProducers())
	q := p11Q{entity: "pics", vendor: &v, branch: b, status: "all"}
	got, err := pv.Preview(context.Background(), p33Admin, p11Params(t, kind, q, f), 0, MaxPreviewLimit)
	if err != nil {
		t.Fatal(err)
	}
	if got.NextAfter != nil || got.Total != int64(len(got.Rows)) {
		t.Fatalf("fixture PICs should fit one page: total=%d rows=%d next=%v", got.Total, len(got.Rows), got.NextAfter)
	}
	ids := p11CellIDs(t, got.Rows)
	slices.Sort(ids)
	return ids
}

// D3: cabang scope shows only that cabang's PICs; vendor scope shows vendor-level + every cabang's.
func TestP33PicScopes(t *testing.T) {
	f := newP11Fix(t)
	for v := range 2 {
		var want []int64
		for _, r := range f.rows["pics"] {
			if r.vendor == v {
				want = append(want, r.id)
			}
		}
		slices.Sort(want)
		if got := p33PicIDs(t, f, KindVendor, v, nil); !slices.Equal(got, want) {
			t.Fatalf("vendor %d scope: PIC ids %v, want vendor-level + all cabang PICs %v", v, got, want)
		}
	}
	for _, vb := range [][2]int{{0, 0}, {0, 1}, {1, 2}} {
		var want []int64
		for _, r := range f.rows["pics"] {
			if r.vendor == vb[0] && r.branch == vb[1] {
				want = append(want, r.id)
			}
		}
		slices.Sort(want)
		b := vb[1]
		if got := p33PicIDs(t, f, KindCabang, vb[0], &b); !slices.Equal(got, want) {
			t.Fatalf("cabang %v scope: PIC ids %v, want only that cabang's %v", vb, got, want)
		}
	}
	// A cabang of another vendor is rejected on the primary lookup, no rows leak.
	v, b := 1, 0
	q := p11Q{entity: "pics", vendor: &v, branch: &b, status: "all"}
	_, err := p33Previewer(f, DefaultProducers()).Preview(context.Background(), p33Admin, p11Params(t, KindCabang, q, f), 0, 10)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("cabang of another vendor: err = %v, want ErrNotFound", err)
	}
}

// Following next_after through the real Previewer yields exactly the rows Produce emits.
func TestP33PagingWalkMatchesProduce(t *testing.T) {
	f := newP11Fix(t)
	ctx := context.Background()
	v := 0
	for _, ent := range []string{"pics", "vaults", "branches"} {
		p := p11Params(t, KindVendor, p11Q{entity: ent, vendor: &v, status: "all"}, f)
		rows, err := p11Produce(ctx, f.q, p, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		want := p11CellIDs(t, rows)
		for limit := 1; limit <= 4; limit++ {
			var got []int64
			after, pages := int64(0), 0
			for {
				pg, err := p33Previewer(f, DefaultProducers()).Preview(ctx, p33Admin, p, after, limit)
				if err != nil {
					t.Fatal(err)
				}
				if pg.Total != int64(len(want)) {
					t.Fatalf("%s limit %d: total %d, want %d", ent, limit, pg.Total, len(want))
				}
				got = append(got, p11CellIDs(t, pg.Rows)...)
				if pg.NextAfter == nil {
					break
				}
				after = *pg.NextAfter
				if pages++; pages > len(want)+1 {
					t.Fatalf("%s limit %d: next_after never ends", ent, limit)
				}
			}
			if !slices.Equal(got, want) {
				t.Fatalf("%s limit %d: walked ids %v, want %v", ent, limit, got, want)
			}
		}
	}
}

// hookPreview runs afterCount right after the real Count, i.e. between Count and Page.
type hookPreview struct {
	PreviewProducer
	afterCount func()
}

func (h hookPreview) Count(ctx context.Context, q Querier, p Params, asOf time.Time) (int64, error) {
	n, err := h.PreviewProducer.Count(ctx, q, p, asOf)
	if err == nil {
		h.afterCount()
	}
	return n, err
}

// Count and Page share one snapshot: a row committed by another connection in between is invisible.
func TestP33CountAndPageShareOneSnapshot(t *testing.T) {
	f := newP11Fix(t)
	ctx := context.Background()
	v := 0
	p := p11Params(t, KindVendor, p11Q{entity: "branches", vendor: &v, status: "all"}, f)
	base, err := p11Produce(ctx, f.q, p, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	lateCode := f.tag + "BMID"
	inner := DefaultProducers()[KindVendor]["branches"].(PreviewProducer)
	inserted := false
	hook := hookPreview{PreviewProducer: inner, afterCount: func() {
		f.id(`INSERT INTO vendor_branches (vendor_id, branch_code, branch_name) VALUES ($1, $2, $2) RETURNING id`, f.vIDs[0], lateCode)
		inserted = true
	}}
	producers := map[Kind]map[string]Producer{KindVendor: {"branches": hook}}

	got, err := p33Previewer(f, producers).Preview(ctx, p33Admin, p, 0, MaxPreviewLimit)
	if err != nil {
		t.Fatal(err)
	}
	if !inserted {
		t.Fatal("hook did not run")
	}
	if got.Total != int64(len(base)) || len(got.Rows) != len(base) {
		t.Fatalf("total=%d rows=%d, want both %d (late row must be invisible)", got.Total, len(got.Rows), len(base))
	}
	for _, r := range got.Rows {
		if r[2] == lateCode {
			t.Fatal("Page saw a row committed after Count's snapshot")
		}
	}
	// a fresh request sees it, so the row really was committed
	fresh, err := p33Previewer(f, DefaultProducers()).Preview(ctx, p33Admin, p, 0, MaxPreviewLimit)
	if err != nil || fresh.Total != int64(len(base))+1 {
		t.Fatalf("fresh preview: total=%d err=%v, want %d", fresh.Total, err, len(base)+1)
	}
}
