package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/cimb-niaga/cms/backend/internal/db"
)

// fakeAssignment models one atm_vendor_packages row for the fake repo.
type fakeAssignment struct {
	id                 int64
	atmID              int64
	vendorPackageID    int64
	isActive           bool
	effectiveStartDate time.Time
	effectiveEndDate   *time.Time // nil = open-ended
}

// fakePackage models one vendor_packages_branch row.
type fakePackage struct {
	id             int64
	vendorBranchID int64
	packageCode    string
}

// fakeATM models one atms row with its LEFT JOIN locations columns already resolved.
type fakeATM struct {
	id                    int64
	terminalID            string
	priorityClass         *string
	isActive              bool
	locationName          *string
	locationCityOrRegency *string
}

// fakeBranchATMRepo is an in-memory BranchATMRepo replicating
// ListBranchATMs/CountBranchATMs's SQL semantics (active-assignment window
// filter, DISTINCT ON dedup + tiebreak, terminal_id ordering) without a live
// DB, so BranchATMService's dedup/scope/pagination logic can be
// property-tested per design.md's Testing Strategy.
type fakeBranchATMRepo struct {
	today       time.Time // stands in for CURRENT_DATE
	branches    map[int64]int64 // branchID -> vendorID
	packages    map[int64]fakePackage
	assignments []fakeAssignment
	atms        map[int64]fakeATM
}

func (f *fakeBranchATMRepo) BranchVendorID(_ context.Context, branchID int64) (*int64, error) {
	v, ok := f.branches[branchID]
	if !ok {
		return nil, nil
	}
	return &v, nil
}

// assignmentWins reports whether candidate beats current under the SQL's
// DISTINCT ON (a.id) tiebreak from `ORDER BY a.id, avp.effective_start_date
// DESC, avp.id DESC`: the kept row per atm_id has the latest
// effective_start_date, ties broken by the highest assignment id.
func assignmentWins(candidate, current fakeAssignment) bool {
	if !candidate.effectiveStartDate.Equal(current.effectiveStartDate) {
		return candidate.effectiveStartDate.After(current.effectiveStartDate)
	}
	return candidate.id > current.id
}

// managedATMs returns the deduplicated, terminal_id-ordered ATM rows for
// branchID exactly as ListBranchATMs's SQL would, before LIMIT/OFFSET.
func (f *fakeBranchATMRepo) managedATMs(branchID int64) []db.ListBranchATMsRow {
	type candidate struct {
		assignment  fakeAssignment
		packageCode string
	}
	best := map[int64]candidate{}

	for _, a := range f.assignments {
		pkg, ok := f.packages[a.vendorPackageID]
		if !ok || pkg.vendorBranchID != branchID {
			continue
		}
		if !a.isActive {
			continue
		}
		if a.effectiveStartDate.After(f.today) {
			continue
		}
		if a.effectiveEndDate != nil && a.effectiveEndDate.Before(f.today) {
			continue
		}
		cur, exists := best[a.atmID]
		if !exists || assignmentWins(a, cur.assignment) {
			best[a.atmID] = candidate{assignment: a, packageCode: pkg.packageCode}
		}
	}

	rows := make([]db.ListBranchATMsRow, 0, len(best))
	for atmID, c := range best {
		atm, ok := f.atms[atmID]
		if !ok {
			continue
		}
		rows = append(rows, db.ListBranchATMsRow{
			AtmID:                 atm.id,
			TerminalID:            atm.terminalID,
			PriorityClass:         atm.priorityClass,
			IsActive:              atm.isActive,
			LocationName:          atm.locationName,
			LocationCityOrRegency: atm.locationCityOrRegency,
			PackageCode:           c.packageCode,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].TerminalID < rows[j].TerminalID })
	return rows
}

func (f *fakeBranchATMRepo) List(_ context.Context, arg db.ListBranchATMsParams) ([]db.ListBranchATMsRow, error) {
	if arg.VendorBranchID == nil {
		return nil, nil
	}
	rows := f.managedATMs(*arg.VendorBranchID)
	start := int(arg.PageOffset)
	if start > len(rows) {
		start = len(rows)
	}
	end := start + int(arg.PageLimit)
	if end > len(rows) {
		end = len(rows)
	}
	return rows[start:end], nil
}

func (f *fakeBranchATMRepo) Count(_ context.Context, branchID int64) (int64, error) {
	return int64(len(f.managedATMs(branchID))), nil
}

var today = mustDay("2026-06-15")

func mustDay(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

// genActiveScenario builds a single branch (id 100, owned by vendor 1) with
// 1..3 packages and 1..8 ATMs, each carrying 1..3 active assignments to
// those packages (dates chosen so the window always covers `today`) --
// exercising the >1-assignment-per-ATM dedup case (Req 1.2).
func genActiveScenario(rt *rapid.T) (repo *fakeBranchATMRepo, vendorID, branchID int64, wantATMIDs map[int64]struct{}) {
	vendorID, branchID = 1, 100

	numPackages := rapid.IntRange(1, 3).Draw(rt, "numPackages")
	packages := map[int64]fakePackage{}
	for i := 1; i <= numPackages; i++ {
		pkgID := int64(i)
		packages[pkgID] = fakePackage{id: pkgID, vendorBranchID: branchID, packageCode: fmt.Sprintf("PKG-%d", pkgID)}
	}

	numATMs := rapid.IntRange(1, 8).Draw(rt, "numATMs")
	atms := map[int64]fakeATM{}
	var assignments []fakeAssignment
	nextAssignmentID := int64(1)
	wantATMIDs = map[int64]struct{}{}

	for i := 1; i <= numATMs; i++ {
		atmID := int64(i)
		atms[atmID] = fakeATM{id: atmID, terminalID: fmt.Sprintf("ATM%03d", atmID), isActive: true}

		numAssignments := rapid.IntRange(1, 3).Draw(rt, fmt.Sprintf("numAssignments%d", atmID))
		for j := 0; j < numAssignments; j++ {
			pkgID := int64(rapid.IntRange(1, numPackages).Draw(rt, fmt.Sprintf("pkg%d_%d", atmID, j)))
			startOffset := rapid.IntRange(-30, 0).Draw(rt, fmt.Sprintf("start%d_%d", atmID, j))
			var end *time.Time
			if rapid.Bool().Draw(rt, fmt.Sprintf("hasEnd%d_%d", atmID, j)) {
				e := today.AddDate(0, 0, rapid.IntRange(0, 30).Draw(rt, fmt.Sprintf("end%d_%d", atmID, j)))
				end = &e
			}
			assignments = append(assignments, fakeAssignment{
				id:                 nextAssignmentID,
				atmID:              atmID,
				vendorPackageID:    pkgID,
				isActive:           true,
				effectiveStartDate: today.AddDate(0, 0, startOffset),
				effectiveEndDate:   end,
			})
			nextAssignmentID++
		}
		wantATMIDs[atmID] = struct{}{}
	}

	repo = &fakeBranchATMRepo{
		today:       today,
		branches:    map[int64]int64{branchID: vendorID},
		packages:    packages,
		assignments: assignments,
		atms:        atms,
	}
	return
}

// Feature: vendor-branch-atms, Property 1: For any vendor branch and any set
// of active ATM_Assignments -- including an ATM linked by multiple active
// assignments to packages of that same branch -- the full (unpaginated)
// result of BranchATMService.List contains each managed ATM's atm_id at
// most once.
func TestProperty1_OneRowPerManagedATM(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		repo, vendorID, branchID, wantATMIDs := genActiveScenario(rt)
		svc := NewBranchATMService(repo)

		res, err := svc.List(context.Background(), vendorID, branchID, int64(len(wantATMIDs))+10, 0)
		if err != nil {
			rt.Fatalf("List: %v", err)
		}

		seen := map[int64]struct{}{}
		for _, a := range res.ATMs {
			if _, dup := seen[a.ATMID]; dup {
				rt.Fatalf("atm_id %d returned more than once", a.ATMID)
			}
			seen[a.ATMID] = struct{}{}
		}
		if len(seen) != len(wantATMIDs) {
			rt.Fatalf("got %d distinct atms, want %d", len(seen), len(wantATMIDs))
		}
	})
}

// Feature: vendor-branch-atms, Property 2: For any vendor branch, the Total
// returned by BranchATMService.List equals the number of distinct ATMs
// managed by that branch under the active-assignment filter, and is
// independent of the requested page and page size.
func TestProperty2_TotalEqualsDistinctManagedATMCount(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		repo, vendorID, branchID, wantATMIDs := genActiveScenario(rt)
		svc := NewBranchATMService(repo)
		want := int64(len(wantATMIDs))

		limit := int64(rapid.IntRange(1, 50).Draw(rt, "pageLimit"))
		offset := int64(rapid.IntRange(0, 50).Draw(rt, "pageOffset"))

		res, err := svc.List(context.Background(), vendorID, branchID, limit, offset)
		if err != nil {
			rt.Fatalf("List: %v", err)
		}
		if res.Total != want {
			rt.Fatalf("Total = %d, want %d (limit=%d offset=%d)", res.Total, want, limit, offset)
		}
	})
}

// Feature: vendor-branch-atms, Property 3: For any vendor branch and any
// page size within limits, concatenating the atm_id sequences of successive
// pages reproduces the full deduplicated result exactly once each, in
// terminal_id ascending order, and no page returns more rows than the
// effective page size.
func TestProperty3_PaginationPartitionsOrderedResult(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		repo, vendorID, branchID, wantATMIDs := genActiveScenario(rt)
		svc := NewBranchATMService(repo)

		full, err := svc.List(context.Background(), vendorID, branchID, int64(len(wantATMIDs))+10, 0)
		if err != nil {
			rt.Fatalf("full List: %v", err)
		}
		for i := 1; i < len(full.ATMs); i++ {
			if full.ATMs[i-1].TerminalID > full.ATMs[i].TerminalID {
				rt.Fatalf("full result not terminal_id ascending at index %d: %q > %q",
					i, full.ATMs[i-1].TerminalID, full.ATMs[i].TerminalID)
			}
		}

		pageSize := int64(rapid.IntRange(1, 100).Draw(rt, "pageSize"))

		var collected []int64
		maxPages := int64(len(wantATMIDs)) + 5
		for page := int64(0); page <= maxPages; page++ {
			res, err := svc.List(context.Background(), vendorID, branchID, pageSize, page*pageSize)
			if err != nil {
				rt.Fatalf("page %d List: %v", page, err)
			}
			if int64(len(res.ATMs)) > pageSize {
				rt.Fatalf("page %d returned %d rows, more than page size %d", page, len(res.ATMs), pageSize)
			}
			if len(res.ATMs) == 0 {
				break
			}
			for _, a := range res.ATMs {
				collected = append(collected, a.ATMID)
			}
		}

		if len(collected) != len(full.ATMs) {
			rt.Fatalf("collected %d atm ids across pages, want %d", len(collected), len(full.ATMs))
		}
		for i, a := range full.ATMs {
			if collected[i] != a.ATMID {
				rt.Fatalf("page-concatenated order mismatch at index %d: got atm_id %d, want %d", i, collected[i], a.ATMID)
			}
		}
	})
}

// Feature: vendor-branch-atms, Property 4: For any (vendorId, branchId)
// pair, BranchATMService.List returns ErrBranchNotFound whenever the branch
// does not exist, or exists but its vendor_id differs from vendorId; and
// returns a (possibly empty) result only when the branch exists and belongs
// to vendorId.
func TestProperty4_VendorScopeEnforcedIndependentOfBranchExistence(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		branchExists := rapid.Bool().Draw(rt, "branchExists")
		vendorID := int64(rapid.IntRange(1, 5).Draw(rt, "vendorID"))
		branchID := int64(rapid.IntRange(100, 105).Draw(rt, "branchID"))

		repo := &fakeBranchATMRepo{
			today:    today,
			branches: map[int64]int64{},
			packages: map[int64]fakePackage{},
			atms:     map[int64]fakeATM{},
		}

		wantErr := true
		if branchExists {
			ownerID := int64(rapid.IntRange(1, 5).Draw(rt, "ownerID"))
			repo.branches[branchID] = ownerID
			wantErr = ownerID != vendorID
		}

		svc := NewBranchATMService(repo)
		res, err := svc.List(context.Background(), vendorID, branchID, 25, 0)

		if wantErr {
			if !errors.Is(err, ErrBranchNotFound) {
				rt.Fatalf("want ErrBranchNotFound, got %v (branchExists=%v)", err, branchExists)
			}
			return
		}
		if err != nil {
			rt.Fatalf("want no error for an owned, existing branch, got %v", err)
		}
		if res.Total != 0 || len(res.ATMs) != 0 {
			rt.Fatalf("expected empty result for a branch with no assignments, got %+v", res)
		}
	})
}

// Feature: vendor-branch-atms, Property 5: For any existing branch owned by
// the requested vendor that has no active managed ATMs, BranchATMService.List
// returns an empty ATM slice and a Total of zero.
func TestProperty5_EmptyBranchYieldsEmptyPageAndZeroTotal(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		const vendorID, branchID = int64(1), int64(100)

		repo := &fakeBranchATMRepo{
			today:    today,
			branches: map[int64]int64{branchID: vendorID},
			packages: map[int64]fakePackage{1: {id: 1, vendorBranchID: branchID, packageCode: "PKG-1"}},
			atms:     map[int64]fakeATM{1: {id: 1, terminalID: "ATM001", isActive: true}},
		}

		// Only-ended, not-yet-started, or inactive assignments -- none should
		// count as "currently managed" (or no assignment at all).
		switch rapid.IntRange(0, 3).Draw(rt, "assignmentShape") {
		case 0:
			// no assignments at all
		case 1:
			end := today.AddDate(0, 0, -1)
			repo.assignments = []fakeAssignment{{id: 1, atmID: 1, vendorPackageID: 1, isActive: true, effectiveStartDate: today.AddDate(0, 0, -30), effectiveEndDate: &end}}
		case 2:
			repo.assignments = []fakeAssignment{{id: 1, atmID: 1, vendorPackageID: 1, isActive: true, effectiveStartDate: today.AddDate(0, 0, 5)}}
		case 3:
			repo.assignments = []fakeAssignment{{id: 1, atmID: 1, vendorPackageID: 1, isActive: false, effectiveStartDate: today.AddDate(0, 0, -30)}}
		}

		svc := NewBranchATMService(repo)
		res, err := svc.List(context.Background(), vendorID, branchID, 25, 0)
		if err != nil {
			rt.Fatalf("List: %v", err)
		}
		if res.Total != 0 {
			rt.Fatalf("Total = %d, want 0", res.Total)
		}
		if len(res.ATMs) != 0 {
			rt.Fatalf("ATMs = %+v, want empty", res.ATMs)
		}
	})
}

// Example/unit tests (tasks.md 4.7).
func TestBranchATMService_List_ExampleCases(t *testing.T) {
	ctx := context.Background()

	t.Run("nonexistent branch returns ErrBranchNotFound", func(t *testing.T) {
		repo := &fakeBranchATMRepo{today: today, branches: map[int64]int64{}}
		svc := NewBranchATMService(repo)
		if _, err := svc.List(ctx, 1, 999, 25, 0); !errors.Is(err, ErrBranchNotFound) {
			t.Fatalf("want ErrBranchNotFound, got %v", err)
		}
	})

	t.Run("cross-vendor branch returns ErrBranchNotFound", func(t *testing.T) {
		repo := &fakeBranchATMRepo{today: today, branches: map[int64]int64{100: 4}}
		svc := NewBranchATMService(repo)
		if _, err := svc.List(ctx, 3, 100, 25, 0); !errors.Is(err, ErrBranchNotFound) {
			t.Fatalf("want ErrBranchNotFound, got %v", err)
		}
	})

	t.Run("null location and priority_class map to nil DTO fields", func(t *testing.T) {
		repo := &fakeBranchATMRepo{
			today:    today,
			branches: map[int64]int64{100: 3},
			packages: map[int64]fakePackage{1: {id: 1, vendorBranchID: 100, packageCode: "PKG-1"}},
			atms:     map[int64]fakeATM{1: {id: 1, terminalID: "ATM001", isActive: true}}, // no location, no priority_class
			assignments: []fakeAssignment{
				{id: 1, atmID: 1, vendorPackageID: 1, isActive: true, effectiveStartDate: today.AddDate(0, 0, -1)},
			},
		}
		svc := NewBranchATMService(repo)
		res, err := svc.List(ctx, 3, 100, 25, 0)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(res.ATMs) != 1 {
			t.Fatalf("want 1 atm, got %d", len(res.ATMs))
		}
		got := res.ATMs[0]
		if got.LocationName != nil || got.LocationCityOrRegency != nil || got.PriorityClass != nil {
			t.Fatalf("want nil location/priority_class fields, got %+v", got)
		}
	})

	t.Run("deterministic package-code tiebreak picks the latest-starting assignment", func(t *testing.T) {
		repo := &fakeBranchATMRepo{
			today:    today,
			branches: map[int64]int64{100: 3},
			packages: map[int64]fakePackage{
				1: {id: 1, vendorBranchID: 100, packageCode: "PKG-OLD"},
				2: {id: 2, vendorBranchID: 100, packageCode: "PKG-NEW"},
			},
			atms: map[int64]fakeATM{1: {id: 1, terminalID: "ATM001", isActive: true}},
			assignments: []fakeAssignment{
				{id: 1, atmID: 1, vendorPackageID: 1, isActive: true, effectiveStartDate: today.AddDate(0, 0, -30)},
				{id: 2, atmID: 1, vendorPackageID: 2, isActive: true, effectiveStartDate: today.AddDate(0, 0, -1)},
			},
		}
		svc := NewBranchATMService(repo)
		res, err := svc.List(ctx, 3, 100, 25, 0)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(res.ATMs) != 1 || res.ATMs[0].PackageCode != "PKG-NEW" {
			t.Fatalf("want PKG-NEW (latest-starting), got %+v", res.ATMs)
		}
	})
}
