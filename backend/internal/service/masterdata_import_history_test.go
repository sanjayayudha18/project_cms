package service

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/repository"
)

type fakeHistoryRepo struct {
	listRows  []db.ListMasterDataImportBatchesRow
	total     int64
	gotFilter repository.ImportBatchFilter
	batch     db.GetMasterDataImportBatchRow
	batchErr  error
	changes   []db.MasterDataChangeRequest
}

func (f *fakeHistoryRepo) ListBatches(_ context.Context, fl repository.ImportBatchFilter) ([]db.ListMasterDataImportBatchesRow, int64, error) {
	f.gotFilter = fl
	return f.listRows, f.total, nil
}

func (f *fakeHistoryRepo) GetBatch(context.Context, int64) (db.GetMasterDataImportBatchRow, error) {
	return f.batch, f.batchErr
}

func (f *fakeHistoryRepo) ListByBatch(context.Context, int64) ([]db.MasterDataChangeRequest, error) {
	return f.changes, nil
}

func TestPageSlice(t *testing.T) {
	rows := []int{1, 2, 3, 4, 5}
	tests := []struct {
		name       string
		page, size int
		want       []int
	}{
		{"first page", 1, 2, []int{1, 2}},
		{"last partial page", 3, 2, []int{5}},
		{"past the end", 4, 2, nil},
		{"page zero", 0, 2, nil},
		{"size zero", 1, 0, nil},
		{"size larger than rows", 1, 50, rows},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pageSlice(rows, tt.page, tt.size); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("pageSlice(%d,%d) = %v, want %v", tt.page, tt.size, got, tt.want)
			}
		})
	}
}

func TestNormalizePaging(t *testing.T) {
	tests := []struct {
		name                 string
		page, size, def, max int
		wantPage, wantSize   int
	}{
		{"defaults", 0, 0, 20, 100, 1, 20},
		{"negative", -3, -1, 20, 100, 1, 20},
		{"capped", 2, 500, 20, 100, 2, 100},
		{"kept", 3, 40, 20, 100, 3, 40},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, s := normalizePaging(tt.page, tt.size, tt.def, tt.max)
			if p != tt.wantPage || s != tt.wantSize {
				t.Fatalf("got %d,%d want %d,%d", p, s, tt.wantPage, tt.wantSize)
			}
		})
	}
}

func TestIsFailedRow(t *testing.T) {
	str := func(s string) *string { return &s }
	tests := []struct {
		name string
		in   *string
		want bool
	}{
		{"nil", nil, false},
		{"empty", str(""), false},
		{"real cause", str("vendor code already exists"), true},
		{"rolled back", str(BatchRollbackPrefix + "change request 7 failed"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isFailedRow(tt.in); got != tt.want {
				t.Fatalf("isFailedRow = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMasterDataImportHistoryService_Get(t *testing.T) {
	cause := "boom"
	rolled := BatchRollbackPrefix + "change request 1 failed"
	repo := &fakeHistoryRepo{
		batch: db.GetMasterDataImportBatchRow{ID: 5, Entity: "vendors", RowCount: 4, MakerID: 2, MakerName: "Siti", HeadStatus: "stale",
			DecidedAt: pgtype.Timestamptz{Time: time.Unix(1700000000, 0), Valid: true}},
		changes: []db.MasterDataChangeRequest{
			{ID: 1, Op: "create", Status: "stale", Payload: []byte(`{"a":1}`), Error: &cause},
			{ID: 2, Op: "update", Status: "stale", Payload: []byte(`{}`), Before: []byte(`{"b":2}`), Error: &rolled},
			{ID: 3, Op: "disable", Status: "stale", Payload: []byte(`{}`)},
			{ID: 4, Op: "enable", Status: "stale", Payload: []byte(`{}`)},
		},
	}
	svc := NewMasterDataImportHistoryService(repo)

	got, err := svc.Get(adminCtx(1), 5, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	if got.Counts != (ImportBatchCounts{Create: 1, Update: 1, Enable: 1, Disable: 1}) {
		t.Fatalf("counts = %+v (must cover the whole batch)", got.Counts)
	}
	if got.Total != 4 || got.Page != 2 || got.PageSize != 3 || len(got.Changes) != 1 || got.Changes[0].ID != 4 {
		t.Fatalf("paging = total %d page %d size %d changes %+v", got.Total, got.Page, got.PageSize, got.Changes)
	}
	if got.DecidedAt == nil || got.DecidedByName != nil || got.Status != "stale" || got.MakerName != "Siti" {
		t.Fatalf("summary = %+v", got.ImportBatchSummary)
	}

	first, err := svc.Get(adminCtx(1), 5, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Changes[0].Failed || first.Changes[1].Failed || first.Changes[2].Failed {
		t.Fatalf("failed flags = %v %v %v, want only the row with the real cause", first.Changes[0].Failed, first.Changes[1].Failed, first.Changes[2].Failed)
	}
	if first.Changes[0].Before != nil || string(first.Changes[1].Before) != `{"b":2}` {
		t.Fatalf("before = %s / %s", first.Changes[0].Before, first.Changes[1].Before)
	}
}

func TestMasterDataImportHistoryService_Get_FailureReason(t *testing.T) {
	cause := "vendor code already exists"
	rolled := BatchRollbackPrefix + "change request 1 failed"
	// The failed row is row 3; page 1 (size 2) does not contain it.
	changes := []db.MasterDataChangeRequest{
		{ID: 1, Op: "create", Status: "stale", Payload: []byte(`{}`), Error: &rolled},
		{ID: 2, Op: "create", Status: "stale", Payload: []byte(`{}`), Error: &rolled},
		{ID: 3, Op: "create", Status: "stale", Payload: []byte(`{}`), Error: &cause},
	}
	oldBatch := []db.MasterDataChangeRequest{
		{ID: 1, Op: "create", Status: "stale", Payload: []byte(`{}`), Error: &rolled},
	}
	tests := []struct {
		name    string
		changes []db.MasterDataChangeRequest
		page    int
		want    *string
	}{
		{"cause on another page", changes, 1, &cause},
		{"cause on the current page", changes, 2, &cause},
		{"old batch without marker", oldBatch, 1, nil},
		{"no error at all", []db.MasterDataChangeRequest{{ID: 1, Op: "create", Payload: []byte(`{}`)}}, 1, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewMasterDataImportHistoryService(&fakeHistoryRepo{
				batch:   db.GetMasterDataImportBatchRow{ID: 5, Entity: "vendors", HeadStatus: "stale"},
				changes: tt.changes,
			})
			got, err := svc.Get(adminCtx(1), 5, tt.page, 2)
			if err != nil {
				t.Fatal(err)
			}
			if (got.FailureReason == nil) != (tt.want == nil) || (tt.want != nil && *got.FailureReason != *tt.want) {
				t.Fatalf("FailureReason = %v, want %v", got.FailureReason, tt.want)
			}
		})
	}
}

func TestMasterDataImportHistoryService_Get_NotFoundAndForbidden(t *testing.T) {
	svc := NewMasterDataImportHistoryService(&fakeHistoryRepo{batchErr: pgx.ErrNoRows})
	if _, err := svc.Get(adminCtx(1), 9, 1, 50); !errors.Is(err, ErrImportBatchNotFound) {
		t.Fatalf("err = %v, want ErrImportBatchNotFound", err)
	}
	if _, err := svc.Get(ctxWithRole(1, "ATM-USER"), 9, 1, 50); !errors.Is(err, ErrMasterDataForbidden) {
		t.Fatalf("err = %v, want ErrMasterDataForbidden", err)
	}
	if _, err := svc.List(context.Background(), repository.ImportBatchFilter{}, 1, 20); !errors.Is(err, ErrMasterDataForbidden) {
		t.Fatalf("List without auth err = %v, want ErrMasterDataForbidden", err)
	}
}

func TestMasterDataImportHistoryService_List(t *testing.T) {
	name := "Budi"
	repo := &fakeHistoryRepo{total: 41, listRows: []db.ListMasterDataImportBatchesRow{
		{ID: 9, Entity: "atms", RowCount: 3, MakerID: 2, MakerName: "Siti", HeadStatus: "applied", DecidedByName: &name,
			CreatedAt: pgtype.Timestamptz{Time: time.Unix(1700000000, 0), Valid: true}},
	}}
	svc := NewMasterDataImportHistoryService(repo)

	got, err := svc.List(adminCtx(1), repository.ImportBatchFilter{Limit: 999, Offset: 999}, 3, 500)
	if err != nil {
		t.Fatal(err)
	}
	if repo.gotFilter.Limit != importHistoryMaxPageSize || repo.gotFilter.Offset != 2*importHistoryMaxPageSize {
		t.Fatalf("filter limit/offset = %d/%d, want %d/%d", repo.gotFilter.Limit, repo.gotFilter.Offset, importHistoryMaxPageSize, 2*importHistoryMaxPageSize)
	}
	if got.Total != 41 || got.Page != 3 || got.PageSize != 100 || len(got.Items) != 1 {
		t.Fatalf("page = %+v", got)
	}
	if it := got.Items[0]; it.Status != "applied" || it.DecidedAt != nil || it.DecidedByName == nil || it.Entity != "atms" {
		t.Fatalf("item = %+v", it)
	}
}
