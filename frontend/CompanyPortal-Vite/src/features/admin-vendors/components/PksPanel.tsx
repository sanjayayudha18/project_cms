import { Button } from "@/components/ui/Button";
import { useState } from "react";
import { useVendorPks } from "../hooks";
import { formatMonitoring, formatValidity } from "../lib/pks";
import type { AdminVendor, VendorPksRecord } from "../types";
import { PksFormDialog } from "./PksFormDialog";
import { PksReasonDialog } from "./PksReasonDialog";
import { PksStatusBadge } from "./PksStatusBadge";

const PENDING_OP_LABEL = {
  create: "Catat PKS baru",
  disable: "Terminate PKS",
  enable: "Batalkan terminate",
} as const;

/** ISO yyyy-mm-dd (atau timestamp) -> dd/mm/yyyy tanpa konversi zona waktu. */
function fmtDate(iso: string | null | undefined): string {
  if (!iso) return "-";
  const [y, m, d] = iso.slice(0, 10).split("-");
  return `${d}/${m}/${y}`;
}

function payloadText(payload: Record<string, unknown>, key: string): string | null {
  const v = payload[key];
  return typeof v === "string" && v !== "" ? v : null;
}

const cardClass = "rounded-[var(--radius-lg)] border border-[var(--n-200)] bg-[var(--n-0)] p-4";
const thClass =
  "px-3 py-2 text-left text-xs font-medium uppercase tracking-wider text-[var(--n-600)]";
const tdClass = "px-3 py-2 text-sm text-[var(--n-800)]";

function HistoryTable({ rows }: { rows: VendorPksRecord[] }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full border-collapse">
        <caption className="sr-only">Riwayat PKS</caption>
        <thead>
          <tr className="border-b border-[var(--n-200)]">
            <th scope="col" className={thClass}>
              Nama PKS
            </th>
            <th scope="col" className={thClass}>
              Efektif
            </th>
            <th scope="col" className={thClass}>
              Expired
            </th>
            <th scope="col" className={thClass}>
              Masa berlaku
            </th>
            <th scope="col" className={thClass}>
              Digantikan
            </th>
            <th scope="col" className={thClass}>
              Terminate
            </th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.id} className="border-b border-[var(--n-100)]">
              <td className={tdClass}>{r.pks_name}</td>
              <td className={`${tdClass} tabular-nums`}>{fmtDate(r.effective_date)}</td>
              <td className={`${tdClass} tabular-nums`}>{fmtDate(r.expiry_date)}</td>
              <td className={tdClass}>{formatValidity(r.validity_months)}</td>
              <td className={`${tdClass} tabular-nums`}>{fmtDate(r.replaced_at)}</td>
              <td className={tdClass}>
                {r.terminated_at ? (
                  <>
                    <span className="tabular-nums">{fmtDate(r.terminated_at)}</span>
                    {r.termination_reason ? `: ${r.termination_reason}` : ""}
                  </>
                ) : (
                  "-"
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

/** Tab PKS: PKS aktif, perubahan yang menunggu persetujuan, riwayat, dan aksi maker-checker. */
export function PksPanel({
  vendorId,
  vendor,
}: {
  vendorId: number;
  vendor: Pick<AdminVendor, "is_active"> | undefined;
}) {
  const query = useVendorPks(vendorId);
  const [formOpen, setFormOpen] = useState(false);
  const [reasonMode, setReasonMode] = useState<"terminate" | "unterminate" | null>(null);

  if (query.isError) {
    return (
      <p role="alert" className="text-sm text-[var(--danger-fg)]">
        {query.error?.message || "PKS tidak dapat dimuat"}
      </p>
    );
  }
  if (query.isLoading || !query.data) {
    return <p className="text-sm text-[var(--n-500)]">Memuat PKS...</p>;
  }

  const { active, history, pending, pks_status, renewal_months } = query.data;
  const vendorInactive = vendor ? !vendor.is_active : false;
  const hasPending = pending !== null;
  const blocked = hasPending || vendorInactive;
  const isTerminated = pks_status === "terminated";
  const blockedReason = vendorInactive
    ? "Vendor nonaktif. Aktifkan vendor dulu."
    : hasPending
      ? "Masih ada perubahan PKS yang menunggu persetujuan."
      : null;

  const pendingName = pending ? payloadText(pending.payload, "pks_name") : null;
  const pendingReason = pending ? payloadText(pending.payload, "reason") : null;

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center gap-3">
        {(!active || !isTerminated) && (
          <Button type="button" disabled={blocked} onClick={() => setFormOpen(true)}>
            Catat PKS baru
          </Button>
        )}
        {active && !isTerminated && (
          <Button
            type="button"
            variant="secondary"
            disabled={blocked}
            onClick={() => setReasonMode("terminate")}
          >
            Terminate PKS
          </Button>
        )}
        {active && isTerminated && (
          <>
            <Button type="button" disabled={blocked} onClick={() => setFormOpen(true)}>
              Catat PKS baru
            </Button>
            <Button
              type="button"
              variant="secondary"
              disabled={blocked}
              onClick={() => setReasonMode("unterminate")}
            >
              Batalkan terminate
            </Button>
          </>
        )}
        {blockedReason && <span className="text-sm text-[var(--n-600)]">{blockedReason}</span>}
      </div>

      {pending && (
        <section aria-label="Menunggu persetujuan" className={cardClass}>
          <h3 className="text-sm font-semibold text-[var(--n-900)]">Menunggu persetujuan</h3>
          <dl className="mt-2 grid grid-cols-1 gap-x-6 gap-y-1 text-sm sm:grid-cols-2">
            <div>
              <dt className="text-xs uppercase tracking-wider text-[var(--n-600)]">Perubahan</dt>
              <dd className="text-[var(--n-800)]">{PENDING_OP_LABEL[pending.op]}</dd>
            </div>
            <div>
              <dt className="text-xs uppercase tracking-wider text-[var(--n-600)]">
                Diajukan oleh
              </dt>
              <dd className="text-[var(--n-800)]">{pending.maker_name}</dd>
            </div>
            {pendingName && (
              <div>
                <dt className="text-xs uppercase tracking-wider text-[var(--n-600)]">Nama PKS</dt>
                <dd className="text-[var(--n-800)]">{pendingName}</dd>
              </div>
            )}
            {pendingReason && (
              <div>
                <dt className="text-xs uppercase tracking-wider text-[var(--n-600)]">Alasan</dt>
                <dd className="text-[var(--n-800)]">{pendingReason}</dd>
              </div>
            )}
          </dl>
        </section>
      )}

      <section aria-label="PKS aktif" className={cardClass}>
        <div className="flex flex-wrap items-center gap-3">
          <h3 className="text-sm font-semibold text-[var(--n-900)]">PKS aktif</h3>
          <PksStatusBadge status={pks_status} />
        </div>
        {active ? (
          <dl className="mt-3 grid grid-cols-1 gap-x-6 gap-y-2 text-sm sm:grid-cols-2 lg:grid-cols-3">
            <div>
              <dt className="text-xs uppercase tracking-wider text-[var(--n-600)]">Nama PKS</dt>
              <dd className="text-[var(--n-800)]">{active.pks_name}</dd>
            </div>
            <div>
              <dt className="text-xs uppercase tracking-wider text-[var(--n-600)]">Efektif</dt>
              <dd className="tabular-nums text-[var(--n-800)]">{fmtDate(active.effective_date)}</dd>
            </div>
            <div>
              <dt className="text-xs uppercase tracking-wider text-[var(--n-600)]">Expired</dt>
              <dd className="tabular-nums text-[var(--n-800)]">{fmtDate(active.expiry_date)}</dd>
            </div>
            <div>
              <dt className="text-xs uppercase tracking-wider text-[var(--n-600)]">Masa berlaku</dt>
              <dd className="text-[var(--n-800)]">{formatValidity(active.validity_months)}</dd>
            </div>
            <div>
              <dt className="text-xs uppercase tracking-wider text-[var(--n-600)]">Monitoring</dt>
              <dd className="tabular-nums text-[var(--n-800)]">
                {formatMonitoring(pks_status, active.remaining_days)}
              </dd>
            </div>
            {isTerminated && (
              <div>
                <dt className="text-xs uppercase tracking-wider text-[var(--n-600)]">Terminate</dt>
                <dd className="text-[var(--n-800)]">
                  <span className="tabular-nums">{fmtDate(active.terminated_at)}</span>
                  {active.termination_reason ? `: ${active.termination_reason}` : ""}
                </dd>
              </div>
            )}
          </dl>
        ) : (
          <p className="mt-2 text-sm text-[var(--n-600)]">Belum ada PKS yang dicatat.</p>
        )}
        <p className="mt-3 text-xs text-[var(--n-500)]">
          Need Renewal mulai {renewal_months} bulan sebelum tanggal expired.
        </p>
      </section>

      <section aria-label="Riwayat PKS" className={cardClass}>
        <h3 className="mb-2 text-sm font-semibold text-[var(--n-900)]">Riwayat PKS</h3>
        {history.length > 0 ? (
          <HistoryTable rows={history} />
        ) : (
          <p className="text-sm text-[var(--n-600)]">Belum ada riwayat.</p>
        )}
      </section>

      <PksFormDialog
        open={formOpen}
        onClose={() => setFormOpen(false)}
        vendorId={vendorId}
        replacesActive={active !== null}
      />
      {active && reasonMode && (
        <PksReasonDialog
          open
          onClose={() => setReasonMode(null)}
          vendorId={vendorId}
          pksId={active.id}
          pksName={active.pks_name}
          mode={reasonMode}
        />
      )}
    </div>
  );
}
