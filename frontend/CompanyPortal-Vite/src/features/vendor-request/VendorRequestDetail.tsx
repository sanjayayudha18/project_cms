/**
 * Vendor Request detail page (Req 14): header (nulls omitted), items table
 * + total, status badge, and state×role×creator-gated actions mirroring the
 * backend's checkActor (vendor_request.go) — including its union cancel
 * rule (creator OR non-creator checker may cancel a pending request).
 */

import { Button } from "@/components/ui/Button";
import { PageHeader } from "@/components/ui/PageHeader";
import { formatAtmDateTime } from "@/features/atm-portal/lib/formatters";
import { useAuthStore } from "@/lib/auth/store";
import { useToast } from "@/lib/hooks/useToast";
import { formatIDR } from "@/lib/utils/formatCurrency";
import { Link, useParams } from "@tanstack/react-router";
import { AlertCircle, Ban, XCircle } from "lucide-react";
import { useState } from "react";
import { StatusBadge } from "./StatusBadge";
import {
  getVendorRequestErrorMessage,
  useApproveVendorRequest,
  useCancelVendorRequest,
  useRejectVendorRequest,
  useReviseVendorRequest,
  useSubmitVendorRequest,
  useUpdateVendorRequestItems,
  useVendorRequest,
} from "./hooks";
import type { VendorRequestDetail as VendorRequestDetailType, VendorRequestItem } from "./types";

const MAKER_ROLES = ["ADMIN", "ATM-USER", "BRANCH-ATM-USER"];
const CHECKER_ROLES = ["ADMIN", "ATM-SPV", "BRANCH-ATM-SPV"];
const ERROR_TOAST_MS = 5000;
const REJECTION_REASON_MAX = 500;
const AMOUNT_MIN = 1;

export function VendorRequestDetail() {
  const routeParams = useParams({ strict: false }) as { id?: string };
  const id = routeParams.id ? Number(routeParams.id) : null;
  const user = useAuthStore((s) => s.user);
  const { toast, dismiss } = useToast();
  const { data, isLoading, isError, error } = useVendorRequest(id);

  const [editingItems, setEditingItems] = useState<VendorRequestItem[] | null>(null);
  const [rejectOpen, setRejectOpen] = useState(false);
  const [rejectReason, setRejectReason] = useState("");
  const [cancelOpen, setCancelOpen] = useState(false);
  const [cancelReason, setCancelReason] = useState("");

  const updateItemsMutation = useUpdateVendorRequestItems();
  const submitMutation = useSubmitVendorRequest();
  const approveMutation = useApproveVendorRequest();
  const rejectMutation = useRejectVendorRequest();
  const reviseMutation = useReviseVendorRequest();
  const cancelMutation = useCancelVendorRequest();

  const isBusy =
    updateItemsMutation.isPending ||
    submitMutation.isPending ||
    approveMutation.isPending ||
    rejectMutation.isPending ||
    reviseMutation.isPending ||
    cancelMutation.isPending;

  function showErrorToast(message: string): void {
    const toastId = toast({ type: "error", message });
    setTimeout(() => dismiss(toastId), ERROR_TOAST_MS);
  }

  async function runAction(action: () => Promise<unknown>): Promise<void> {
    try {
      await action();
    } catch (err) {
      showErrorToast(getVendorRequestErrorMessage(err));
    }
  }

  if (id === null || Number.isNaN(id)) {
    return <NotFoundState />;
  }

  if (isLoading) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader eyebrow="Replenishment" title="Vendor Request" />
        <DetailSkeleton />
      </div>
    );
  }

  if (isError) {
    if (error?.status === 404) return <NotFoundState />;
    return (
      <div className="flex flex-col gap-6">
        <PageHeader eyebrow="Replenishment" title="Vendor Request" />
        <div className="flex flex-col items-center gap-3 rounded-[var(--radius-lg)] border border-[var(--n-200)] py-12">
          <AlertCircle className="h-10 w-10 text-[var(--danger-fg)]" aria-hidden="true" />
          <p className="text-sm text-[var(--n-700)]">Gagal memuat detail vendor request</p>
        </div>
      </div>
    );
  }

  if (!data) return <NotFoundState />;

  const role = user?.role;
  const isCreator = user !== null && user !== undefined && data.created_by.id === user.id;
  const isMaker = role ? MAKER_ROLES.includes(role) : false;
  const isChecker = role ? CHECKER_ROLES.includes(role) : false;

  const canEditItems = data.status === "draft" && isCreator && isMaker;
  const canSubmit = data.status === "draft" && isCreator && isMaker;
  const canApproveReject = data.status === "pending_approval" && isChecker && !isCreator;
  const canRevise = data.status === "rejected" && isCreator && isMaker;
  // CIT-2 (Task 13.3, Req 5.3): union rule for draft/pending_approval —
  // creator OR a non-creator checker may cancel. replenishment-request-
  // enhancements (Req 3.1, 3.7) adds a second, stricter arm: an approved
  // request is cancelable too, but Checker-only (no four-eyes here — direct
  // cancellation, not an approve/reject pair — so a Checker who is also the
  // creator is still allowed on this arm, mirroring the backend's checkActor).
  const canCancel =
    ((isCreator || (isChecker && !isCreator)) &&
      (data.status === "draft" || data.status === "pending_approval")) ||
    (isChecker && data.status === "approved");

  function startEditItems(): void {
    if (!data) return;
    setEditingItems(data.items.map((it) => ({ ...it })));
  }

  function updateEditedAmount(itemId: number, value: number): void {
    setEditingItems((cur) =>
      cur ? cur.map((it) => (it.id === itemId ? { ...it, amount_replenish: value } : it)) : cur,
    );
  }

  async function saveEditedItems(): Promise<void> {
    if (!editingItems || !data) return;
    await runAction(async () => {
      await updateItemsMutation.mutateAsync({
        id: data.id,
        items: editingItems.map((it) => ({
          terminal_id: it.terminal_id,
          periode_pred: it.periode_pred,
          denom: it.denom,
          amount_replenish: it.amount_replenish,
        })),
      });
      setEditingItems(null);
    });
  }

  async function handleReject(): Promise<void> {
    if (!data) return;
    const reason = rejectReason.trim();
    if (reason.length < 1 || reason.length > REJECTION_REASON_MAX) return;
    await runAction(async () => {
      await rejectMutation.mutateAsync({ id: data.id, reason });
      setRejectOpen(false);
      setRejectReason("");
    });
  }

  // replenishment-request-enhancements (Req 3.2, 3.3): same 1-500
  // non-whitespace bound as reject, required before the mutation fires.
  async function handleCancel(): Promise<void> {
    if (!data) return;
    const reason = cancelReason.trim();
    if (reason.length < 1 || reason.length > REJECTION_REASON_MAX) return;
    await runAction(async () => {
      await cancelMutation.mutateAsync({ id: data.id, reason });
      setCancelOpen(false);
      setCancelReason("");
    });
  }

  const editValid =
    editingItems?.every(
      (it) => Number.isInteger(it.amount_replenish) && it.amount_replenish >= AMOUNT_MIN,
    ) ?? false;
  const rejectValid =
    rejectReason.trim().length >= 1 && rejectReason.trim().length <= REJECTION_REASON_MAX;
  const cancelValid =
    cancelReason.trim().length >= 1 && cancelReason.trim().length <= REJECTION_REASON_MAX;

  const items = editingItems ?? data.items;

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Replenishment"
        title={data.request_number}
        description="Detail pengajuan vendor request"
        actions={
          <div className="flex flex-wrap gap-2">
            <StatusBadge status={data.status} />
            {data.is_canceled && <StatusBadge status={data.status} isCanceled />}
          </div>
        }
      />

      <Link
        to="/replenishment/vendor-requests"
        className="text-sm font-medium text-[var(--red-600)] hover:underline"
      >
        ← Kembali ke Daftar Vendor Request
      </Link>

      <DetailFields detail={data} />

      <div className="overflow-x-auto rounded-[var(--radius-lg)] border border-[var(--n-200)]">
        <table className="w-full min-w-[640px] border-collapse text-sm">
          <thead>
            <tr className="border-[var(--n-200)] border-b bg-[var(--n-50)]">
              <th className="px-3 py-2 text-left font-medium text-[var(--n-600)]">ATM ID</th>
              <th className="px-3 py-2 text-right font-medium text-[var(--n-600)]">Denominasi</th>
              <th className="px-3 py-2 text-right font-medium text-[var(--n-600)]">
                Amount Replenish
              </th>
            </tr>
          </thead>
          <tbody>
            {items.map((item) => (
              <tr key={item.id} className="border-[var(--n-100)] border-b last:border-0">
                <td className="px-3 py-2">{item.terminal_id}</td>
                <td className="px-3 py-2 text-right tabular-nums">{formatIDR(item.denom)}</td>
                <td className="px-3 py-2 text-right tabular-nums">
                  {editingItems ? (
                    <input
                      type="number"
                      aria-label={`Amount replenish untuk ${item.terminal_id}`}
                      value={item.amount_replenish}
                      onChange={(e) => updateEditedAmount(item.id, Number(e.target.value))}
                      className="w-36 rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-2 py-1 text-right text-sm tabular-nums outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
                    />
                  ) : (
                    formatIDR(item.amount_replenish)
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="flex flex-wrap items-center justify-between gap-4 rounded-[var(--radius-lg)] border border-[var(--n-200)] bg-[var(--n-50)] px-4 py-3">
        <div className="text-sm text-[var(--n-700)]">
          <span className="font-semibold text-[var(--n-900)]">{items.length}</span> item — Total:{" "}
          <span className="font-semibold tabular-nums text-[var(--n-900)]">
            Rp {formatIDR(data.total_amount)}
          </span>
        </div>

        <div className="flex flex-wrap gap-3">
          {editingItems ? (
            <>
              <Button variant="secondary" disabled={isBusy} onClick={() => setEditingItems(null)}>
                Batal
              </Button>
              <Button disabled={isBusy || !editValid} onClick={saveEditedItems}>
                {updateItemsMutation.isPending ? "Menyimpan..." : "Simpan Item"}
              </Button>
            </>
          ) : (
            <>
              {canEditItems && (
                <Button variant="secondary" disabled={isBusy} onClick={startEditItems}>
                  Edit Item
                </Button>
              )}
              {canSubmit && (
                <Button
                  disabled={isBusy}
                  onClick={() => runAction(() => submitMutation.mutateAsync(data.id))}
                >
                  {submitMutation.isPending ? "Mengirim..." : "Kirim untuk Approval"}
                </Button>
              )}
              {canApproveReject && (
                <>
                  <Button variant="danger" disabled={isBusy} onClick={() => setRejectOpen(true)}>
                    <XCircle className="mr-1.5 h-4 w-4" aria-hidden="true" />
                    Tolak
                  </Button>
                  <Button
                    disabled={isBusy}
                    onClick={() => runAction(() => approveMutation.mutateAsync(data.id))}
                  >
                    {approveMutation.isPending ? "Menyetujui..." : "Setujui"}
                  </Button>
                </>
              )}
              {canRevise && (
                <Button
                  disabled={isBusy}
                  onClick={() => runAction(() => reviseMutation.mutateAsync(data.id))}
                >
                  {reviseMutation.isPending ? "Merevisi..." : "Revisi"}
                </Button>
              )}
              {canCancel && (
                <Button variant="danger" disabled={isBusy} onClick={() => setCancelOpen(true)}>
                  <Ban className="mr-1.5 h-4 w-4" aria-hidden="true" />
                  Batalkan
                </Button>
              )}
            </>
          )}
        </div>
      </div>

      {rejectOpen && (
        <ReasonModal
          title="Tolak Vendor Request"
          reasonLabel="Alasan Penolakan"
          confirmLabel="Tolak Request"
          busyLabel="Menolak..."
          reason={rejectReason}
          onReasonChange={setRejectReason}
          valid={rejectValid}
          busy={rejectMutation.isPending}
          onConfirm={handleReject}
          onDismiss={() => {
            setRejectOpen(false);
            setRejectReason("");
          }}
        />
      )}

      {cancelOpen && (
        <ReasonModal
          title="Batalkan Vendor Request"
          reasonLabel="Alasan Pembatalan"
          confirmLabel="Batalkan Request"
          busyLabel="Membatalkan..."
          reason={cancelReason}
          onReasonChange={setCancelReason}
          valid={cancelValid}
          busy={cancelMutation.isPending}
          onConfirm={handleCancel}
          onDismiss={() => {
            setCancelOpen(false);
            setCancelReason("");
          }}
        />
      )}
    </div>
  );
}

const CATEGORY_LABELS: Record<string, string> = {
  planned: "Planned",
  emergency: "Emergency",
  additional: "Additional",
};

function DetailFields({ detail }: { detail: VendorRequestDetailType }) {
  const alwaysShown: Array<[string, string]> = [
    ["Tanggal Forecast", detail.forecast_date],
    ["Tanggal Replenish", detail.replenish_date ?? "-"],
    ["Kategori", detail.request_category ? (CATEGORY_LABELS[detail.request_category] ?? "-") : "-"],
  ];
  const fields: Array<[string, string | null]> = [
    ["Dibuat Oleh", detail.created_by.full_name],
    ["Dibuat Pada", formatAtmDateTime(new Date(detail.created_at))],
    detail.submitted_at ? ["Dikirim Pada", formatAtmDateTime(new Date(detail.submitted_at))] : null,
    detail.approved_by ? ["Disetujui Oleh", detail.approved_by.full_name] : null,
    detail.approved_at ? ["Disetujui Pada", formatAtmDateTime(new Date(detail.approved_at))] : null,
    detail.rejected_by ? ["Ditolak Oleh", detail.rejected_by.full_name] : null,
    detail.rejected_at ? ["Ditolak Pada", formatAtmDateTime(new Date(detail.rejected_at))] : null,
    detail.rejection_reason ? ["Alasan Penolakan", detail.rejection_reason] : null,
    // replenishment-request-enhancements (Req 3.11 Opsi B): populated from
    // the response now that migration 038 persists it on vendor_requests.
    detail.cancellation_reason ? ["Alasan Pembatalan", detail.cancellation_reason] : null,
    detail.notes ? ["Catatan", detail.notes] : null,
  ].filter((f): f is [string, string] => f !== null);
  const allFields = [...alwaysShown, ...fields];

  return (
    <div className="grid grid-cols-2 gap-x-6 gap-y-3 rounded-[var(--radius-lg)] border border-[var(--n-200)] bg-[var(--n-0)] p-4 sm:grid-cols-3">
      {allFields.map(([label, value]) => (
        <div key={label} className="flex flex-col gap-0.5">
          <span className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]">
            {label}
          </span>
          <span className="text-sm text-[var(--n-800)]">{value}</span>
        </div>
      ))}
    </div>
  );
}

interface ReasonModalProps {
  title: string;
  reasonLabel: string;
  confirmLabel: string;
  busyLabel: string;
  reason: string;
  onReasonChange: (value: string) => void;
  valid: boolean;
  busy: boolean;
  onConfirm: () => void;
  onDismiss: () => void;
}

/**
 * Shared 1-500-non-whitespace-char reason modal for both Reject (Req 2.3,
 * 2.4) and Cancel (Req 3.2, 3.3) — same shape, different copy, so one
 * component instead of two near-identical ones (was RejectModal).
 */
function ReasonModal({
  title,
  reasonLabel,
  confirmLabel,
  busyLabel,
  reason,
  onReasonChange,
  valid,
  busy,
  onConfirm,
  onDismiss,
}: ReasonModalProps) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4">
      <div className="flex w-full max-w-md flex-col gap-4 rounded-[var(--radius-lg)] bg-[var(--n-0)] p-6 shadow-[var(--shadow-md)]">
        <h2 className="text-lg font-semibold text-[var(--n-900)]">{title}</h2>
        <div className="flex flex-col gap-1">
          <label htmlFor="reason-modal-input" className="text-sm text-[var(--n-700)]">
            {reasonLabel}
          </label>
          <textarea
            id="reason-modal-input"
            value={reason}
            onChange={(e) => onReasonChange(e.target.value.slice(0, REJECTION_REASON_MAX))}
            maxLength={REJECTION_REASON_MAX}
            rows={4}
            // biome-ignore lint/a11y/noAutofocus: modal opens on explicit user action; focusing the only input is the expected behavior
            autoFocus
            className="rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 py-2 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
          />
          <span className="text-xs text-[var(--n-500)]">
            {reason.trim().length}/{REJECTION_REASON_MAX}
          </span>
        </div>
        <div className="flex justify-end gap-3">
          <Button variant="secondary" disabled={busy} onClick={onDismiss}>
            Batal
          </Button>
          <Button variant="danger" disabled={busy || !valid} onClick={onConfirm}>
            {busy ? busyLabel : confirmLabel}
          </Button>
        </div>
      </div>
    </div>
  );
}

function DetailSkeleton() {
  return (
    <div className="flex flex-col gap-4">
      <div className="h-24 animate-pulse rounded-[var(--radius-lg)] bg-[var(--n-100)]" />
      <div className="h-48 animate-pulse rounded-[var(--radius-lg)] bg-[var(--n-100)]" />
    </div>
  );
}

function NotFoundState() {
  return (
    <div className="flex flex-col gap-6">
      <PageHeader eyebrow="Replenishment" title="Vendor Request Tidak Ditemukan" />
      <div className="rounded-[var(--radius-lg)] border border-[var(--n-200)] bg-[var(--n-50)] p-8 text-center">
        <p className="text-sm text-[var(--n-700)]">
          Vendor request yang Anda cari tidak ditemukan atau telah dihapus.
        </p>
        <Link
          to="/replenishment/vendor-requests"
          className="mt-4 inline-block text-sm font-medium text-[var(--red-600)] hover:underline"
        >
          Kembali ke Daftar Vendor Request
        </Link>
      </div>
    </div>
  );
}
