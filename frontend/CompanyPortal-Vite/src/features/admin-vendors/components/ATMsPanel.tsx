import { Badge } from "@/components/ui/Badge";
import { DataTable } from "@/components/ui/DataTable";
import type { ColumnDef } from "@tanstack/react-table";
import { CheckCircle, XCircle } from "lucide-react";
import { useBranchATMs } from "../hooks";
import type { ManagedATM } from "../types";

interface ATMsPanelProps {
  vendorId: number;
  branchId: number;
}

function locationLabel(atm: ManagedATM): string {
  if (!atm.location_name) return "—";
  return atm.location_city_or_regency ? `${atm.location_name} · ${atm.location_city_or_regency}` : atm.location_name;
}

/**
 * Read-only "ATM" sub-tab (.kiro/specs/vendor-branch-atms): ATMs managed by
 * this branch through its packages' atm_vendor_packages assignments. No
 * create/edit/disable, no maker-checker -- display only.
 */
export function ATMsPanel({ vendorId, branchId }: ATMsPanelProps) {
  const query = useBranchATMs(vendorId, branchId, { page: 1, page_size: 100 });
  const atms = query.data?.atms ?? [];

  const columns: ColumnDef<ManagedATM, unknown>[] = [
    {
      accessorKey: "terminal_id",
      header: "Terminal ID",
      cell: ({ getValue }) => <span className="tabular-nums">{String(getValue())}</span>,
    },
    { id: "location", header: "Lokasi", cell: ({ row }) => locationLabel(row.original) },
    {
      accessorKey: "priority_class",
      header: "Priority Class",
      cell: ({ getValue }) => String(getValue() ?? "—"),
    },
    { accessorKey: "package_code", header: "Kode Paket" },
    {
      id: "status",
      header: "Status",
      cell: ({ row }) =>
        row.original.is_active ? (
          <Badge variant="success" icon={CheckCircle} label="Aktif" />
        ) : (
          <Badge variant="danger" icon={XCircle} label="Nonaktif" />
        ),
    },
  ];

  return (
    <div className="flex flex-col gap-3">
      {query.isLoading && <p className="text-sm text-[var(--n-500)]">Memuat…</p>}
      {query.isError && (
        <p role="alert" className="text-sm text-[var(--danger-fg)]">
          Gagal memuat data ATM cabang
        </p>
      )}
      {!query.isLoading && !query.isError && (
        <DataTable data={atms} columns={columns} emptyMessage="Cabang ini belum mengelola ATM" />
      )}
    </div>
  );
}
