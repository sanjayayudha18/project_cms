/**
 * Laporan selesai replenish UI for the Vendor Request detail page
 * (.claude/sdlc/atm-visit-quota spec FR6.1-6.3): per-ATM status table with
 * sisa kunjungan + kelebihan-kuota badge, the maker's "Laporkan selesai"
 * dialog, and the checker's approve dialog with the over-quota warning.
 * Status is always text + icon, never colour alone (Sec 13).
 */

import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { visitSisa } from "@/features/atm-portal/visitQuota";
import { AlertTriangle, CheckCircle2, XCircle } from "lucide-react";
import type { ReactNode } from "react";
import type { CompletionResult, CompletionResultInput, RequestAtmStatus } from "./types";

function SisaCell({ atm }: { atm: RequestAtmStatus }) {
  if (atm.visit_remaining === null || atm.visit_quota_total === null) {
    return <span className="text-[var(--n-500)]">-</span>;
  }
  const { sisa, kelebihan } = visitSisa(atm.visit_remaining);
  return (
    <span className="tabular-nums">
      {sisa}/{atm.visit_quota_total}
      {kelebihan > 0 && <span className="ml-1 text-[var(--n-600)]">(+{kelebihan})</span>}
    </span>
  );
}

function ResultBadge({ result }: { result: CompletionResult | null }) {
  if (result === "success") return <Badge variant="success" icon={CheckCircle2} label="Berhasil" />;
  if (result === "failed") return <Badge variant="neutral" icon={XCircle} label="Gagal" />;
  return <span className="text-[var(--n-500)]">-</span>;
}

export function CompletionAtmTable({ atms }: { atms: RequestAtmStatus[] }) {
  return (
    <section aria-labelledby="completion-atm-heading" className="flex flex-col gap-2">
      <h2 id="completion-atm-heading" className="text-sm font-semibold text-[var(--n-900)]">
        Laporan Selesai &amp; Kuota Kunjungan
      </h2>
      <div className="overflow-x-auto rounded-[var(--radius-lg)] border border-[var(--n-200)]">
        <table className="w-full min-w-[520px] border-collapse text-sm">
          <thead>
            <tr className="border-[var(--n-200)] border-b bg-[var(--n-50)]">
              <th className="px-3 py-2 text-left font-medium text-[var(--n-600)]">ATM ID</th>
              <th className="px-3 py-2 text-left font-medium text-[var(--n-600)]">No. Tiket</th>
              <th className="px-3 py-2 text-left font-medium text-[var(--n-600)]">Hasil</th>
              <th className="px-3 py-2 text-right font-medium text-[var(--n-600)]">
                Sisa Kunjungan
              </th>
              <th className="px-3 py-2 text-left font-medium text-[var(--n-600)]">Keterangan</th>
            </tr>
          </thead>
          <tbody>
            {atms.map((atm) => (
              <tr key={atm.terminal_id} className="border-[var(--n-100)] border-b last:border-0">
                <td className="px-3 py-2 font-mono">{atm.terminal_id}</td>
                <td className="px-3 py-2 font-mono">{atm.ticket_number || "-"}</td>
                <td className="px-3 py-2">
                  <ResultBadge result={atm.completion_result} />
                </td>
                <td className="px-3 py-2 text-right">
                  <SisaCell atm={atm} />
                </td>
                <td className="px-3 py-2">
                  {atm.is_over_quota && (
                    <Badge variant="warning" icon={AlertTriangle} label="Kelebihan kuota" />
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

interface ModalShellProps {
  title: string;
  children: ReactNode;
  footer: ReactNode;
}

function ModalShell({ title, children, footer }: ModalShellProps) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4">
      <dialog
        open
        aria-modal="true"
        aria-label={title}
        className="static m-0 flex max-h-[90vh] w-full max-w-lg flex-col gap-4 rounded-[var(--radius-lg)] border-0 bg-[var(--n-0)] p-6 text-[var(--n-800)] shadow-[var(--shadow-md)]"
      >
        <h2 className="text-lg font-semibold text-[var(--n-900)]">{title}</h2>
        <div className="min-h-0 overflow-y-auto">{children}</div>
        <div className="flex justify-end gap-3">{footer}</div>
      </dialog>
    </div>
  );
}

interface ReportCompletionModalProps {
  atms: RequestAtmStatus[];
  results: Record<string, CompletionResult>;
  onResultChange: (terminalId: string, result: CompletionResult) => void;
  busy: boolean;
  onConfirm: () => void;
  onDismiss: () => void;
}

/** Maker: every ATM defaults to "Berhasil"; toggle the ones that failed (FR6.1). */
export function ReportCompletionModal({
  atms,
  results,
  onResultChange,
  busy,
  onConfirm,
  onDismiss,
}: ReportCompletionModalProps) {
  const failedCount = atms.filter((a) => results[a.terminal_id] === "failed").length;
  return (
    <ModalShell
      title="Laporkan Selesai Replenish"
      footer={
        <>
          <Button variant="secondary" disabled={busy} onClick={onDismiss}>
            Batal
          </Button>
          <Button disabled={busy} onClick={onConfirm}>
            {busy ? "Mengirim..." : "Kirim Laporan"}
          </Button>
        </>
      }
    >
      <p className="mb-3 text-sm text-[var(--n-700)]">
        Tandai ATM yang gagal diisi. Hanya ATM berhasil yang mengurangi kuota kunjungan setelah
        disetujui SPV. {atms.length - failedCount} berhasil, {failedCount} gagal.
      </p>
      <ul className="flex flex-col divide-y divide-[var(--n-100)]">
        {atms.map((atm) => (
          <li key={atm.terminal_id} className="flex items-center justify-between gap-3 py-2">
            <span className="font-mono text-sm">{atm.terminal_id}</span>
            <fieldset className="flex gap-3 text-sm">
              <legend className="sr-only">Hasil replenish {atm.terminal_id}</legend>
              {(["success", "failed"] as const).map((r) => (
                <label key={r} className="flex items-center gap-1.5">
                  <input
                    type="radio"
                    name={`result-${atm.terminal_id}`}
                    checked={(results[atm.terminal_id] ?? "success") === r}
                    onChange={() => onResultChange(atm.terminal_id, r)}
                  />
                  {r === "success" ? "Berhasil" : "Gagal"}
                </label>
              ))}
            </fieldset>
          </li>
        ))}
      </ul>
    </ModalShell>
  );
}

/** ATMs reported "success" whose sisa is already 0 or below: approving goes over quota. */
export function atmsGoingOverQuota(atms: RequestAtmStatus[]): string[] {
  return atms
    .filter(
      (a) =>
        a.completion_result === "success" && a.visit_remaining !== null && a.visit_remaining <= 0,
    )
    .map((a) => a.terminal_id);
}

interface ApproveCompletionModalProps {
  atms: RequestAtmStatus[];
  busy: boolean;
  onConfirm: () => void;
  onDismiss: () => void;
}

/** Checker: shows which ATMs will exceed their kuota; approval is still allowed (FR6.2). */
export function ApproveCompletionModal({
  atms,
  busy,
  onConfirm,
  onDismiss,
}: ApproveCompletionModalProps) {
  const over = atmsGoingOverQuota(atms);
  const successCount = atms.filter((a) => a.completion_result === "success").length;
  return (
    <ModalShell
      title="Setujui Laporan Selesai"
      footer={
        <>
          <Button variant="secondary" disabled={busy} onClick={onDismiss}>
            Batal
          </Button>
          <Button disabled={busy} onClick={onConfirm}>
            {busy ? "Menyetujui..." : "Setujui Laporan"}
          </Button>
        </>
      }
    >
      <p className="text-sm text-[var(--n-700)]">
        {successCount} ATM berhasil akan mengurangi kuota kunjungan masing-masing 1.
      </p>
      {over.length > 0 && (
        <div
          role="alert"
          className="mt-3 flex gap-2 rounded-[var(--radius-md)] bg-[var(--warning-bg)] p-3 text-sm text-[var(--warning-fg)]"
        >
          <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
          <div>
            <p className="font-medium">
              {over.length} ATM akan melebihi kuota kunjungan dan ditandai "Kelebihan kuota":
            </p>
            <p className="mt-1 font-mono">{over.join(", ")}</p>
          </div>
        </div>
      )}
    </ModalShell>
  );
}

/** Builds the submit payload from the dialog state (every ATM present, default success). */
export function toCompletionPayload(
  atms: RequestAtmStatus[],
  results: Record<string, CompletionResult>,
): CompletionResultInput[] {
  return atms.map((a) => ({
    terminal_id: a.terminal_id,
    result: results[a.terminal_id] ?? "success",
  }));
}
