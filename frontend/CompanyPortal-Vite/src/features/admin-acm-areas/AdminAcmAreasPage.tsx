import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { DataTable } from "@/components/ui/DataTable";
import { NoticeBanner } from "@/components/ui/NoticeBanner";
import { PageHeader } from "@/components/ui/PageHeader";
import { useToast } from "@/lib/hooks/useToast";
import type { ColumnDef } from "@tanstack/react-table";
import { AlertTriangle, CheckCircle, XCircle } from "lucide-react";
import { useState } from "react";
import { AcmAreaDetailPanel } from "./AcmAreaDetailPanel";
import {
  type AcmArea,
  type AcmAreaMutation,
  useAcmAreaMutation,
  useAcmAreaWarnings,
  useAcmAreas,
} from "./api";

const inputClass =
  "min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]";

/**
 * Pengaturan -> Area ACM (cit-acm-plan FR7, ADMIN). Areas group vendor branches
 * (one area per branch) and ACM-USER/ACM-SPV members. Changes apply immediately
 * and are audited.
 */
export function AdminAcmAreasPage() {
  const { toast } = useToast();
  const areasQuery = useAcmAreas();
  const warningsQuery = useAcmAreaWarnings();
  const mutation = useAcmAreaMutation();
  const [newName, setNewName] = useState("");
  const [selectedId, setSelectedId] = useState<number | null>(null);

  function run(m: AcmAreaMutation, success: string, onDone?: () => void): void {
    mutation.mutate(m, {
      onSuccess: () => {
        toast({ type: "success", message: success });
        onDone?.();
      },
      onError: (err) => toast({ type: "error", message: err.message }),
    });
  }

  function create(e: React.FormEvent): void {
    e.preventDefault();
    const name = newName.trim();
    if (!name) return;
    run({ op: "create", name }, `Area "${name}" dibuat`, () => setNewName(""));
  }

  const columns: ColumnDef<AcmArea, unknown>[] = [
    { accessorKey: "name", header: "Area" },
    {
      accessorKey: "branch_count",
      header: "Cabang",
      meta: { align: "right" },
      cell: ({ getValue }) => <span className="tabular-nums">{String(getValue())}</span>,
    },
    {
      accessorKey: "member_count",
      header: "Anggota",
      meta: { align: "right" },
      cell: ({ getValue }) => <span className="tabular-nums">{String(getValue())}</span>,
    },
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
    {
      id: "actions",
      header: "Aksi",
      meta: { align: "right" },
      cell: ({ row }) => {
        const a = row.original;
        return (
          <div className="flex justify-end gap-2">
            <Button variant="secondary" onClick={() => setSelectedId(a.id)}>
              Kelola
            </Button>
            <Button
              variant={a.is_active ? "danger" : "secondary"}
              disabled={mutation.isPending}
              onClick={() =>
                run(
                  { op: a.is_active ? "disable" : "enable", id: a.id },
                  a.is_active
                    ? `Area "${a.name}" dinonaktifkan; cabangnya dilepas`
                    : `Area "${a.name}" diaktifkan`,
                )
              }
            >
              {a.is_active ? "Nonaktifkan" : "Aktifkan"}
            </Button>
          </div>
        );
      },
    },
  ];

  const warnings = warningsQuery.data ?? [];

  return (
    <div className="flex flex-col gap-6 p-6">
      <PageHeader
        eyebrow="Pengaturan"
        title="Area ACM"
        description="Kelompok cabang vendor dan anggota tim ACM yang menyusun penetapan vault. Satu cabang hanya boleh di satu area."
      />

      {warnings.length > 0 && (
        <div className="flex flex-col gap-2">
          <NoticeBanner
            icon={AlertTriangle}
            variant="warning"
            title={`${warnings.length} cabang belum punya area ACM`}
            description="Request replenish untuk ATM cabang ini tertahan di tahap penetapan vault sampai cabangnya dimasukkan ke sebuah area."
          />
          <ul className="flex flex-col gap-1 text-sm text-[var(--n-700)]">
            {warnings.map((w) => (
              <li key={w.vendor_branch_id}>
                {w.vendor_name} · {w.branch_code} {w.branch_name} —{" "}
                <span className="tabular-nums">{w.request_count}</span> request
              </li>
            ))}
          </ul>
        </div>
      )}

      <form onSubmit={create} className="flex flex-wrap items-end gap-2">
        <label className="flex flex-col gap-1 text-sm font-medium text-[var(--n-700)]">
          Nama area baru
          <input
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            maxLength={100}
            className={inputClass}
          />
        </label>
        <Button type="submit" disabled={!newName.trim() || mutation.isPending}>
          Tambah Area
        </Button>
      </form>

      {areasQuery.isLoading && <p className="text-sm text-[var(--n-500)]">Memuat…</p>}
      {areasQuery.isError && (
        <p role="alert" className="text-sm text-[var(--danger-fg)]">
          Gagal memuat area ACM
        </p>
      )}
      {areasQuery.data && (
        <DataTable data={areasQuery.data} columns={columns} emptyMessage="Belum ada area ACM" />
      )}

      {selectedId !== null && (
        <AcmAreaDetailPanel
          key={selectedId}
          areaId={selectedId}
          onClose={() => setSelectedId(null)}
        />
      )}
    </div>
  );
}
