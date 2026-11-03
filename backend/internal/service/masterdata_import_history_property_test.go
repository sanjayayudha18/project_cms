package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/repository"
)

// maxImportRows is the largest batch the importer accepts (Req 3.1).
const maxImportRows = 2000

// Feature: master-data-import-revamp, Property 7: pagination covers the batch.
// Concatenating every page of pageSlice yields all n rows once, in order; pages
// past the end are empty.
func TestProperty7_PageSliceCoversEveryRowOnce(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, maxImportRows).Draw(t, "n")
		size := rapid.IntRange(1, maxImportRows+50).Draw(t, "pageSize")
		rows := make([]int64, n)
		for i := range rows {
			rows[i] = int64(i + 1)
		}

		pages := (n + size - 1) / size
		var joined []int64
		for p := 1; p <= pages; p++ {
			part := pageSlice(rows, p, size)
			if len(part) == 0 || len(part) > size {
				t.Fatalf("page %d/%d has %d rows (size %d)", p, pages, len(part), size)
			}
			joined = append(joined, part...)
		}
		if len(joined) != n {
			t.Fatalf("joined %d rows, want %d", len(joined), n)
		}
		for i, id := range joined {
			if id != int64(i+1) {
				t.Fatalf("row %d = id %d, want %d (order or duplicate)", i, id, i+1)
			}
		}
		if extra := pageSlice(rows, pages+rapid.IntRange(1, 5).Draw(t, "beyond"), size); len(extra) != 0 {
			t.Fatalf("page past the end returned %d rows", len(extra))
		}
	})
}

// Feature: master-data-import-revamp, Property 7: counts always cover the whole
// batch, whatever page is asked for.
func TestProperty7_CountsCoverWholeBatch(t *testing.T) {
	ops := []string{"create", "update", "enable", "disable"}
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, maxImportRows).Draw(t, "n")
		rows := make([]db.MasterDataChangeRequest, n)
		want := ImportBatchCounts{}
		for i := range rows {
			op := rapid.SampledFrom(ops).Draw(t, "op")
			rows[i] = db.MasterDataChangeRequest{ID: int64(i + 1), Op: op, Payload: []byte(`{}`)}
			switch op {
			case "create":
				want.Create++
			case "update":
				want.Update++
			case "enable":
				want.Enable++
			case "disable":
				want.Disable++
			}
		}
		svc := NewMasterDataImportHistoryService(&fakeHistoryRepo{batch: db.GetMasterDataImportBatchRow{ID: 1}, changes: rows})

		page := rapid.IntRange(1, 60).Draw(t, "page")
		size := rapid.IntRange(1, 100).Draw(t, "size")
		got, err := svc.Get(adminCtx(1), 1, page, size)
		if err != nil {
			t.Fatal(err)
		}
		if got.Counts != want {
			t.Fatalf("counts = %+v, want %+v (page %d size %d)", got.Counts, want, page, size)
		}
		if sum := got.Counts.Create + got.Counts.Update + got.Counts.Enable + got.Counts.Disable; sum != n || got.Total != n {
			t.Fatalf("counts sum %d, total %d, want %d", sum, got.Total, n)
		}
		if len(got.Changes) > size {
			t.Fatalf("page holds %d rows, size %d", len(got.Changes), size)
		}
	})
}

// countingHistoryRepo records any read; Property 8 requires zero.
type countingHistoryRepo struct {
	fakeHistoryRepo
	calls int
}

func (c *countingHistoryRepo) ListBatches(ctx context.Context, f repository.ImportBatchFilter) ([]db.ListMasterDataImportBatchesRow, int64, error) {
	c.calls++
	return c.fakeHistoryRepo.ListBatches(ctx, f)
}

func (c *countingHistoryRepo) GetBatch(ctx context.Context, id int64) (db.GetMasterDataImportBatchRow, error) {
	c.calls++
	return c.fakeHistoryRepo.GetBatch(ctx, id)
}

func (c *countingHistoryRepo) ListByBatch(ctx context.Context, id int64) ([]db.MasterDataChangeRequest, error) {
	c.calls++
	return c.fakeHistoryRepo.ListByBatch(ctx, id)
}

// Feature: master-data-import-revamp, Property 8 (history part): any role other
// than ADMIN / ADMIN_PARAM gets ErrMasterDataForbidden from List and Get, and the
// repository is never read.
func TestProperty8_HistoryRejectsNonAdminRoles(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		role := rapid.OneOf(
			rapid.SampledFrom([]string{"ATM-USER", "ATM-SPV", "BRANCH-ATM-SPV", "ACM-USER", "VENDOR-USER", "", "  "}),
			rapid.StringMatching(`[A-Za-z_\- ]{0,16}`),
		).Draw(t, "role")
		if r := strings.ToUpper(strings.TrimSpace(role)); r == "ADMIN" || r == "ADMIN_PARAM" {
			t.Skip("admin role")
		}
		repo := &countingHistoryRepo{fakeHistoryRepo: fakeHistoryRepo{batch: db.GetMasterDataImportBatchRow{ID: 1}}}
		svc := NewMasterDataImportHistoryService(repo)
		ctx := ctxWithRole(rapid.Int64Range(1, 1000).Draw(t, "user"), role)

		if _, err := svc.List(ctx, repository.ImportBatchFilter{}, 1, 20); !errors.Is(err, ErrMasterDataForbidden) {
			t.Fatalf("List role %q err = %v, want ErrMasterDataForbidden", role, err)
		}
		if _, err := svc.Get(ctx, 1, 1, 50); !errors.Is(err, ErrMasterDataForbidden) {
			t.Fatalf("Get role %q err = %v, want ErrMasterDataForbidden", role, err)
		}
		if repo.calls != 0 {
			t.Fatalf("repo read %d times for role %q", repo.calls, role)
		}
	})
}

func TestHistoryService_AdminRolesAllowed(t *testing.T) {
	for _, role := range []string{"ADMIN", "ADMIN_PARAM", " admin_param "} {
		svc := NewMasterDataImportHistoryService(&fakeHistoryRepo{})
		if _, err := svc.List(ctxWithRole(1, role), repository.ImportBatchFilter{}, 1, 20); err != nil {
			t.Errorf("role %q: %v", role, err)
		}
	}
}

// Req 9.3: reading history writes no audit. The service holds only its read
// repository, so there is nothing to write an audit with.
func TestHistoryService_HasNoAuditDependency(t *testing.T) {
	typ := reflect.TypeOf(MasterDataImportHistoryService{})
	if typ.NumField() != 1 || typ.Field(0).Name != "repo" {
		t.Fatalf("service fields changed (%d); a new dependency needs an audit decision per Req 9.3", typ.NumField())
	}
}
