package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cimb-niaga/cms/backend/internal/db"
	"github.com/cimb-niaga/cms/backend/internal/notification"
)

// ImportNotifier tells people about the life of a master-data import batch
// (master-data-import-revamp Req 8). It is best-effort by design: no method
// returns an error, so a notification problem can never change the outcome of
// a submit, approve, reject or apply. Failures are only logged.
type ImportNotifier interface {
	// BatchSubmitted notifies the assigned approvers of the lowest pending step.
	BatchSubmitted(ctx context.Context, batchID, approvalRequestID int64)
	BatchApplied(ctx context.Context, batchID, makerID int64)
	BatchRejected(ctx context.Context, batchID, makerID int64)
	BatchApplyFailed(ctx context.Context, batchID, makerID int64)
}

const (
	importNotifyEntityType = "master_data_import_batch"

	importNotifySubmitted   = "master_data_import.submitted"
	importNotifyApplied     = "master_data_import.applied"
	importNotifyRejected    = "master_data_import.rejected"
	importNotifyApplyFailed = "master_data_import.apply_failed"
)

var importEntityLabels = map[string]string{
	ExportVendors:        "Vendor",
	ExportVendorBranches: "Cabang vendor",
	ExportVendorVaults:   "Vault vendor",
	ExportVendorPICs:     "PIC vendor",
	ExportATMs:           "ATM",
	ExportATMAssignments: "Kelolaan ATM",
}

// importNotifyQuerier is what the notifier reads from the DB on top of what
// notification.Service.Send writes.
type importNotifyQuerier interface {
	notification.Querier
	GetMasterDataImportBatch(ctx context.Context, id int64) (db.GetMasterDataImportBatchRow, error)
	ListPendingApproversForRequest(ctx context.Context, approvalRequestID int64) ([]int64, error)
}

type notificationImportNotifier struct {
	svc *notification.Service
	q   importNotifyQuerier
}

// NewImportNotifier builds the notifier over the primary pool. It sends outside
// any business transaction (the approval orchestrator does not run in one).
func NewImportNotifier(svc *notification.Service, conn db.DBTX) ImportNotifier {
	return &notificationImportNotifier{svc: svc, q: db.New(conn)}
}

func (n *notificationImportNotifier) BatchSubmitted(ctx context.Context, batchID, approvalRequestID int64) {
	approvers, err := n.q.ListPendingApproversForRequest(ctx, approvalRequestID)
	if err != nil {
		warnImportNotify(ctx, importNotifySubmitted, batchID, err)
		return
	}
	n.send(ctx, batchID, importNotifySubmitted, "Impor data master menunggu persetujuan",
		"/approvals", true, notification.Recipients{UserIDs: approvers},
		"menunggu persetujuan Anda")
}

func (n *notificationImportNotifier) BatchApplied(ctx context.Context, batchID, makerID int64) {
	n.toMaker(ctx, batchID, makerID, importNotifyApplied, "Impor data master diterapkan", "sudah disetujui dan diterapkan")
}

func (n *notificationImportNotifier) BatchRejected(ctx context.Context, batchID, makerID int64) {
	n.toMaker(ctx, batchID, makerID, importNotifyRejected, "Impor data master ditolak", "ditolak")
}

func (n *notificationImportNotifier) BatchApplyFailed(ctx context.Context, batchID, makerID int64) {
	n.toMaker(ctx, batchID, makerID, importNotifyApplyFailed, "Impor data master gagal diterapkan", "disetujui tetapi gagal diterapkan")
}

func (n *notificationImportNotifier) toMaker(ctx context.Context, batchID, makerID int64, typ, title, outcome string) {
	link := fmt.Sprintf("/settings/admin/master-data-io?tab=history&batch=%d", batchID)
	n.send(ctx, batchID, typ, title, link, false, notification.Recipients{UserIDs: []int64{makerID}}, outcome)
}

// send loads the batch for the message body and fans the message out. Errors
// are logged, never returned.
func (n *notificationImportNotifier) send(ctx context.Context, batchID int64, typ, title, link string, email bool, to notification.Recipients, outcome string) {
	batch, err := n.q.GetMasterDataImportBatch(ctx, batchID)
	if err != nil {
		warnImportNotify(ctx, typ, batchID, err)
		return
	}
	msg := notification.Message{
		Type: typ, Title: title, Body: importNotifyBody(batch, outcome), Link: link,
		EntityType: importNotifyEntityType, EntityID: &batchID, Email: email,
	}
	if err := n.svc.Send(ctx, n.q, msg, to); err != nil {
		warnImportNotify(ctx, typ, batchID, err)
	}
}

// importNotifyBody names the entity, batch number, file (when known) and row count.
func importNotifyBody(b db.GetMasterDataImportBatchRow, outcome string) string {
	label, ok := importEntityLabels[b.Entity]
	if !ok {
		label = b.Entity
	}
	file := ""
	if b.FileName != nil {
		file = fmt.Sprintf(" (file %s)", *b.FileName)
	}
	return fmt.Sprintf("Impor CSV %s batch #%d%s, %d baris, %s.", label, b.ID, file, b.RowCount, outcome)
}

func warnImportNotify(ctx context.Context, typ string, batchID int64, err error) {
	slog.WarnContext(ctx, "import notification failed", "type", typ, "batch_id", batchID, "error", err)
}
