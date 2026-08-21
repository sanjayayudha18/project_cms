import { createColumnHelper } from "@tanstack/react-table";
import { AlertTriangle, CheckCircle2, Clock, Search, Truck } from "lucide-react";
import { useMemo, useState } from "react";

import { Badge } from "@/components/ui/Badge";
import type { BadgeVariant } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { DataTable } from "@/components/ui/DataTable";
import { FilterSelect } from "@/components/ui/FilterSelect";
import { PageHeader } from "@/components/ui/PageHeader";
import schedulesData from "@/data/replenishment-schedules.json";
import { useToast } from "@/lib/hooks/useToast";
import { formatIDRFull } from "@/lib/utils/formatters";
import { filterSchedules, sortByStatusPriority } from "./replenishment.utils";
import type { ReplenishmentSchedule } from "./types";

const schedules = schedulesData as ReplenishmentSchedule[];

const STATUS_CONFIG: Record<
  ReplenishmentSchedule["status"],
  { label: string; variant: BadgeVariant; icon: typeof Truck }
> = {
  "in-transit": { label: "Dalam perjalanan", variant: "info", icon: Truck },
  completed: { label: "Selesai", variant: "success", icon: CheckCircle2 },
  delayed: { label: "Terlambat", variant: "warning", icon: AlertTriangle },
  "pending-vendor": { label: "Menunggu vendor", variant: "warning", icon: Clock },
  scheduled: { label: "Terjadwal", variant: "neutral", icon: Clock },
};

const columnHelper = createColumnHelper<ReplenishmentSchedule>();

const columns = [
  columnHelper.accessor("id", {
    header: "Jadwal",
    cell: (info) => (
      <div>
        <span className="font-semibold text-[var(--n-900)]">{info.getValue()}</span>
        <span className="block text-xs text-[var(--n-500)]">{info.row.original.routeCode}</span>
      </div>
    ),
  }),
  columnHelper.accessor("region", {
    header: "Wilayah",
    cell: (info) => info.getValue(),
  }),
  columnHelper.accessor("vendor", {
    header: "Vendor",
    cell: (info) => info.getValue(),
  }),
  columnHelper.display({
    id: "window",
    header: "Jendela waktu",
    cell: ({ row }) => `${row.original.windowStart}\u2013${row.original.windowEnd}`,
  }),
  columnHelper.accessor("machineCount", {
    header: "Mesin",
    cell: (info) => info.getValue(),
  }),
  columnHelper.accessor("status", {
    header: "Status",
    cell: (info) => {
      const cfg = STATUS_CONFIG[info.getValue()];
      return <Badge variant={cfg.variant} icon={cfg.icon} label={cfg.label} />;
    },
  }),
  columnHelper.accessor("cashValue", {
    header: "Nilai kas",
    cell: (info) => formatIDRFull(info.getValue()),
    meta: { align: "right" },
  }),
];

/**
 * Replenishment schedules screen.
 * Displays filterable table of cash replenishment schedules with region/vendor filters.
 */
export function ReplenishmentScreen() {
  const { toast } = useToast();
  const [regionFilter, setRegionFilter] = useState<string | null>(null);
  const [vendorFilter, setVendorFilter] = useState<string | null>(null);

  const regions = useMemo(
    () =>
      Array.from(new Set(schedules.map((s) => s.region)))
        .sort()
        .map((r) => ({ value: r, label: r })),
    [],
  );

  const vendors = useMemo(
    () =>
      Array.from(new Set(schedules.map((s) => s.vendor)))
        .sort()
        .map((v) => ({ value: v, label: v })),
    [],
  );

  const todayLabel = useMemo(() => {
    const d = new Date();
    return d.toLocaleDateString("id-ID", { day: "numeric", month: "short", year: "numeric" });
  }, []);

  const filteredData = useMemo(() => {
    const filtered = filterSchedules(
      schedules,
      regionFilter ?? "All regions",
      vendorFilter ?? "All vendors",
    );
    return sortByStatusPriority(filtered);
  }, [regionFilter, vendorFilter]);

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Operasi kas"
        title="Jadwal pengisian ulang"
        description="Pantau rute CIT harian, penugasan vendor, dan kepatuhan jadwal di seluruh wilayah."
        actions={
          <>
            <Button variant="secondary">Impor rencana</Button>
            <Button
              variant="primary"
              onClick={() => toast({ type: "success", message: "Jadwal berhasil dibuat" })}
            >
              Jadwal baru
            </Button>
          </>
        }
      />

      {/* Filter toolbar */}
      <div className="flex flex-wrap items-end gap-4">
        <FilterSelect
          label="Tanggal"
          options={[{ value: todayLabel, label: todayLabel }]}
          value={todayLabel}
          onChange={() => {}}
          placeholder={todayLabel}
        />
        <FilterSelect
          label="Wilayah"
          options={regions}
          value={regionFilter}
          onChange={setRegionFilter}
          placeholder="Semua wilayah"
        />
        <FilterSelect
          label="Vendor"
          options={vendors}
          value={vendorFilter}
          onChange={setVendorFilter}
          placeholder="Semua vendor"
        />
        <span className="text-sm text-[var(--n-500)] self-end pb-3">
          {filteredData.length} jadwal
        </span>
      </div>

      {/* Data table or empty state */}
      {filteredData.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 text-center">
          <Search className="h-12 w-12 text-[var(--n-300)] mb-3" aria-hidden="true" />
          <h3 className="text-base font-semibold text-[var(--n-800)] mb-1">
            Tidak ada jadwal ditemukan
          </h3>
          <p className="text-sm text-[var(--n-500)]">Coba nomor rute, vendor, atau wilayah lain.</p>
        </div>
      ) : (
        <div className="overflow-x-auto -mx-4 px-4 md:mx-0 md:px-0">
          <DataTable data={filteredData} columns={columns} />
        </div>
      )}
    </div>
  );
}
