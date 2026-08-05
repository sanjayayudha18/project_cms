import { AlertTriangle, Banknote, Route, Wifi } from "lucide-react";
import { formatIDRAbbreviated } from "@/lib/utils/formatters";
import kpiData from "@/data/dashboard-kpi.json";
import type { DashboardKpi } from "./types";

/**
 * MetricStrip — menampilkan 4 kartu KPI dalam grid responsif.
 *
 * Layout:
 * - >1080px: 4-column grid dengan divider vertikal
 * - 760–1080px: 2×2 grid dengan border-bottom pada baris pertama
 * - <760px: kolom tunggal dengan border-bottom separator
 *
 * @validates Requirements 2.2
 */

interface MetricCardData {
  label: string;
  icon: React.ReactNode;
  value: string;
  meta: string;
}

const kpi = kpiData as DashboardKpi;

const metrics: MetricCardData[] = [
  {
    label: "Managed Cash",
    icon: <Banknote size={14} />,
    value: formatIDRAbbreviated(kpi.managedCash),
    meta: `↑ ${kpi.managedCashChange}% dari kemarin`,
  },
  {
    label: "ATM Availability",
    icon: <Wifi size={14} />,
    value: `${kpi.atmAvailability}%`,
    meta: `${kpi.atmOnline.toLocaleString("id-ID")} dari ${kpi.atmTotal.toLocaleString("id-ID")} online`,
  },
  {
    label: "Rute Hari Ini",
    icon: <Route size={14} />,
    value: `${kpi.todayRoutes}`,
    meta: `${kpi.routesCompleted} selesai, ${kpi.routesActive} aktif`,
  },
  {
    label: "Exceptions",
    icon: <AlertTriangle size={14} />,
    value: `${kpi.exceptions}`,
    meta: `${kpi.exceptionsHigh} prioritas tinggi sebelum ${kpi.exceptionsCutoffHour}:00`,
  },
];

export function MetricStrip() {
  return (
    <div
      className="mt-6 bg-[var(--n-0)] border border-[var(--n-200)] rounded-[var(--radius-lg)]
        grid grid-cols-1 min-[760px]:grid-cols-2 min-[1080px]:grid-cols-4"
    >
      {metrics.map((metric, index) => (
        <div
          key={metric.label}
          className={`p-5 ${getCardBorderClasses(index)}`}
        >
          <div className="flex items-center gap-1.5 text-xs text-[var(--n-500)]">
            {metric.icon}
            <span>{metric.label}</span>
          </div>
          <div className="mt-1 text-2xl font-bold tabular-nums text-[var(--n-900)]">
            {metric.value}
          </div>
          <div className="mt-1 text-xs text-[var(--n-500)]">
            {metric.meta}
          </div>
        </div>
      ))}
    </div>
  );
}

/**
 * Compute border classes for responsive dividers between metric cards.
 *
 * Desktop (4-col): vertical border-right on first 3 cards.
 * Tablet (2×2): border-bottom on first 2 cards, border-right on 1st and 3rd.
 * Mobile (1-col): border-bottom on all except last.
 */
function getCardBorderClasses(index: number): string {
  const classes: string[] = [];

  // Mobile: border-bottom on all except last
  if (index < 3) {
    classes.push("border-b border-[var(--n-200)] min-[760px]:border-b-0");
  }

  // Tablet (2×2): border-bottom on top row (indices 0, 1), border-right on left column (indices 0, 2)
  if (index < 2) {
    classes.push("min-[760px]:border-b min-[760px]:border-[var(--n-200)] min-[1080px]:border-b-0");
  }
  if (index % 2 === 0) {
    classes.push("min-[760px]:border-r min-[760px]:border-[var(--n-200)] min-[1080px]:border-r-0");
  }

  // Desktop (4-col): vertical border-right on first 3
  if (index < 3) {
    classes.push("min-[1080px]:border-r min-[1080px]:border-[var(--n-200)]");
  }

  return classes.join(" ");
}
