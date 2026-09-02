import { Badge } from "@/components/ui/Badge";
import { DataTable } from "@/components/ui/DataTable";
import { DatePicker } from "@/components/ui/DatePicker";
import { EmptyState } from "@/components/ui/EmptyState";
import { FilterTabs } from "@/components/ui/FilterTabs";
import { OrderSummaryBar } from "@/features/orders/OrderSummaryBar";
import { useOrders } from "@/features/orders/useOrders";
import { formatIDR } from "@/lib/formatters";
import type { CITOrder } from "@/lib/types";
import { type ColumnDef, type SortingState, createColumnHelper } from "@tanstack/react-table";
import { PackageSearch } from "lucide-react";
import { useMemo, useState } from "react";

type OrderStatus = CITOrder["status"];
type StatusFilter = "All" | OrderStatus;

const statusBadgeMap: Record<OrderStatus, "info" | "warning" | "success" | "danger"> = {
  Scheduled: "info",
  "In Transit": "warning",
  Completed: "success",
  Failed: "danger",
};

const columnHelper = createColumnHelper<CITOrder>();

const columns: ColumnDef<CITOrder, unknown>[] = [
  columnHelper.accessor("id", {
    header: "Order ID",
    cell: (info) => info.getValue(),
  }),
  columnHelper.accessor("atmId", {
    header: "ATM ID",
    cell: (info) => info.getValue(),
  }),
  columnHelper.accessor("location", {
    header: "Location",
    cell: (info) => info.getValue(),
  }),
  columnHelper.accessor("orderType", {
    header: "Order Type",
    cell: (info) => info.getValue(),
  }),
  columnHelper.accessor("scheduledDate", {
    header: "Scheduled Date",
    cell: (info) => {
      const date = new Date(info.getValue());
      return date.toLocaleDateString("id-ID", {
        day: "2-digit",
        month: "short",
        year: "numeric",
      });
    },
  }),
  columnHelper.accessor("amount", {
    header: "Amount",
    cell: (info) => formatIDR(info.getValue()),
    meta: { numeric: true },
  }),
  columnHelper.accessor("status", {
    header: "Status",
    cell: (info) => {
      const status = info.getValue();
      return <Badge variant={statusBadgeMap[status]}>{status}</Badge>;
    },
    enableSorting: false,
  }),
] as ColumnDef<CITOrder, unknown>[];

export function OrdersPage() {
  const { data: orders = [], isLoading } = useOrders();

  const [statusFilter, setStatusFilter] = useState<StatusFilter>("All");
  const [startDate, setStartDate] = useState<string | null>(null);
  const [endDate, setEndDate] = useState<string | null>(null);
  const [sorting, setSorting] = useState<SortingState>([{ id: "scheduledDate", desc: true }]);

  // Compute counts per status from UNFILTERED data (for filter tabs)
  const statusCounts = useMemo(() => {
    const counts: Record<StatusFilter, number> = {
      All: orders.length,
      Scheduled: 0,
      "In Transit": 0,
      Completed: 0,
      Failed: 0,
    };
    for (const order of orders) {
      counts[order.status]++;
    }
    return counts;
  }, [orders]);

  // Apply filters: status AND date range (AND logic)
  const filteredOrders = useMemo(() => {
    let result = orders;

    // Status filter
    if (statusFilter !== "All") {
      result = result.filter((o) => o.status === statusFilter);
    }

    // Date range filter (inclusive)
    if (startDate) {
      result = result.filter((o) => o.scheduledDate >= startDate);
    }
    if (endDate) {
      result = result.filter((o) => o.scheduledDate <= endDate);
    }

    return result;
  }, [orders, statusFilter, startDate, endDate]);

  // Filter tab options with counts
  const filterOptions: readonly { value: StatusFilter; label: string; count: number }[] = [
    { value: "All", label: "All", count: statusCounts.All },
    { value: "Scheduled", label: "Scheduled", count: statusCounts.Scheduled },
    { value: "In Transit", label: "In Transit", count: statusCounts["In Transit"] },
    { value: "Completed", label: "Completed", count: statusCounts.Completed },
    { value: "Failed", label: "Failed", count: statusCounts.Failed },
  ];

  if (isLoading) {
    return (
      <div className="flex flex-col gap-6">
        <h1 className="text-xl font-semibold text-surface-text">CIT Orders</h1>
        <p className="text-sm text-neutral-500">Memuat data...</p>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-xl font-semibold text-surface-text">CIT Orders</h1>

      {/* Summary bar — always uses unfiltered data */}
      <OrderSummaryBar orders={orders} />

      {/* Filters */}
      <div className="flex flex-col gap-4">
        <FilterTabs options={filterOptions} selected={statusFilter} onChange={setStatusFilter} />
        <DatePicker
          startDate={startDate}
          endDate={endDate}
          onStartDateChange={setStartDate}
          onEndDateChange={setEndDate}
        />
      </div>

      {/* Data table or empty state */}
      {filteredOrders.length === 0 ? (
        <EmptyState
          icon={PackageSearch}
          title="Tidak ada order yang cocok"
          description="Tidak ditemukan order CIT yang sesuai dengan filter yang dipilih."
        />
      ) : (
        <DataTable
          data={filteredOrders}
          columns={columns}
          sorting={sorting}
          onSortingChange={setSorting}
        />
      )}
    </div>
  );
}
