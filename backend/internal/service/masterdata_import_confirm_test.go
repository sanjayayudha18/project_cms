package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgtype"
	"pgregory.net/rapid"

	"github.com/cimb-niaga/cms/backend/internal/audit"
	"github.com/cimb-niaga/cms/backend/internal/db"
)

// Feature: master-data-import-revamp, Property 6: nama file tersanitasi.
func TestSanitizeImportFileName_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		raw := rapid.String().Draw(t, "raw")
		got := sanitizeImportFileName(raw)
		if got == nil {
			return
		}
		n := utf8.RuneCountInString(*got)
		if n < 1 || n > maxImportFileNameRunes {
			t.Fatalf("rune count %d out of 1..%d for %q", n, maxImportFileNameRunes, *got)
		}
		if strings.ContainsAny(*got, `/\`) {
			t.Fatalf("separator left in %q", *got)
		}
		for _, r := range *got {
			if unicode.IsControl(r) {
				t.Fatalf("control rune %U left in %q", r, *got)
			}
		}
		if again := sanitizeImportFileName(*got); again == nil || *again != *got {
			t.Fatalf("not idempotent: %q -> %v", *got, again)
		}
	})
}

func TestSanitizeImportFileName(t *testing.T) {
	long := strings.Repeat("あ", 255)
	tests := []struct {
		name string
		in   string
		want *string
	}{
		{"plain", "vendors.csv", strp("vendors.csv")},
		{"windows path", `C:\Users\me\Docs\vendors.csv`, strp("vendors.csv")},
		{"unix path", "/home/me/vendors.csv", strp("vendors.csv")},
		{"mixed separators", `a/b\c.csv`, strp("c.csv")},
		{"dot dot", "..", nil},
		{"dot dot with dir", "../..", nil},
		{"single dot", ".", nil},
		{"empty", "", nil},
		{"only spaces", "   \t ", nil},
		{"only separators", "///", nil},
		{"control chars stripped", "ven\x00dors\n.csv", strp("vendors.csv")},
		{"trimmed", "  vendors.csv  ", strp("vendors.csv")},
		{"multibyte exactly 255 kept", long, strp(long)},
		{"multibyte 256 cut to 255", long + "あ", strp(long)},
		{"cut then trimmed", strings.Repeat("a", 254) + " b", strp(strings.Repeat("a", 254))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeImportFileName(tt.in)
			switch {
			case tt.want == nil && got != nil:
				t.Fatalf("got %q, want nil", *got)
			case tt.want != nil && got == nil:
				t.Fatalf("got nil, want %q", *tt.want)
			case tt.want != nil && *got != *tt.want:
				t.Fatalf("got %q, want %q", *got, *tt.want)
			}
		})
	}
}

type finishBatches struct {
	setCalls int
	setErr   error
}

func (*finishBatches) FindOpen(context.Context, string, string) (*db.FindOpenMasterDataImportBatchRow, error) {
	return nil, nil
}

func (*finishBatches) Create(context.Context, string, string, *string, int64, []db.CreateMasterDataChangeRequestParams) (*db.FindOpenMasterDataImportBatchRow, bool, error) {
	return nil, false, nil
}

func (b *finishBatches) SetApprovalRequestID(context.Context, int64, int64) error {
	b.setCalls++
	return b.setErr
}

type finishOrchestrator struct {
	approvalID int64
	created    bool
	err        error
	calls      int
}

func (o *finishOrchestrator) SubmitForApproval(context.Context, int64, string, int64, pgtype.Numeric, string) (db.ApprovalRequest, bool, error) {
	o.calls++
	return db.ApprovalRequest{ID: o.approvalID}, o.created, o.err
}

type finishAudit struct {
	entries []audit.Entry
	err     error
}

func (a *finishAudit) Write(_ context.Context, e audit.Entry) error {
	a.entries = append(a.entries, e)
	return a.err
}

type finishNotifier struct{ submitted [][2]int64 }

func (n *finishNotifier) BatchSubmitted(_ context.Context, batchID, approvalID int64) {
	n.submitted = append(n.submitted, [2]int64{batchID, approvalID})
}
func (*finishNotifier) BatchApplied(context.Context, int64, int64)     {}
func (*finishNotifier) BatchRejected(context.Context, int64, int64)    {}
func (*finishNotifier) BatchApplyFailed(context.Context, int64, int64) {}

type finishRig struct {
	batches  *finishBatches
	orch     *finishOrchestrator
	audit    *finishAudit
	notifier *finishNotifier
	importer *MasterDataImporter
}

func newFinishRig(orch *finishOrchestrator, withNotifier bool) *finishRig {
	r := &finishRig{batches: &finishBatches{}, orch: orch, audit: &finishAudit{}, notifier: &finishNotifier{}}
	deps := ImportConfirmDeps{Batches: r.batches, Orchestrator: orch, Audit: r.audit}
	if withNotifier {
		deps.Notifier = r.notifier
	}
	r.importer = NewMasterDataImporter(nil).WithConfirm(deps)
	return r
}

func newBatchRow(approval *int64) *db.FindOpenMasterDataImportBatchRow {
	return &db.FindOpenMasterDataImportBatchRow{ID: 11, MakerID: 7, HeadID: 21, RowCount: 3,
		FileName: strp("stored.csv"), ApprovalRequestID: approval}
}

func TestImportFinish_NewBatchAuditsAndNotifiesOnce(t *testing.T) {
	r := newFinishRig(&finishOrchestrator{approvalID: 99, created: true}, true)

	res, err := r.importer.finish(context.Background(), 7, ExportVendors, "hash", newBatchRow(nil), false, "10.0.0.1")
	if err != nil {
		t.Fatalf("finish: %v", err)
	}
	if res.ApprovalRequestID == nil || *res.ApprovalRequestID != 99 {
		t.Fatalf("approval id = %v, want 99", res.ApprovalRequestID)
	}
	if len(r.audit.entries) != 1 || r.audit.entries[0].Action != "submit_batch" {
		t.Fatalf("audit entries = %+v, want one submit_batch", r.audit.entries)
	}
	after, _ := r.audit.entries[0].After.(map[string]any)
	if got, _ := after["file_name"].(*string); got == nil || *got != "stored.csv" {
		t.Errorf("audit file_name = %v, want the batch's stored name", after["file_name"])
	}
	if len(r.notifier.submitted) != 1 || r.notifier.submitted[0] != [2]int64{11, 99} {
		t.Fatalf("notifier calls = %v, want exactly [{11 99}]", r.notifier.submitted)
	}
}

func TestImportFinish_NoAuditNoNotifier(t *testing.T) {
	existingApproval := int64(5)
	tests := []struct {
		name         string
		batch        *db.FindOpenMasterDataImportBatchRow
		orch         *finishOrchestrator
		wantSubmit   int
		wantSetLinks int
	}{
		{"existing batch with approval", newBatchRow(&existingApproval), &finishOrchestrator{approvalID: 1, created: true}, 0, 0},
		{"submit returns created=false", newBatchRow(nil), &finishOrchestrator{approvalID: 99, created: false}, 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newFinishRig(tt.orch, true)
			if _, err := r.importer.finish(context.Background(), 7, ExportVendors, "hash", tt.batch, true, "ip"); err != nil {
				t.Fatalf("finish: %v", err)
			}
			if r.orch.calls != tt.wantSubmit || r.batches.setCalls != tt.wantSetLinks {
				t.Errorf("submit=%d setApproval=%d, want %d/%d", r.orch.calls, r.batches.setCalls, tt.wantSubmit, tt.wantSetLinks)
			}
			if len(r.audit.entries) != 0 {
				t.Errorf("audit written %d time(s), want 0", len(r.audit.entries))
			}
			if len(r.notifier.submitted) != 0 {
				t.Errorf("notifier called %d time(s), want 0", len(r.notifier.submitted))
			}
		})
	}
}

func TestImportFinish_NilNotifierIsSafe(t *testing.T) {
	r := newFinishRig(&finishOrchestrator{approvalID: 99, created: true}, false)
	if _, err := r.importer.finish(context.Background(), 7, ExportVendors, "hash", newBatchRow(nil), false, "ip"); err != nil {
		t.Fatalf("finish with nil notifier: %v", err)
	}
	if len(r.audit.entries) != 1 {
		t.Errorf("audit entries = %d, want 1", len(r.audit.entries))
	}
}

func TestImportFinish_AuditFailureSkipsNotifier(t *testing.T) {
	r := newFinishRig(&finishOrchestrator{approvalID: 99, created: true}, true)
	r.audit.err = errors.New("audit down")

	if _, err := r.importer.finish(context.Background(), 7, ExportVendors, "hash", newBatchRow(nil), false, "ip"); err == nil {
		t.Fatal("expected the audit error to be returned")
	}
	if len(r.notifier.submitted) != 0 {
		t.Errorf("notifier called %d time(s) after a failed audit, want 0", len(r.notifier.submitted))
	}
}
