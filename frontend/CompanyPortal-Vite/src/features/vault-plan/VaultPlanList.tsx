import { Badge, type BadgeVariant } from "@/components/ui/Badge";
import { DataTable } from "@/components/ui/DataTable";
import { PageHeader } from "@/components/ui/PageHeader";
import { Link } from "@tanstack/react-router";
import type { ColumnDef } from "@tanstack/react-table";
import { AlertTriangle } from "lucide-react";
import { useState } from "react";
import {
  PLAN_STATUS_LABELS,
  type VaultPlanFilter,
  type VaultPlanStatus,
  type VaultPlanSummary,
  useVaultPlans,
} from "./api";

const inputClass =
  "min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm";

const STATUS_VARIANTS: Record<VaultPlanStatus, BadgeVariant> = {
  draft: "neutral",
  pending_acm_approval: "warning",
  acm_approved: "success",
  cancelled: "neutral",
};

export function PlanStatusBadge({ status }: { status: VaultPlanStatus }) {
  return <Badge variant={STATUS_VARIANTS[status]} label={PLAN_STATUS_LABELS[status]} />;
}

const columns: ColumnDef<VaultPlanSummary, unknown>[] = [
  {
    accessorKey: "request_number",
    header: "No. Request",
    cell: ({ row }) => (
      <Link
        to="/cit/vault-plans/$id"
        params={{ id: String(row.original.id) }}
        className="font-mono font-medium text-[var(--red-600)] hover:underline"
      >
        {row.original.request_number}
      </Link>
    ),
  },
  { accessorKey: "replenish_date", header: "Tgl Replenish" },
  { accessorKey: "acm_area_name", header: "Area ACM" },
  {
    accessorKey: "assigned_count",
    header: "ATM Ditetapkan",
    meta: { align: "right" },
    cell: ({ getValue }) => <span className="tabular-nums">{String(getValue())}</span>,
  },
  {
    accessorKey: "warning_count",
    header: "Peringatan Kapasitas",
    meta: { align: "right" },
    cell: ({ row }) =>
      row.original.warning_count > 0 ? (
        <Badge variant="warning" icon={AlertTriangle} label={`${row.original.warning_count} ATM`} />
      ) : (
        <span className="tabular-nums">0</span>
      ),
  },
  {
    accessorKey: "status",
    header: "Status",
    cell: ({ row }) => <PlanStatusBadge status={row.original.status} />,
  },
];

/** CIT -> Penetapan Vault (cit-acm-plan FR2): rencana vault per request x area ACM. */
export function VaultPlanList() {
  const [filter, setFilter] = useState<VaultPlanFilter>({ status: "draft", from: "", to: "" });
  const query = useVaultPlans(filter);

  return (
    <div className="flex flex-col gap-6 p-6">
      <PageHeader
        eyebrow="CIT"
        title="Penetapan Vault"
        description="Tetapkan branch vault untuk setiap ATM di request replenish area Anda, lalu kirim untuk approval ACM-SPV."
      />
      <div className="flex flex-wrap items-end gap-3">
        <label className="flex w-fit flex-col gap-1 text-sm font-medium text-[var(--n-700)]">
          Status
          <select
            value={filter.status}
            onChange={(e) => setFilter((f) => ({ ...f, status: e.target.value }))}
            className={inputClass}
          >
            {(Object.keys(PLAN_STATUS_LABELS) as VaultPlanStatus[]).map((s) => (
              <option key={s} value={s}>
                {PLAN_STATUS_LABELS[s]}
              </option>
            ))}
            <option value="">Semua</option>
          </select>
        </label>
        <label className="flex flex-col gap-1 text-sm font-medium text-[var(--n-700)]">
          Replenish dari
          <input
            type="date"
            value={filter.from}
            onChange={(e) => setFilter((f) => ({ ...f, from: e.target.value }))}
            className={inputClass}
          />
        </label>
        <label className="flex flex-col gap-1 text-sm font-medium text-[var(--n-700)]">
          sampai
          <input
            type="date"
            value={filter.to}
            onChange={(e) => setFilter((f) => ({ ...f, to: e.target.value }))}
            className={inputClass}
          />
        </label>
      </div>
      {query.isLoading && <p className="text-sm text-[var(--n-500)]">Memuat…</p>}
      {query.isError && (
        <p role="alert" className="text-sm text-[var(--danger-fg)]">
          Gagal memuat rencana vault
        </p>
      )}
      {query.data && (
        <DataTable data={query.data} columns={columns} emptyMessage="Tidak ada rencana vault" />
      )}
    </div>
  );
}
