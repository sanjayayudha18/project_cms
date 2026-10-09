import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { DataTable } from "@/components/ui/DataTable";
import { DatePicker } from "@/components/ui/DatePicker";
import { EmptyState } from "@/components/ui/EmptyState";
import { FilterTabs } from "@/components/ui/FilterTabs";
import { ORDER_PAGE_SIZE, type PartyStatus, type VendorOrderSummary } from "@/features/orders/api";
import { ROLE_LABEL, formatDate, orderStatus, totalsText } from "@/features/orders/labels";
import { useOrders } from "@/features/orders/useOrders";
import { Link } from "@tanstack/react-router";
import { type ColumnDef, createColumnHelper } from "@tanstack/react-table";
import { PackageSearch } from "lucide-react";
import { useState } from "react";

// cit-send-vendor FR9.1: replenish requests sent to this vendor, one row per
// (request, branch, role). Filters and paging run on the server.

type StatusFilter = "all" | PartyStatus;

const FILTERS: readonly { value: StatusFilter; label: string }[] = [
  { value: "all", label: "Semua" },
  { value: "pending", label: "Menunggu keputusan" },
  { value: "accepted", label: "Diterima" },
  { value: "rejected", label: "Ditolak" },
  { value: "withdrawn", label: "Ditarik" },
];

const columnHelper = createColumnHelper<VendorOrderSummary>();

const columns = [
  columnHelper.accessor("request_number", {
    header: "Nomor Request",
    cell: (info) => (
      <Link
        to="/orders/$id"
        params={{ id: String(info.row.original.id) }}
        className="font-medium text-sidebar-active underline-offset-2 hover:underline"
      >
        {info.getValue()}
      </Link>
    ),
    enableSorting: false,
  }),
  columnHelper.accessor("replenish_date", {
    header: "Tanggal Replenish",
    cell: (info) => formatDate(info.getValue()),
    enableSorting: false,
  }),
  columnHelper.accessor("role", {
    header: "Peran",
    cell: (info) => ROLE_LABEL[info.getValue()],
    enableSorting: false,
  }),
  columnHelper.accessor("branch_name", {
    header: "Branch",
    cell: (info) => info.getValue(),
    enableSorting: false,
  }),
  columnHelper.accessor("atm_count", {
    header: "Jumlah ATM",
    cell: (info) => info.getValue(),
    meta: { numeric: true },
    enableSorting: false,
  }),
  columnHelper.accessor("totals", {
    header: "Total per Denom",
    cell: (info) => totalsText(info.getValue()),
    meta: { numeric: true },
    enableSorting: false,
  }),
  columnHelper.display({
    id: "status",
    header: "Status",
    cell: (info) => {
      const s = orderStatus(info.row.original);
      return <Badge variant={s.variant}>{s.label}</Badge>;
    },
  }),
] as ColumnDef<VendorOrderSummary, unknown>[];

export function OrdersPage() {
  const [status, setStatus] = useState<StatusFilter>("all");
  const [from, setFrom] = useState<string | null>(null);
  const [to, setTo] = useState<string | null>(null);
  const [page, setPage] = useState(1);

  const { data, isLoading, isError } = useOrders({
    partyStatus: status === "all" ? "" : status,
    from,
    to,
    page,
  });
  const items = data?.items ?? [];
  const lastPage = Math.max(1, Math.ceil((data?.total ?? 0) / ORDER_PAGE_SIZE));

  function handleStatus(v: StatusFilter) {
    setStatus(v);
    setPage(1);
  }
  function handleFrom(v: string | null) {
    setFrom(v);
    setPage(1);
  }
  function handleTo(v: string | null) {
    setTo(v);
    setPage(1);
  }

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-xl font-semibold text-surface-text">CIT Orders</h1>

      <div className="flex flex-col gap-4">
        <FilterTabs options={FILTERS} selected={status} onChange={handleStatus} />
        <DatePicker
          startDate={from}
          endDate={to}
          onStartDateChange={handleFrom}
          onEndDateChange={handleTo}
        />
      </div>

      {isLoading ? (
        <p className="text-sm text-neutral-500">Memuat data...</p>
      ) : isError ? (
        <p role="alert" className="text-sm font-semibold text-danger-fg">
          Gagal memuat request. Coba muat ulang halaman.
        </p>
      ) : items.length === 0 ? (
        <EmptyState
          icon={PackageSearch}
          title="Belum ada request"
          description="Tidak ada request replenish yang sesuai dengan filter yang dipilih."
        />
      ) : (
        <>
          <DataTable data={items} columns={columns} />
          <nav aria-label="Halaman" className="flex items-center justify-end gap-3 text-sm">
            <Button
              variant="secondary"
              size="sm"
              disabled={page <= 1}
              onClick={() => setPage(page - 1)}
            >
              Sebelumnya
            </Button>
            <span className="tabular-nums">
              Halaman {page} dari {lastPage}
            </span>
            <Button
              variant="secondary"
              size="sm"
              disabled={page >= lastPage}
              onClick={() => setPage(page + 1)}
            >
              Berikutnya
            </Button>
          </nav>
        </>
      )}
    </div>
  );
}
