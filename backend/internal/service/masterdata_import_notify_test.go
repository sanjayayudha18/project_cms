package service

import (
	"context"
	"errors"
	"testing"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/notification"
)

// failingNotifyQuerier fails every read the notifier makes. The embedded
// notification.Querier is nil on purpose: the notifier must stop before Send.
type failingNotifyQuerier struct {
	notification.Querier
	calls int
}

func (f *failingNotifyQuerier) GetMasterDataImportBatch(context.Context, int64) (db.GetMasterDataImportBatchRow, error) {
	f.calls++
	return db.GetMasterDataImportBatchRow{}, errors.New("db down")
}

func (f *failingNotifyQuerier) ListPendingApproversForRequest(context.Context, int64) ([]int64, error) {
	f.calls++
	return nil, errors.New("db down")
}

func TestImportNotifier_QuerierFailureIsSwallowed(t *testing.T) {
	q := &failingNotifyQuerier{}
	var n ImportNotifier = &notificationImportNotifier{svc: notification.NewService(false), q: q}
	ctx := context.Background()

	// No return values and no panic: a failing DB can never reach the caller.
	n.BatchSubmitted(ctx, 1, 10)
	n.BatchApplied(ctx, 1, 2)
	n.BatchRejected(ctx, 1, 2)
	n.BatchApplyFailed(ctx, 1, 2)
	if q.calls != 4 {
		t.Fatalf("querier calls = %d, want 4 (one failed read per method)", q.calls)
	}
}

func TestImportNotifyBody(t *testing.T) {
	file := "vendor_nov.csv"
	tests := []struct {
		name    string
		batch   db.GetMasterDataImportBatchRow
		outcome string
		want    string
	}{
		{"with file", db.GetMasterDataImportBatchRow{ID: 12, Entity: ExportVendors, FileName: &file, RowCount: 35}, "ditolak",
			"Impor CSV Vendor batch #12 (file vendor_nov.csv), 35 baris, ditolak."},
		{"no file", db.GetMasterDataImportBatchRow{ID: 3, Entity: ExportATMAssignments, RowCount: 1}, "sudah disetujui dan diterapkan",
			"Impor CSV Kelolaan ATM batch #3, 1 baris, sudah disetujui dan diterapkan."},
		{"unknown entity falls back to code", db.GetMasterDataImportBatchRow{ID: 4, Entity: "zzz", RowCount: 2}, "ditolak",
			"Impor CSV zzz batch #4, 2 baris, ditolak."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := importNotifyBody(tt.batch, tt.outcome); got != tt.want {
				t.Fatalf("body = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestImportEntityLabelsCoverEveryImportEntity(t *testing.T) {
	for _, e := range []string{ExportVendors, ExportVendorBranches, ExportVendorVaults, ExportVendorPICs, ExportATMs, ExportATMAssignments} {
		if importEntityLabels[e] == "" {
			t.Errorf("no label for import entity %q", e)
		}
	}
}
