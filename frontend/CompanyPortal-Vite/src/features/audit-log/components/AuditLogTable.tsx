import { Badge } from "@/components/ui/Badge";
import { DataTable } from "@/components/ui/DataTable";
import { formatWibDateTimeSec } from "@/features/eod-monitoring/utils";
import type { ColumnDef } from "@tanstack/react-table";
import { CheckCircle2, Circle, Info, Pencil, XCircle } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useMemo } from "react";
import { type AuditLogListItem, type BadgeVariant, actionBadgeVariant } from "../types";

const ACTION_ICONS: Record<BadgeVariant, LucideIcon> = {
  success: CheckCircle2,
  danger: XCircle,
  info: Info,
  warning: Pencil,
  neutral: Circle,
};

/** Action badge always carries icon + label so colour is never the only signal (Sec 13). */
export function ActionBadge({ action }: { action: string }) {
  const variant = actionBadgeVariant(action);
  return <Badge variant={variant} icon={ACTION_ICONS[variant]} label={action} />;
}

interface AuditLogTableProps {
  rows: AuditLogListItem[];
  onSelect: (id: number) => void;
}

/** Server-ordered list (created_at DESC): client sorting is off so a page never re-orders itself. */
export function AuditLogTable({ rows, onSelect }: AuditLogTableProps) {
  // Memoized: TanStack Table treats a new `cell` function as a new component type, so
  // rebuilding columns every render remounts the row buttons and drops keyboard focus
  // (the detail dialog could then never return focus to the button that opened it).
  const columns = useMemo<ColumnDef<AuditLogListItem, unknown>[]>(
    () => [
      {
        id: "created_at",
        header: "Waktu (WIB)",
        enableSorting: false,
        cell: ({ row }) => (
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              onSelect(row.original.id);
            }}
            className="min-h-[44px] cursor-pointer text-left tabular-nums text-[var(--red-600)] underline-offset-2 outline-none hover:underline focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
          >
            {formatWibDateTimeSec(row.original.created_at)}
          </button>
        ),
      },
      {
        id: "actor_id",
        header: "Aktor",
        enableSorting: false,
        cell: ({ row }) => <span className="tabular-nums">#{row.original.actor_id}</span>,
      },
      {
        id: "action",
        header: "Aksi",
        enableSorting: false,
        cell: ({ row }) => <ActionBadge action={row.original.action} />,
      },
      { accessorKey: "entity_type", header: "Tipe Entitas", enableSorting: false },
      {
        id: "entity_id",
        header: "ID Entitas",
        enableSorting: false,
        cell: ({ row }) => <span className="tabular-nums">{row.original.entity_id}</span>,
      },
      {
        id: "ip",
        header: "IP",
        enableSorting: false,
        cell: ({ row }) => <span className="tabular-nums">{row.original.ip ?? "-"}</span>,
      },
    ],
    [onSelect],
  );

  return (
    <DataTable
      data={rows}
      columns={columns}
      onRowClick={(row) => onSelect(row.id)}
      emptyMessage="Tidak ada log audit"
    />
  );
}
