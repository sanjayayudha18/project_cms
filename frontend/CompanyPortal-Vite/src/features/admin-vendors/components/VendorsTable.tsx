import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { DataTable } from "@/components/ui/DataTable";
import { Link } from "@tanstack/react-router";
import type { ColumnDef } from "@tanstack/react-table";
import { CheckCircle, XCircle } from "lucide-react";
import { useMemo } from "react";
import { PendingApprovalBadge } from "../../master-data/PendingApprovalBadge";
import { formatMonitoring } from "../lib/pks";
import type { PksType } from "../pksApi";
import type { AdminVendor } from "../types";
import { PksStatusBadge } from "./PksStatusBadge";

interface VendorsTableProps {
  vendors: AdminVendor[];
  pksTypes?: PksType[];
  onEdit: (vendor: AdminVendor) => void;
  onDisable: (vendor: AdminVendor) => void;
  onEnable: (vendor: AdminVendor) => void;
  /** Records with a change waiting for approval: badged, and their actions locked. */
  pendingIds?: Set<number>;
}

/**
 * Vendors table (Req 5.2, 5.3, 5.6):
 * - Grouped columns per active PKS type (ordered by code): Status + Monitoring.
 * - Single PKS columns (Nama PKS, Tanggal Expired PKS, Monitoring PKS, Status PKS) removed.
 * - Status badge always carries icon + label (a11y).
 */
export function VendorsTable({
  vendors,
  pksTypes,
  onEdit,
  onDisable,
  onEnable,
  pendingIds,
}: VendorsTableProps) {
  const activeTypes = useMemo(() => {
    if (pksTypes && pksTypes.length > 0) {
      return pksTypes
        .filter((t) => t.is_active)
        .sort((a, b) => a.code.localeCompare(b.code))
        .map((t) => ({ code: t.code, name: t.name }));
    }
    // Fallback: extract active types from vendor PKS summaries if pksTypes prop is not provided
    const typeMap = new Map<string, string>();
    for (const v of vendors) {
      if (v.pks) {
        for (const p of v.pks) {
          if (!typeMap.has(p.pks_type)) {
            typeMap.set(p.pks_type, p.pks_type_name);
          }
        }
      }
    }
    return Array.from(typeMap.entries())
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([code, name]) => ({ code, name }));
  }, [pksTypes, vendors]);

  const pksColumns: ColumnDef<AdminVendor, unknown>[] = useMemo(
    () =>
      activeTypes.map((t) => ({
        id: `pks_group_${t.code}`,
        header: `PKS ${t.code}`,
        columns: [
          {
            id: `pks_${t.code}_status`,
            header: "Status",
            cell: ({ row }) => {
              if (row.original.kind === "INTERNAL") {
                return <span>-</span>;
              }
              const item = row.original.pks?.find((p) => p.pks_type === t.code);
              return <PksStatusBadge status={item?.pks_status ?? "belum_diisi"} />;
            },
          },
          {
            id: `pks_${t.code}_monitoring`,
            header: "Monitoring",
            cell: ({ row }) => {
              if (row.original.kind === "INTERNAL") {
                return <span className="tabular-nums">-</span>;
              }
              const item = row.original.pks?.find((p) => p.pks_type === t.code);
              return (
                <span className="tabular-nums">
                  {formatMonitoring(item?.pks_status ?? "belum_diisi", item?.remaining_days)}
                </span>
              );
            },
          },
        ],
      })),
    [activeTypes],
  );

  const columns: ColumnDef<AdminVendor, unknown>[] = useMemo(
    () => [
      {
        accessorKey: "code",
        header: "Kode",
        cell: ({ row }) => (
          <Link
            to="/settings/admin/vendors/$vendorId"
            params={{ vendorId: String(row.original.id) }}
            className="font-medium text-[var(--red-600)] underline"
          >
            {row.original.code}
          </Link>
        ),
      },
      { accessorKey: "name", header: "Nama" },
      {
        id: "director",
        header: "Nama Direktur Utama",
        cell: ({ row }) => row.original.director?.name ?? "-",
      },
      ...pksColumns,
      { accessorKey: "contact_email", header: "Email Kontak" },
      { accessorKey: "contact_phone", header: "Telepon" },
      {
        id: "status",
        header: "Status",
        cell: ({ row }) => (
          <span className="inline-flex flex-wrap items-center gap-1">
            {row.original.is_active ? (
              <Badge variant="success" icon={CheckCircle} label="Aktif" />
            ) : (
              <Badge variant="danger" icon={XCircle} label="Nonaktif" />
            )}
            {pendingIds?.has(row.original.id) && <PendingApprovalBadge />}
          </span>
        ),
      },
      {
        id: "actions",
        header: "Aksi",
        meta: { align: "right" },
        cell: ({ row }) => {
          const vendor = row.original;
          return (
            <div className="flex justify-end gap-2">
              <Button
                variant="secondary"
                disabled={pendingIds?.has(vendor.id)}
                onClick={() => onEdit(vendor)}
              >
                Ubah
              </Button>
              {vendor.is_active ? (
                <Button
                  variant="danger"
                  disabled={pendingIds?.has(vendor.id)}
                  onClick={() => onDisable(vendor)}
                >
                  Nonaktifkan
                </Button>
              ) : (
                <Button
                  variant="secondary"
                  disabled={pendingIds?.has(vendor.id)}
                  onClick={() => onEnable(vendor)}
                >
                  Aktifkan
                </Button>
              )}
            </div>
          );
        },
      },
    ],
    [pksColumns, onEdit, onDisable, onEnable, pendingIds],
  );

  return (
    <DataTable
      data={vendors}
      columns={columns}
      emptyMessage="Tidak ada vendor yang sesuai dengan filter saat ini"
    />
  );
}
