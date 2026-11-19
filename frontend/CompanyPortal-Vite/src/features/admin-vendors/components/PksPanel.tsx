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

function HistoryTable({
  rows,
  caption = "Riwayat PKS",
}: {
  rows: VendorPksRecord[];
  caption?: string;
}) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full border-collapse">
        <caption className="sr-only">{caption}</caption>
        <thead>
          <tr className="border-b border-[var(--n-200)]">
            <th scope="col" className={thClass}>
              No. WOD
            </th>
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
              <td className={`${tdClass} tabular-nums font-medium`}>{r.wod_number}</td>
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

interface FormState {
  open: boolean;
  initialTypeCode?: string;
  replacesActive?: boolean;
}

interface ReasonState {
  mode: "terminate" | "unterminate";
  pksId: number;
  pksName: string;
}

/** Tab PKS: satu kartu per type (aktif + WOD, riwayat, pending) dan aksi maker-checker. */
export function PksPanel({
  vendorId,
  vendor,
}: {
  vendorId: number;
  vendor: Pick<AdminVendor, "is_active"> | undefined;
}) {
  const query = useVendorPks(vendorId);
  const [formState, setFormState] = useState<FormState>({ open: false });
  const [reasonState, setReasonState] = useState<ReasonState | null>(null);

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

  const { groups = [], renewal_months } = query.data;
  const vendorInactive = vendor ? !vendor.is_active : false;

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="text-base font-semibold text-[var(--n-900)]">PKS Vendor</h2>
          <p className="text-xs text-[var(--n-500)]">
            Need Renewal mulai {renewal_months} bulan sebelum tanggal expired.
          </p>
        </div>
        <div className="flex items-center gap-3">
          <Button
            type="button"
            disabled={vendorInactive}
            onClick={() => setFormState({ open: true })}
          >
            Catat PKS baru
          </Button>
          {vendorInactive && (
            <span className="text-sm text-[var(--n-600)]">
              Vendor nonaktif. Aktifkan vendor dulu.
            </span>
          )}
        </div>
      </div>

      {groups.length === 0 ? (
        <section aria-label="PKS aktif" className={cardClass}>
          <p className="text-sm text-[var(--n-600)]">Belum ada PKS yang dicatat.</p>
          <p className="mt-2 text-sm text-[var(--n-600)]">Belum ada riwayat.</p>
        </section>
      ) : (
        groups.map((group) => {
          const typeCode = group.pks_type.code;
          const typeName = group.pks_type.name;
          const hasPending = group.pending !== null;
          const isTerminated = group.pks_status === "terminated";
          const typeBlocked = hasPending || vendorInactive;
          const blockedReason = hasPending
            ? "Masih ada perubahan PKS yang menunggu persetujuan."
            : null;

          const pending = group.pending;
          const pendingWod = pending ? payloadText(pending.payload, "wod_number") : null;
          const pendingName = pending ? payloadText(pending.payload, "pks_name") : null;
          const pendingReason = pending ? payloadText(pending.payload, "reason") : null;

          const activeAriaLabel = groups.length === 1 ? "PKS aktif" : `PKS aktif ${typeCode}`;
          const pendingAriaLabel =
            groups.length === 1 ? "Menunggu persetujuan" : `Menunggu persetujuan ${typeCode}`;
          const historyAriaLabel = groups.length === 1 ? "Riwayat PKS" : `Riwayat PKS ${typeCode}`;
          const historyCaption = groups.length === 1 ? "Riwayat PKS" : `Riwayat PKS ${typeCode}`;

          return (
            <article
              key={typeCode}
              aria-label={`PKS ${typeCode}`}
              className={`${cardClass} flex flex-col gap-4`}
            >
              <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--n-200)] pb-3">
                <h3 className="text-base font-semibold text-[var(--n-900)]">
                  Type PKS: <span className="tabular-nums font-bold">{typeCode}</span> - {typeName}
                </h3>
                <div className="flex flex-wrap items-center gap-2">
                  {group.active && !isTerminated && (
                    <Button
                      type="button"
                      variant="secondary"
                      disabled={typeBlocked}
                      onClick={() => {
                        const rec = group.active;
                        if (rec) {
                          setReasonState({
                            mode: "terminate",
                            pksId: rec.id,
                            pksName: rec.pks_name,
                          });
                        }
                      }}
                    >
                      Terminate PKS
                    </Button>
                  )}
                  {group.active && isTerminated && (
                    <Button
                      type="button"
                      variant="secondary"
                      disabled={typeBlocked}
                      onClick={() => {
                        const rec = group.active;
                        if (rec) {
                          setReasonState({
                            mode: "unterminate",
                            pksId: rec.id,
                            pksName: rec.pks_name,
                          });
                        }
                      }}
                    >
                      Batalkan terminate
                    </Button>
                  )}
                </div>
              </div>
              {blockedReason && <p className="text-xs text-[var(--n-600)]">{blockedReason}</p>}

              {pending && (
                <section
                  aria-label={pendingAriaLabel}
                  className="rounded-[var(--radius-md)] border border-[var(--n-200)] bg-[var(--n-50)] p-3"
                >
                  <h4 className="text-sm font-semibold text-[var(--n-900)]">
                    Menunggu persetujuan
                  </h4>
                  <dl className="mt-2 grid grid-cols-1 gap-x-6 gap-y-1 text-sm sm:grid-cols-2 lg:grid-cols-4">
                    <div>
                      <dt className="text-xs uppercase tracking-wider text-[var(--n-600)]">
                        Perubahan
                      </dt>
                      <dd className="text-[var(--n-800)]">{PENDING_OP_LABEL[pending.op]}</dd>
                    </div>
                    <div>
                      <dt className="text-xs uppercase tracking-wider text-[var(--n-600)]">
                        Diajukan oleh
                      </dt>
                      <dd className="text-[var(--n-800)]">{pending.maker_name}</dd>
                    </div>
                    {pendingWod && (
                      <div>
                        <dt className="text-xs uppercase tracking-wider text-[var(--n-600)]">
                          No. WOD
                        </dt>
                        <dd className="tabular-nums text-[var(--n-800)]">{pendingWod}</dd>
                      </div>
                    )}
                    {pendingName && (
                      <div>
                        <dt className="text-xs uppercase tracking-wider text-[var(--n-600)]">
                          Nama PKS
                        </dt>
                        <dd className="text-[var(--n-800)]">{pendingName}</dd>
                      </div>
                    )}
                    {pendingReason && (
                      <div className="sm:col-span-2">
                        <dt className="text-xs uppercase tracking-wider text-[var(--n-600)]">
                          Alasan
                        </dt>
                        <dd className="text-[var(--n-800)]">{pendingReason}</dd>
                      </div>
                    )}
                  </dl>
                </section>
              )}

              <section aria-label={activeAriaLabel}>
                <div className="flex items-center justify-between gap-3">
                  <h4 className="text-xs font-semibold uppercase tracking-wider text-[var(--n-600)]">
                    PKS aktif
                  </h4>
                  <PksStatusBadge status={group.pks_status} />
                </div>
                {group.active ? (
                  <dl className="mt-2 grid grid-cols-1 gap-x-6 gap-y-2 text-sm sm:grid-cols-2 lg:grid-cols-3">
                    <div>
                      <dt className="text-xs uppercase tracking-wider text-[var(--n-600)]">
                        No. WOD
                      </dt>
                      <dd className="tabular-nums font-medium text-[var(--n-800)]">
                        {group.active.wod_number}
                      </dd>
                    </div>
                    <div>
                      <dt className="text-xs uppercase tracking-wider text-[var(--n-600)]">
                        Nama PKS
                      </dt>
                      <dd className="text-[var(--n-800)]">{group.active.pks_name}</dd>
                    </div>
                    <div>
                      <dt className="text-xs uppercase tracking-wider text-[var(--n-600)]">
                        Efektif
                      </dt>
                      <dd className="tabular-nums text-[var(--n-800)]">
                        {fmtDate(group.active.effective_date)}
                      </dd>
                    </div>
                    <div>
                      <dt className="text-xs uppercase tracking-wider text-[var(--n-600)]">
                        Expired
                      </dt>
                      <dd className="tabular-nums text-[var(--n-800)]">
                        {fmtDate(group.active.expiry_date)}
                      </dd>
                    </div>
                    <div>
                      <dt className="text-xs uppercase tracking-wider text-[var(--n-600)]">
                        Masa berlaku
                      </dt>
                      <dd className="text-[var(--n-800)]">
                        {formatValidity(group.active.validity_months)}
                      </dd>
                    </div>
                    <div>
                      <dt className="text-xs uppercase tracking-wider text-[var(--n-600)]">
                        Monitoring
                      </dt>
                      <dd className="tabular-nums text-[var(--n-800)]">
                        {formatMonitoring(group.pks_status, group.active.remaining_days)}
                      </dd>
                    </div>
                    {isTerminated && (
                      <div className="sm:col-span-2 lg:col-span-3">
                        <dt className="text-xs uppercase tracking-wider text-[var(--n-600)]">
                          Terminate
                        </dt>
                        <dd className="text-[var(--n-800)]">
                          <span className="tabular-nums">
                            {fmtDate(group.active.terminated_at)}
                          </span>
                          {group.active.termination_reason
                            ? `: ${group.active.termination_reason}`
                            : ""}
                        </dd>
                      </div>
                    )}
                  </dl>
                ) : (
                  <p className="mt-2 text-sm text-[var(--n-600)]">Belum ada PKS yang dicatat.</p>
                )}
              </section>

              <section aria-label={historyAriaLabel}>
                <h4 className="mb-2 text-xs font-semibold uppercase tracking-wider text-[var(--n-600)]">
                  Riwayat PKS
                </h4>
                {group.history.length > 0 ? (
                  <HistoryTable rows={group.history} caption={historyCaption} />
                ) : (
                  <p className="text-sm text-[var(--n-600)]">Belum ada riwayat.</p>
                )}
              </section>
            </article>
          );
        })
      )}

      <PksFormDialog
        open={formState.open}
        onClose={() => setFormState({ open: false })}
        vendorId={vendorId}
        initialTypeCode={formState.initialTypeCode}
        replacesActive={formState.replacesActive}
        groups={groups}
      />
      {reasonState && (
        <PksReasonDialog
          open
          onClose={() => setReasonState(null)}
          vendorId={vendorId}
          pksId={reasonState.pksId}
          pksName={reasonState.pksName}
          mode={reasonState.mode}
        />
      )}
    </div>
  );
}
