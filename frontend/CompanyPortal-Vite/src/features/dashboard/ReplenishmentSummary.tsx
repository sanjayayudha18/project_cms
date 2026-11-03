/**
 * ReplenishmentSummary — kartu dashboard realisasi pengisian (replenishment-realisasi FR4.2 + FNC 003):
 * instruksi hari ini (ATM + nominal order) dan hasil klasifikasi 7 hari terakhir per kelas.
 */
import { Link } from "@tanstack/react-router";

import { Badge } from "@/components/ui/Badge";
import { getTodayWib } from "@/features/eod-monitoring/utils";
import { CLASS_VARIANT } from "@/features/replenishment/ReplenishmentScreen";
import {
  CLASSIFICATION_LABELS,
  type Classification,
  useReplenishmentSummary,
} from "@/features/replenishment/api";
import { formatIDRFull } from "@/lib/utils/formatters";

const ORDER: Classification[] = ["on_schedule", "early", "late", "not_done", "no_order", "pending"];

function sevenDaysBefore(iso: string): string {
  const d = new Date(`${iso}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() - 6);
  return d.toISOString().slice(0, 10);
}

export function ReplenishmentSummary() {
  const today = getTodayWib();
  const { data, isLoading, isError } = useReplenishmentSummary(sevenDaysBefore(today), today);
  const todayInstr = data?.instructions.find((i) => i.date?.startsWith(today));
  const counts = new Map((data?.by_class ?? []).map((c) => [c.classification, c]));
  const sum = (key: "amount_diff" | "vendor_report_late") =>
    (data?.by_class ?? []).reduce((s, c) => s + c[key], 0);

  return (
    <section aria-labelledby="replenishment-summary-heading">
      <div className="mb-4 flex items-center justify-between">
        <h2
          id="replenishment-summary-heading"
          className="text-lg font-semibold text-[var(--n-900)]"
        >
          Realisasi Pengisian
        </h2>
        <Link
          to="/replenishment"
          className="text-sm font-medium text-[var(--red-600)] hover:text-[var(--red-700)] transition-colors duration-100"
        >
          Lihat semua &rarr;
        </Link>
      </div>
      <div className="rounded-[var(--radius-lg)] border border-[var(--n-200)] bg-[var(--n-0)] p-5">
        {isLoading && <p className="text-sm text-[var(--n-500)]">Memuat ringkasan...</p>}
        {isError && (
          <p className="text-sm text-[var(--danger-fg)]">Ringkasan realisasi gagal dimuat.</p>
        )}
        {data && (
          <div className="flex flex-col gap-5">
            <dl className="grid grid-cols-2 gap-4">
              <div>
                <dt className="text-xs uppercase tracking-wider text-[var(--n-500)]">
                  ATM dijadwalkan hari ini
                </dt>
                <dd className="text-2xl font-semibold tabular-nums text-[var(--n-900)]">
                  {todayInstr?.terminals ?? 0}
                </dd>
              </div>
              <div>
                <dt className="text-xs uppercase tracking-wider text-[var(--n-500)]">
                  Nominal instruksi hari ini
                </dt>
                <dd className="text-2xl font-semibold tabular-nums text-[var(--n-900)]">
                  {formatIDRFull(todayInstr?.order_amount ?? 0)}
                </dd>
              </div>
            </dl>
            <div>
              <h3 className="mb-2 text-sm font-medium text-[var(--n-700)]">
                Klasifikasi 7 hari terakhir
              </h3>
              <ul className="flex flex-wrap gap-2">
                {ORDER.map((c) => (
                  <li key={c} className="flex items-center gap-1.5">
                    <Badge variant={CLASS_VARIANT[c]} label={CLASSIFICATION_LABELS[c]} />
                    <span className="tabular-nums text-sm font-semibold text-[var(--n-900)]">
                      {counts.get(c)?.total ?? 0}
                    </span>
                  </li>
                ))}
              </ul>
              <p className="mt-3 text-sm text-[var(--n-600)]">
                <span className="tabular-nums font-semibold">{sum("amount_diff")}</span> tiket
                dengan selisih nominal ·{" "}
                <span className="tabular-nums font-semibold">{sum("vendor_report_late")}</span>{" "}
                laporan vendor telat
              </p>
            </div>
          </div>
        )}
      </div>
    </section>
  );
}
