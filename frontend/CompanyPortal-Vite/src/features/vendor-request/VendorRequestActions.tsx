/**
 * Maker/checker action UI of the Vendor Request detail page: the button bar
 * under the items table and the reason / laporan selesai modals. State and
 * handlers live in useVendorRequestActions.
 */

import { Button } from "@/components/ui/Button";
import { Ban, CheckCircle2, XCircle } from "lucide-react";
import { ApproveCompletionModal, ReportCompletionModal } from "./CompletionPanels";
import { ReasonModal } from "./VendorRequestDetailParts";
import type { VendorRequestDetail as VendorRequestDetailType } from "./types";
import type { RequestPermissions, VendorRequestActionsState } from "./useVendorRequestActions";

interface ActionProps {
  a: VendorRequestActionsState;
}

export function VendorRequestActionBar({ a, perms }: ActionProps & { perms: RequestPermissions }) {
  const { isBusy, pending } = a;

  if (a.editingItems) {
    return (
      <>
        <Button variant="secondary" disabled={isBusy} onClick={() => a.setEditingItems(null)}>
          Batal
        </Button>
        <Button disabled={isBusy || !a.editValid} onClick={a.saveEditedItems}>
          {pending.updateItems ? "Menyimpan..." : "Simpan Item"}
        </Button>
      </>
    );
  }

  return (
    <>
      {perms.canEditItems && (
        <Button variant="secondary" disabled={isBusy} onClick={a.startEditItems}>
          Edit Item
        </Button>
      )}
      {perms.canSubmit && (
        <Button disabled={isBusy} onClick={a.submit}>
          {pending.submit ? "Mengirim..." : "Kirim untuk Approval"}
        </Button>
      )}
      {perms.canApproveReject && (
        <>
          <Button variant="danger" disabled={isBusy} onClick={() => a.reject.setOpen(true)}>
            <XCircle className="mr-1.5 h-4 w-4" aria-hidden="true" />
            Tolak
          </Button>
          <Button disabled={isBusy} onClick={a.approve}>
            {pending.approve ? "Menyetujui..." : "Setujui"}
          </Button>
        </>
      )}
      {perms.canRevise && (
        <Button disabled={isBusy} onClick={a.revise}>
          {pending.revise ? "Merevisi..." : "Revisi"}
        </Button>
      )}
      {perms.canVendorReturn && (
        <Button variant="secondary" disabled={isBusy} onClick={() => a.vendorReturn.setOpen(true)}>
          Kembalikan ke Pembuat
        </Button>
      )}
      {perms.canCancel && (
        <Button variant="danger" disabled={isBusy} onClick={() => a.cancel.setOpen(true)}>
          <Ban className="mr-1.5 h-4 w-4" aria-hidden="true" />
          Batalkan
        </Button>
      )}
      {perms.canReportCompletion && (
        <Button disabled={isBusy} onClick={() => a.setCompleteOpen(true)}>
          <CheckCircle2 className="mr-1.5 h-4 w-4" aria-hidden="true" />
          Laporkan Selesai
        </Button>
      )}
      {perms.canReviewCompletion && (
        <>
          <Button
            variant="danger"
            disabled={isBusy}
            onClick={() => a.rejectCompletion.setOpen(true)}
          >
            <XCircle className="mr-1.5 h-4 w-4" aria-hidden="true" />
            Tolak Laporan
          </Button>
          <Button disabled={isBusy} onClick={() => a.setApproveCompletionOpen(true)}>
            Setujui Laporan
          </Button>
        </>
      )}
    </>
  );
}

export function VendorRequestModals({ a, data }: ActionProps & { data: VendorRequestDetailType }) {
  return (
    <>
      {a.completeOpen && (
        <ReportCompletionModal
          atms={data.atms}
          results={a.completeResults}
          onResultChange={(terminalId, result) =>
            a.setCompleteResults((cur) => ({ ...cur, [terminalId]: result }))
          }
          busy={a.pending.submitCompletion}
          onConfirm={a.handleSubmitCompletion}
          onDismiss={a.dismissComplete}
        />
      )}

      {a.approveCompletionOpen && (
        <ApproveCompletionModal
          atms={data.atms}
          busy={a.pending.approveCompletion}
          onConfirm={a.handleApproveCompletion}
          onDismiss={() => a.setApproveCompletionOpen(false)}
        />
      )}

      {a.rejectCompletion.open && (
        <ReasonModal
          title="Tolak Laporan Selesai"
          reasonLabel="Alasan Penolakan Laporan"
          confirmLabel="Tolak Laporan"
          busyLabel="Menolak..."
          reason={a.rejectCompletion.reason}
          onReasonChange={a.rejectCompletion.setReason}
          valid={a.rejectCompletion.valid}
          busy={a.pending.rejectCompletion}
          onConfirm={a.handleRejectCompletion}
          onDismiss={a.rejectCompletion.close}
        />
      )}

      {a.reject.open && (
        <ReasonModal
          title="Tolak Vendor Request"
          reasonLabel="Alasan Penolakan"
          confirmLabel="Tolak Request"
          busyLabel="Menolak..."
          reason={a.reject.reason}
          onReasonChange={a.reject.setReason}
          valid={a.reject.valid}
          busy={a.pending.reject}
          onConfirm={a.handleReject}
          onDismiss={a.reject.close}
        />
      )}

      {a.vendorReturn.open && (
        <ReasonModal
          title="Kembalikan ke Pembuat"
          reasonLabel="Alasan (dibaca pembuat request)"
          confirmLabel="Kembalikan"
          busyLabel="Mengembalikan..."
          reason={a.vendorReturn.reason}
          onReasonChange={a.vendorReturn.setReason}
          valid={a.vendorReturn.valid}
          busy={a.pending.vendorReturn}
          onConfirm={a.handleVendorReturn}
          onDismiss={a.vendorReturn.close}
        />
      )}

      {a.cancel.open && (
        <ReasonModal
          title="Batalkan Vendor Request"
          reasonLabel="Alasan Pembatalan"
          confirmLabel="Batalkan Request"
          busyLabel="Membatalkan..."
          reason={a.cancel.reason}
          onReasonChange={a.cancel.setReason}
          valid={a.cancel.valid}
          busy={a.pending.cancel}
          onConfirm={a.handleCancel}
          onDismiss={a.cancel.close}
        />
      )}
    </>
  );
}
