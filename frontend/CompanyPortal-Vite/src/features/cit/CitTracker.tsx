import { useMemo, useState } from "react";

import { EmptyState } from "@/components/ui/EmptyState";
import { FilterSelect } from "@/components/ui/FilterSelect";
import vendorsData from "@/data/vendors.json";
import { compoundFilter } from "@/lib/utils/filters";

import { CitSummary } from "./CitSummary";
import { CitTable } from "./CitTable";
import type { CitStatus } from "./types";
import { useCitData } from "./useCitData";

const statusOptions: { value: string; label: string }[] = [
  { value: "Scheduled", label: "Scheduled" },
  { value: "In Transit", label: "In Transit" },
  { value: "Completed", label: "Completed" },
  { value: "Failed", label: "Failed" },
];

const vendorOptions = vendorsData.map((v) => ({
  value: v.id,
  label: v.name,
}));

export function CitTracker() {
  const { data: orders = [] } = useCitData();
  const [statusFilter, setStatusFilter] = useState<CitStatus | null>(null);
  const [vendorFilter, setVendorFilter] = useState<string | null>(null);

  const filteredOrders = useMemo(
    () =>
      compoundFilter(orders, {
        status: statusFilter,
        vendorId: vendorFilter,
      }),
    [orders, statusFilter, vendorFilter],
  );

  return (
    <div className="flex flex-col gap-6 p-6">
      <h1 className="text-xl font-semibold text-[var(--n-900)]">CIT Tracker</h1>

      <div className="flex flex-wrap items-end gap-4">
        <FilterSelect
          label="Status"
          options={statusOptions}
          value={statusFilter}
          onChange={(val) => setStatusFilter(val as CitStatus | null)}
        />
        <FilterSelect
          label="Vendor"
          options={vendorOptions}
          value={vendorFilter}
          onChange={setVendorFilter}
        />
      </div>

      <CitSummary data={filteredOrders} />

      {filteredOrders.length === 0 ? (
        <EmptyState message="Tidak ada order CIT yang sesuai filter" />
      ) : (
        <CitTable data={filteredOrders} />
      )}
    </div>
  );
}
