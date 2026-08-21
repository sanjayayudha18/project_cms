/**
 * ReplenishmentSummary — tabel jadwal pengisian ulang hari ini untuk dashboard.
 * Menampilkan DataTable yang di-sort berdasarkan prioritas status dengan info rute,
 * vendor, progress, status, dan nilai.
 *
 * @validates Requirements 2.4
 */
import { Link } from "@tanstack/react-router";
import { createColumnHelper } from "@tanstack/react-table";

import { Badge } from "@/components/ui/Badge";
import { DataTable } from "@/components/ui/DataTable";
import { ProgressBar } from "@/components/ui/ProgressBar";
import schedules from "@/data/replenishment-schedules.json";
import { sortByStatusPriority } from "@/features/dashboard/replenishment.utils";
import { formatIDRFull } from "@/lib/utils/formatters";
import type { BadgeVariant, ProgressBarStatus, ReplenishmentSchedule } from "./types";

/** Map schedule status to Badge variant. */
const statusBadgeVariant: Record<ReplenishmentSchedule["status"], BadgeVariant> = {
  "in-transit": "info",
  completed: "success",
  delayed: "warning",
  scheduled: "neutral",
  "pending-vendor": "neutral",
};

/** Map schedule status to display label (Bahasa Indonesia). */
const statusLabel: Record<ReplenishmentSchedule["status"], string> = {
  "in-transit": "Dalam Perjalanan",
  completed: "Selesai",
  delayed: "Tertunda",
  scheduled: "Terjadwal",
  "pending-vendor": "Menunggu Vendor",
};

/** Map schedule status to ProgressBar-compatible status. */
function toProgressBarStatus(status: ReplenishmentSchedule["status"]): ProgressBarStatus {
  if (status === "completed") return "completed";
  if (status === "delayed") return "delayed";
  return "in-transit";
}

const columnHelper = createColumnHelper<ReplenishmentSchedule>();

const columns = [
  columnHelper.accessor("routeCode", {
    header: "Rute",
    cell: (info) => (
      <div>
        <span className="font-bold text-[var(--n-900)]">{info.getValue()}</span>
        <span className="block text-xs text-[var(--n-500)]">{info.row.original.region}</span>
      </div>
    ),
  }),
  columnHelper.accessor("vendor", {
    header: "Vendor",
    cell: (info) => <span className="text-[var(--n-700)]">{info.getValue()}</span>,
  }),
  columnHelper.accessor("machineCount", {
    header: "Mesin",
    cell: (info) => <span className="tabular-nums text-[var(--n-800)]">{info.getValue()}</span>,
  }),
  columnHelper.display({
    id: "progress",
    header: "Progres",
    cell: (info) => (
      <ProgressBar
        completed={info.row.original.completionCount}
        total={info.row.original.machineCount}
        status={toProgressBarStatus(info.row.original.status)}
      />
    ),
  }),
  columnHelper.accessor("status", {
    header: "Status",
    cell: (info) => {
      const status = info.getValue();
      return <Badge variant={statusBadgeVariant[status]} label={statusLabel[status]} />;
    },
  }),
  columnHelper.accessor("cashValue", {
    header: "Nilai",
    cell: (info) => (
      <span className="tabular-nums font-medium text-[var(--n-900)]">
        {formatIDRFull(info.getValue())}
      </span>
    ),
    meta: { align: "right" },
  }),
];

/** Sorted schedule data for the dashboard table. */
const sortedSchedules = sortByStatusPriority(schedules as ReplenishmentSchedule[]);

export function ReplenishmentSummary() {
  return (
    <section aria-labelledby="replenishment-summary-heading">
      <div className="mb-4 flex items-center justify-between">
        <h2
          id="replenishment-summary-heading"
          className="text-lg font-semibold text-[var(--n-900)]"
        >
          Pengisian Ulang Hari Ini
        </h2>
        <Link
          to="/replenishment"
          className="text-sm font-medium text-[var(--red-600)] hover:text-[var(--red-700)] transition-colors duration-100"
        >
          Lihat semua &rarr;
        </Link>
      </div>
      <DataTable data={sortedSchedules} columns={columns} />
    </section>
  );
}
