import { Button } from "@/components/ui/Button";
import { DataTable } from "@/components/ui/DataTable";
import { Dialog } from "@/components/ui/Dialog";
import type { ColumnDef } from "@tanstack/react-table";
import { useState } from "react";
import {
  type CategoryParamsRequest,
  type CategoryParamsSet,
  formatIdrAmount,
  useCashCountMutation,
  useCategoryParams,
} from "./api";
import { Field, FormActions, LoadState, inputClass, useSubmit } from "./shared";

const money = "text-right tabular-nums";
const date = (s: string | null) => (s ? s.slice(0, 10) : "-");

const columns: ColumnDef<CategoryParamsSet, unknown>[] = [
  {
    accessorKey: "medium_min_amount",
    header: "Batas bawah Medium (IDR)",
    meta: { align: "right" },
    cell: ({ row }) => (
      <span className={money}>{formatIdrAmount(row.original.medium_min_amount)}</span>
    ),
  },
  {
    accessorKey: "medium_max_amount",
    header: "Batas atas Medium (IDR)",
    meta: { align: "right" },
    cell: ({ row }) => (
      <span className={money}>{formatIdrAmount(row.original.medium_max_amount)}</span>
    ),
  },
  { accessorKey: "onsite_high", header: "Onsite High", meta: { align: "right" } },
  { accessorKey: "onsite_medium", header: "Onsite Medium", meta: { align: "right" } },
  { accessorKey: "onsite_low", header: "Onsite Low", meta: { align: "right" } },
  { id: "from", header: "Berlaku", cell: ({ row }) => date(row.original.effective_from) },
  { id: "to", header: "Digantikan", cell: ({ row }) => date(row.original.superseded_at) },
];

/**
 * Kategori vault (AC-5.1-01/03/04): one active set + history. Only "create" exists:
 * submitting proposes a new set that replaces the active one once approved.
 */
export function CategoryParamsTab({ canWrite }: { canWrite: boolean }) {
  const query = useCategoryParams();
  const { run, isPending } = useSubmit(
    useCashCountMutation<CategoryParamsRequest, never>("category-params"),
  );
  const [open, setOpen] = useState(false);
  const active = query.data?.active ?? null;

  return (
    <div className="flex flex-col gap-4">
      <LoadState isLoading={query.isLoading} isError={query.isError} label="parameter kategori" />
      {query.data && (
        <>
          <section
            aria-label="Set parameter aktif"
            className="rounded-[var(--radius-md)] border border-[var(--n-200)] p-4"
          >
            <div className="flex flex-wrap items-center justify-between gap-2">
              <h2 className="text-sm font-semibold text-[var(--n-800)]">Set aktif</h2>
              {canWrite && <Button onClick={() => setOpen(true)}>Ajukan Set Baru</Button>}
            </div>
            {active ? (
              <dl className="mt-3 grid grid-cols-2 gap-3 text-sm sm:grid-cols-5">
                <Stat label="Low jika saldo di bawah (IDR)">
                  {formatIdrAmount(active.medium_min_amount)}
                </Stat>
                <Stat label="High jika saldo di atas (IDR)">
                  {formatIdrAmount(active.medium_max_amount)}
                </Stat>
                <Stat label="Onsite High / 6 bln">{active.onsite_high}</Stat>
                <Stat label="Onsite Medium / 6 bln">{active.onsite_medium}</Stat>
                <Stat label="Onsite Low / 6 bln">{active.onsite_low}</Stat>
              </dl>
            ) : (
              <p className="mt-3 text-sm text-[var(--n-500)]">Belum ada set aktif.</p>
            )}
            <p className="mt-3 text-xs text-[var(--n-500)]">
              Medium = saldo dari batas bawah sampai batas atas (keduanya inklusif).
            </p>
          </section>

          <h2 className="text-sm font-semibold text-[var(--n-800)]">Riwayat set</h2>
          <DataTable
            data={query.data.history}
            columns={columns}
            emptyMessage="Belum ada riwayat set lama"
          />
        </>
      )}

      {open && (
        <SetDialog
          initial={active}
          isPending={isPending}
          onClose={() => setOpen(false)}
          onSubmit={(body) => run({ op: "create", body }, () => setOpen(false))}
        />
      )}
    </div>
  );
}

function Stat({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-1">
      <dt className="text-xs text-[var(--n-500)]">{label}</dt>
      <dd className="text-lg font-semibold tabular-nums text-[var(--n-900)]">{children}</dd>
    </div>
  );
}

const DECIMAL = /^\d+(\.\d{1,2})?$/;

function SetDialog({
  initial,
  isPending,
  onClose,
  onSubmit,
}: {
  initial: CategoryParamsSet | null;
  isPending: boolean;
  onClose: () => void;
  onSubmit: (body: CategoryParamsRequest) => void;
}) {
  const [min, setMin] = useState(initial?.medium_min_amount ?? "");
  const [max, setMax] = useState(initial?.medium_max_amount ?? "");
  const [high, setHigh] = useState(String(initial?.onsite_high ?? 3));
  const [medium, setMedium] = useState(String(initial?.onsite_medium ?? 2));
  const [low, setLow] = useState(String(initial?.onsite_low ?? 1));

  const counts = [high, medium, low].map(Number);
  const countsOk = counts.every((n) => Number.isInteger(n) && n >= 0 && n <= 6);
  // Amounts stay decimal text end to end (no float); the API is the authority on min < max.
  const canSubmit = DECIMAL.test(min) && DECIMAL.test(max) && countsOk && !isPending;

  function submit(e: React.FormEvent): void {
    e.preventDefault();
    if (!canSubmit) return;
    onSubmit({
      medium_min_amount: min,
      medium_max_amount: max,
      currency_code: "IDR",
      onsite_high: counts[0] ?? 0,
      onsite_medium: counts[1] ?? 0,
      onsite_low: counts[2] ?? 0,
    });
  }

  return (
    <Dialog open onClose={onClose} title="Ajukan Set Parameter Kategori Vault">
      <form onSubmit={submit} className="flex flex-col gap-4">
        <Field label="Batas bawah Medium (IDR)">
          <input
            inputMode="decimal"
            value={min}
            onChange={(e) => setMin(e.target.value)}
            className={`${inputClass} text-right tabular-nums`}
            placeholder="5000000000.00"
          />
        </Field>
        <Field label="Batas atas Medium (IDR)">
          <input
            inputMode="decimal"
            value={max}
            onChange={(e) => setMax(e.target.value)}
            className={`${inputClass} text-right tabular-nums`}
            placeholder="15000000000.00"
          />
        </Field>
        <p className="text-xs text-[var(--n-500)]">
          Angka desimal dengan titik, maksimal 2 desimal. Batas bawah harus lebih besar dari 0 dan
          lebih kecil dari batas atas.
        </p>
        <div className="grid grid-cols-3 gap-3">
          {(
            [
              ["Onsite High", high, setHigh],
              ["Onsite Medium", medium, setMedium],
              ["Onsite Low", low, setLow],
            ] as const
          ).map(([label, value, set]) => (
            <Field key={label} label={label}>
              <input
                type="number"
                min={0}
                max={6}
                value={value}
                onChange={(e) => set(e.target.value)}
                className={`${inputClass} text-right tabular-nums`}
              />
            </Field>
          ))}
        </div>
        <p className="text-xs text-[var(--n-500)]">Kunjungan onsite per 6 bulan, 0–6.</p>
        <FormActions onClose={onClose} canSubmit={canSubmit} />
      </form>
    </Dialog>
  );
}
