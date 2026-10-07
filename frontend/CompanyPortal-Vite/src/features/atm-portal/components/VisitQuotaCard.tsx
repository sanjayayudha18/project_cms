/**
 * "Kuota Kunjungan" card on the ATM profile (.claude/sdlc/atm-visit-quota
 * spec FR6.5): paket, kuota, sisa, kelebihan, last reset, visits of the
 * current period. Checkers (SPV/ADMIN) can reset the kuota to the active
 * package's cr_frequency and cancel a wrongly counted visit (reason required).
 * Hidden for roles the endpoint refuses (403).
 */

import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { ReasonModal } from "@/features/vendor-request/VendorRequestDetail";
import { useAuthStore } from "@/lib/auth/store";
import { useToast } from "@/lib/hooks/useToast";
import { AlertTriangle, Ban, RotateCcw } from "lucide-react";
import { useState } from "react";
import { formatAtmDateTime } from "../lib/formatters";
import {
  type AtmVisit,
  type AtmVisitQuota,
  VISIT_QUOTA_CHECKER_ROLES,
  useAtmVisitQuota,
  useCancelAtmVisit,
  useResetAtmQuota,
} from "../visitQuota";

const REASON_MAX = 500;
const TOAST_MS = 5000;

function Stat({ label, value, emphasis }: { label: string; value: string; emphasis?: boolean }) {
  return (
    <div className="flex flex-col gap-0.5">
      <span className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]">
        {label}
      </span>
      <span
        className={`tabular-nums ${emphasis ? "text-2xl font-semibold text-[var(--n-900)]" : "text-sm text-[var(--n-800)]"}`}
      >
        {value}
      </span>
    </div>
  );
}

function VisitRow({
  visit,
  canCancel,
  onCancel,
}: {
  visit: AtmVisit;
  canCancel: boolean;
  onCancel: (visit: AtmVisit) => void;
}) {
  const cancelled = visit.cancelled_at !== null;
  return (
    <li className="flex flex-wrap items-center justify-between gap-2 py-2">
      <div className="flex flex-col">
        <span
          className={`font-mono text-sm ${cancelled ? "line-through text-[var(--n-500)]" : ""}`}
        >
          {visit.request_number}
        </span>
        <span className="text-xs text-[var(--n-600)]">
          {formatAtmDateTime(new Date(visit.created_at))}
          {cancelled && visit.cancel_reason ? ` — dibatalkan: ${visit.cancel_reason}` : ""}
        </span>
      </div>
      <div className="flex items-center gap-2">
        {visit.is_over_quota && !cancelled && (
          <Badge variant="warning" icon={AlertTriangle} label="Kelebihan kuota" />
        )}
        {cancelled && <Badge variant="neutral" icon={Ban} label="Dibatalkan" />}
        {canCancel && !cancelled && (
          <Button variant="secondary" onClick={() => onCancel(visit)}>
            Batalkan
          </Button>
        )}
      </div>
    </li>
  );
}

function resetLabel(quota: AtmVisitQuota): string {
  if (!quota.reset_at) return "—";
  const by = quota.reset_by ? quota.reset_by.full_name : "otomatis";
  return `${formatAtmDateTime(new Date(quota.reset_at))} · ${by}`;
}

function QuotaSummary({ quota }: { quota: AtmVisitQuota }) {
  if (!quota.has_quota) {
    return (
      <p className="text-sm text-[var(--n-700)]">
        Belum ada kunjungan tercatat. Kuota paket aktif:{" "}
        <span className="font-medium tabular-nums">
          {quota.current_quota ?? "—"}
          {quota.current_package_code ? ` (${quota.current_package_code})` : ""}
        </span>
      </p>
    );
  }
  return (
    <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
      <Stat label="Sisa Kunjungan" value={`${quota.sisa} / ${quota.quota_total}`} emphasis />
      <Stat label="Kelebihan" value={String(quota.kelebihan)} />
      <Stat label="Paket" value={quota.package_code} />
      <Stat label="Reset Terakhir" value={resetLabel(quota)} />
    </div>
  );
}

export function VisitQuotaCard({ terminalId }: { terminalId: string }) {
  const role = useAuthStore((s) => s.user?.role);
  const isChecker = role ? VISIT_QUOTA_CHECKER_ROLES.includes(role) : false;
  const { toast, dismiss } = useToast();
  const query = useAtmVisitQuota(terminalId);
  const resetMutation = useResetAtmQuota();
  const cancelMutation = useCancelAtmVisit();
  const [confirmReset, setConfirmReset] = useState(false);
  const [cancelTarget, setCancelTarget] = useState<AtmVisit | null>(null);
  const [cancelReason, setCancelReason] = useState("");

  function notify(type: "success" | "error", message: string): void {
    const id = toast({ type, message });
    setTimeout(() => dismiss(id), TOAST_MS);
  }

  if (query.isError && query.error?.status === 403) return null;

  const quota = query.data;
  const canReset = isChecker && quota !== undefined && quota.current_quota !== null;

  async function handleReset(): Promise<void> {
    try {
      await resetMutation.mutateAsync(terminalId);
      notify("success", "Kuota kunjungan di-reset");
    } catch (err) {
      notify("error", err instanceof Error ? err.message : "Gagal me-reset kuota");
    } finally {
      setConfirmReset(false);
    }
  }

  async function handleCancelVisit(): Promise<void> {
    if (!cancelTarget) return;
    try {
      await cancelMutation.mutateAsync({ visitId: cancelTarget.id, reason: cancelReason.trim() });
      notify("success", "Kunjungan dibatalkan, kuota dikembalikan");
      setCancelTarget(null);
      setCancelReason("");
    } catch (err) {
      notify("error", err instanceof Error ? err.message : "Gagal membatalkan kunjungan");
    }
  }

  const reasonLength = cancelReason.trim().length;

  return (
    <section
      aria-labelledby="visit-quota-heading"
      className="flex flex-col gap-4 rounded-[var(--radius-lg)] border border-[var(--n-200)] bg-[var(--n-0)] p-4"
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 id="visit-quota-heading" className="text-sm font-semibold text-[var(--n-900)]">
          Kuota Kunjungan Replenish
        </h2>
        {canReset &&
          (confirmReset ? (
            <div className="flex items-center gap-2 text-sm">
              <span>Reset sisa ke {quota.current_quota}?</span>
              <Button variant="secondary" onClick={() => setConfirmReset(false)}>
                Batal
              </Button>
              <Button disabled={resetMutation.isPending} onClick={handleReset}>
                {resetMutation.isPending ? "Me-reset..." : "Ya, Reset"}
              </Button>
            </div>
          ) : (
            <Button variant="secondary" onClick={() => setConfirmReset(true)}>
              <RotateCcw className="mr-1.5 h-4 w-4" aria-hidden="true" />
              Reset Kuota
            </Button>
          ))}
      </div>

      {query.isLoading && (
        <div className="h-16 animate-pulse rounded-[var(--radius-md)] bg-[var(--n-100)]" />
      )}
      {query.isError && (
        <p className="text-sm text-[var(--danger-fg)]">Gagal memuat kuota kunjungan</p>
      )}

      {quota && (
        <>
          <QuotaSummary quota={quota} />
          {quota.current_quota === null && (
            <p className="flex items-center gap-1.5 text-sm text-[var(--n-700)]">
              <AlertTriangle className="h-4 w-4" aria-hidden="true" />
              Paket tidak dikenali — kuota tidak dapat dihitung atau di-reset.
            </p>
          )}
          {quota.visits.length > 0 && (
            <div>
              <h3 className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]">
                Kunjungan periode berjalan
              </h3>
              <ul className="divide-y divide-[var(--n-100)]">
                {quota.visits.map((v) => (
                  <VisitRow key={v.id} visit={v} canCancel={isChecker} onCancel={setCancelTarget} />
                ))}
              </ul>
            </div>
          )}
        </>
      )}

      {cancelTarget && (
        <ReasonModal
          title={`Batalkan Kunjungan ${cancelTarget.request_number}`}
          reasonLabel="Alasan Pembatalan Kunjungan"
          confirmLabel="Batalkan Kunjungan"
          busyLabel="Membatalkan..."
          reason={cancelReason}
          onReasonChange={setCancelReason}
          valid={reasonLength >= 1 && reasonLength <= REASON_MAX}
          busy={cancelMutation.isPending}
          onConfirm={handleCancelVisit}
          onDismiss={() => {
            setCancelTarget(null);
            setCancelReason("");
          }}
        />
      )}
    </section>
  );
}
