/**
 * Presentational pieces of the Vendor Request detail page: field grid, shared
 * reason modal, skeleton and not-found states.
 */

import { Button } from "@/components/ui/Button";
import { PageHeader } from "@/components/ui/PageHeader";
import { formatAtmDateTime } from "@/features/atm-portal/lib/formatters";
import { Link } from "@tanstack/react-router";
import type { VendorRequestDetail as VendorRequestDetailType } from "./types";

export const REJECTION_REASON_MAX = 500;

const CATEGORY_LABELS: Record<string, string> = {
  planned: "Planned",
  emergency: "Emergency",
  additional: "Additional",
};

export function DetailFields({ detail }: { detail: VendorRequestDetailType }) {
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
    // atm-visit-quota: laporan selesai replenish.
    detail.completion_submitted_by
      ? ["Dilaporkan Selesai Oleh", detail.completion_submitted_by.full_name]
      : null,
    detail.completion_submitted_at
      ? ["Dilaporkan Pada", formatAtmDateTime(new Date(detail.completion_submitted_at))]
      : null,
    detail.completion_approved_by
      ? ["Laporan Disetujui Oleh", detail.completion_approved_by.full_name]
      : null,
    detail.completion_rejection_reason
      ? ["Alasan Penolakan Laporan", detail.completion_rejection_reason]
      : null,
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
export function ReasonModal({
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

export function DetailSkeleton() {
  return (
    <div className="flex flex-col gap-4">
      <div className="h-24 animate-pulse rounded-[var(--radius-lg)] bg-[var(--n-100)]" />
      <div className="h-48 animate-pulse rounded-[var(--radius-lg)] bg-[var(--n-100)]" />
    </div>
  );
}

export function NotFoundState() {
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
