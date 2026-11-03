import { Button } from "@/components/ui/Button";
import type {
  PartyAtm,
  RealizationInput,
  RealizationView,
  VendorOrderDetail,
} from "@/features/orders/api";
import { denomLabel, formatDate } from "@/features/orders/labels";
import { useSubmitRealization } from "@/features/orders/useOrders";
import type { ApiError } from "@/lib/api/client";
import { formatIDR } from "@/lib/formatters";
import { useState } from "react";

// replenishment-realisasi FR2 / FR5.1: the replenish branch reports, per ATM, whether it was filled,
// the date and the amount per denom. CIMB (ATM-SPV) approves or rejects; a rejected report can be
// sent again. The checks below mirror the server (S2 validateRealization) to stop typos early; the
// official result still comes from the ATM's own ITM data.

const EARLIEST_DAYS = 7; // K1: at most 7 days before the schedule

export interface AtmDraft {
  readonly result: "success" | "failed";
  readonly date: string; // YYYY-MM-DD
  readonly amounts: Readonly<Record<number, string>>; // denom -> digits typed
}

function todayWib(): string {
  return new Date().toLocaleDateString("sv-SE", { timeZone: "Asia/Jakarta" });
}

function shiftDays(iso: string, days: number): string {
  const d = new Date(`${iso}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() + days);
  return d.toISOString().slice(0, 10);
}

/** Problems of one ATM's draft; empty = ok. scheduled = YYYY-MM-DD. */
export function draftErrors(
  atm: PartyAtm,
  d: AtmDraft,
  scheduled: string,
  today: string,
): string[] {
  if (d.result === "failed") return [];
  const out: string[] = [];
  if (!d.date) out.push("isi tanggal realisasi");
  else if (d.date > today) out.push("tanggal tidak boleh setelah hari ini");
  else if (d.date < shiftDays(scheduled, -EARLIEST_DAYS))
    out.push(`tanggal paling awal ${EARLIEST_DAYS} hari sebelum jadwal`);
  let filled = false;
  for (const { denom } of atm.denoms) {
    const raw = (d.amounts[denom] ?? "").trim();
    if (raw === "") continue;
    if (!/^\d+$/.test(raw)) {
      out.push(`nominal ${denomLabel(denom)} hanya angka`);
      continue;
    }
    if (Number(raw) % denom !== 0)
      out.push(`nominal ${denomLabel(denom)} harus kelipatan ${denom.toLocaleString("id-ID")}`);
    if (Number(raw) > 0) filled = true;
  }
  if (!filled) out.push("isi minimal satu nominal");
  return out;
}

export function toInput(atm: PartyAtm, d: AtmDraft): RealizationInput {
  if (d.result === "failed") return { terminal_id: atm.terminal_id, result: "failed" };
  const amounts: Record<string, number> = {};
  for (const { denom } of atm.denoms) {
    const raw = (d.amounts[denom] ?? "").trim();
    if (raw !== "") amounts[String(denom)] = Number(raw);
  }
  return { terminal_id: atm.terminal_id, result: "success", realized_date: d.date, amounts };
}

function initialDraft(
  atm: PartyAtm,
  prev: RealizationView | undefined,
  scheduled: string,
  today: string,
): AtmDraft {
  if (prev?.result === "failed") return { result: "failed", date: "", amounts: {} };
  const amounts: Record<number, string> = {};
  for (const { denom, amount } of atm.denoms) {
    const reported = prev?.amounts.find((a) => a.denom === denom)?.amount;
    amounts[denom] = String(reported ?? amount);
  }
  const date = prev?.realized_date?.slice(0, 10) ?? (scheduled <= today ? scheduled : today);
  return { result: "success", date, amounts };
}

function errorMessage(err: unknown): string {
  const e = err as Partial<ApiError> | null;
  if (e?.status === 409)
    return "Laporan tidak dapat dikirim untuk status request saat ini. Muat ulang halaman.";
  return e?.message ?? "Terjadi kesalahan. Coba lagi.";
}

export function RealizationPanel({ order }: { readonly order: VendorOrderDetail }) {
  const reports = order.realizations ?? [];
  return (
    <section
      aria-labelledby="realization-heading"
      className="flex flex-col gap-3 rounded-lg border border-neutral-200 bg-white p-4"
    >
      <h2 id="realization-heading" className="text-base font-semibold text-surface-text">
        Laporan realisasi
      </h2>
      {order.can_report && order.completion_rejection_reason && (
        <p role="alert" className="text-sm text-danger-fg">
          <span className="font-semibold">Laporan sebelumnya ditolak CIMB: </span>
          {order.completion_rejection_reason}
        </p>
      )}
      {order.can_report ? (
        <RealizationForm order={order} reports={reports} />
      ) : reports.length > 0 ? (
        <ReportTable reports={reports} />
      ) : (
        <p className="text-sm text-neutral-600">
          Laporan realisasi dapat dikirim setelah semua pihak vendor menerima request ini.
        </p>
      )}
    </section>
  );
}

function ReportTable({ reports }: { readonly reports: readonly RealizationView[] }) {
  return (
    <div className="overflow-x-auto rounded-lg border border-neutral-200">
      <table className="w-full text-sm">
        <thead className="bg-neutral-50 text-left text-xs uppercase tracking-wide text-neutral-500">
          <tr>
            <th scope="col" className="px-3 py-2">
              Terminal
            </th>
            <th scope="col" className="px-3 py-2">
              Hasil
            </th>
            <th scope="col" className="px-3 py-2">
              Tanggal
            </th>
            <th scope="col" className="px-3 py-2 text-right">
              Nominal
            </th>
          </tr>
        </thead>
        <tbody>
          {reports.map((r) => (
            <tr key={r.terminal_id} className="border-t border-neutral-100">
              <td className="px-3 py-2 font-medium">{r.terminal_id}</td>
              <td className="px-3 py-2">{r.result === "failed" ? "Gagal" : "Berhasil"}</td>
              <td className="px-3 py-2">{formatDate(r.realized_date)}</td>
              <td className="px-3 py-2 text-right tabular-nums">
                {r.amounts.length === 0
                  ? "-"
                  : r.amounts.map((a) => (
                      <span key={a.denom} className="block">
                        {denomLabel(a.denom)} {formatIDR(a.amount)}
                      </span>
                    ))}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function RealizationForm({
  order,
  reports,
}: { readonly order: VendorOrderDetail; readonly reports: readonly RealizationView[] }) {
  const today = todayWib();
  const scheduled = order.replenish_date?.slice(0, 10) ?? today;
  const atms = order.content.atms;
  const submit = useSubmitRealization(order.id);
  const [drafts, setDrafts] = useState<Record<string, AtmDraft>>(() =>
    Object.fromEntries(
      atms.map((a) => [
        a.terminal_id,
        initialDraft(
          a,
          reports.find((r) => r.terminal_id === a.terminal_id),
          scheduled,
          today,
        ),
      ]),
    ),
  );
  const errors = atms.map((a) => ({
    atm: a,
    errs: draftErrors(a, drafts[a.terminal_id] as AtmDraft, scheduled, today),
  }));
  const isValid = errors.every((e) => e.errs.length === 0);

  function update(terminal: string, patch: Partial<AtmDraft>) {
    setDrafts((prev) => ({ ...prev, [terminal]: { ...(prev[terminal] as AtmDraft), ...patch } }));
  }

  return (
    <form
      className="flex flex-col gap-4"
      onSubmit={(e) => {
        e.preventDefault();
        if (isValid) submit.mutate(atms.map((a) => toInput(a, drafts[a.terminal_id] as AtmDraft)));
      }}
    >
      <p className="text-sm text-neutral-600">
        Isi untuk setiap ATM. Nominal awal diambil dari order; ubah bila realisasi berbeda.
        Klasifikasi resmi tetap dari data ITM mesin.
      </p>
      {submit.isError && (
        <p role="alert" className="text-sm font-semibold text-danger-fg">
          {errorMessage(submit.error)}
        </p>
      )}
      {errors.map(({ atm, errs }) => {
        const d = drafts[atm.terminal_id] as AtmDraft;
        const id = `rz-${atm.terminal_id}`;
        return (
          <fieldset
            key={atm.terminal_id}
            className="flex flex-col gap-2 rounded-md border border-neutral-200 p-3"
          >
            <legend className="px-1 text-sm font-semibold">
              {atm.terminal_id}{" "}
              <span className="font-normal text-neutral-500">· tiket {atm.ticket_number}</span>
            </legend>
            <div className="flex flex-wrap gap-4 text-sm">
              {(["success", "failed"] as const).map((r) => (
                <label key={r} className="flex min-h-[44px] items-center gap-2">
                  <input
                    type="radio"
                    name={`${id}-result`}
                    checked={d.result === r}
                    onChange={() => update(atm.terminal_id, { result: r })}
                  />
                  {r === "success" ? "Berhasil diisi" : "Gagal diisi"}
                </label>
              ))}
            </div>
            {d.result === "success" && (
              <div className="flex flex-wrap items-end gap-3">
                <label className="flex flex-col gap-1 text-sm" htmlFor={`${id}-date`}>
                  Tanggal realisasi
                  <input
                    id={`${id}-date`}
                    type="date"
                    value={d.date}
                    max={today}
                    onChange={(e) => update(atm.terminal_id, { date: e.target.value })}
                    className="min-h-[44px] rounded-md border border-neutral-300 px-2"
                  />
                </label>
                {atm.denoms.map(({ denom, amount }) => (
                  <label
                    key={denom}
                    className="flex flex-col gap-1 text-sm"
                    htmlFor={`${id}-${denom}`}
                  >
                    Nominal {denomLabel(denom)} (IDR)
                    <input
                      id={`${id}-${denom}`}
                      inputMode="numeric"
                      value={d.amounts[denom] ?? ""}
                      onChange={(e) =>
                        update(atm.terminal_id, {
                          amounts: { ...d.amounts, [denom]: e.target.value.replace(/[^\d]/g, "") },
                        })
                      }
                      className="min-h-[44px] w-44 rounded-md border border-neutral-300 px-2 text-right tabular-nums"
                    />
                    <span className="text-xs text-neutral-500">Order {formatIDR(amount)}</span>
                  </label>
                ))}
              </div>
            )}
            {errs.length > 0 && (
              <ul className="list-disc pl-5 text-xs text-danger-fg">
                {errs.map((m) => (
                  <li key={m}>{m}</li>
                ))}
              </ul>
            )}
          </fieldset>
        );
      })}
      <div>
        <Button type="submit" disabled={!isValid} isLoading={submit.isPending}>
          Kirim laporan realisasi
        </Button>
      </div>
    </form>
  );
}
