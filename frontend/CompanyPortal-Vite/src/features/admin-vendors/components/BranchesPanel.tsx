import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { ConfirmActionDialog } from "@/components/ui/ConfirmActionDialog";
import { DataTable } from "@/components/ui/DataTable";
import { useToast } from "@/lib/hooks/useToast";
import { Link } from "@tanstack/react-router";
import type { ColumnDef } from "@tanstack/react-table";
import { CheckCircle, XCircle } from "lucide-react";
import { useState } from "react";
import { PendingApprovalBadge } from "../../master-data/PendingApprovalBadge";
import { pendingApprovalMessage } from "../../master-data/changeRequest";
import { usePendingEntityIds } from "../../master-data/pending";
import { useDisableVendorBranch, useEnableVendorBranch, useVendorBranches } from "../hooks";
import type { AdminVendorBranch } from "../types";
import { VendorBranchFormDialog } from "./VendorBranchFormDialog";

interface BranchesPanelProps {
  vendorId: number;
}

/**
 * Cabang list for a vendor: clickable rows navigate to a dedicated branch
 * detail page (vaults/PICs/packages), plus create/edit/disable/enable for
 * the branch itself.
 */
export function BranchesPanel({ vendorId }: BranchesPanelProps) {
  const { toast } = useToast();
  const query = useVendorBranches(vendorId, { page: 1, page_size: 100, status: "all" });
  const pendingIds = usePendingEntityIds("vendor_branch");
  const disableMutation = useDisableVendorBranch(vendorId);
  const enableMutation = useEnableVendorBranch(vendorId);

  const [isFormOpen, setFormOpen] = useState(false);
  const [pendingAction, setPendingAction] = useState<{
    type: "disable" | "enable";
    branch: AdminVendorBranch;
  } | null>(null);

  const branches = query.data?.branches ?? [];

  function confirmPendingAction(): void {
    if (!pendingAction) return;
    const mutation = pendingAction.type === "disable" ? disableMutation : enableMutation;
    mutation.mutate(pendingAction.branch.id, {
      onSuccess: (res) => {
        toast({ type: "success", message: pendingApprovalMessage(res) });
        setPendingAction(null);
      },
      onError: (err) => {
        toast({ type: "error", message: err.message });
        setPendingAction(null);
      },
    });
  }

  const columns: ColumnDef<AdminVendorBranch, unknown>[] = [
    {
      accessorKey: "branch_code",
      header: "Kode",
      cell: ({ row }) => (
        <Link
          to="/settings/admin/vendors/$vendorId/branches/$branchId"
          params={{ vendorId: String(vendorId), branchId: String(row.original.id) }}
          className="font-medium text-[var(--red-600)] underline"
        >
          {row.original.branch_code}
        </Link>
      ),
    },
    { accessorKey: "branch_name", header: "Nama" },
    { accessorKey: "region", header: "Wilayah", cell: ({ getValue }) => String(getValue() ?? "—") },
    {
      accessorKey: "category",
      header: "Tipe",
      cell: ({ getValue }) => (
        <span className="inline-flex items-center rounded-full bg-[var(--n-100)] px-2 py-0.5 text-xs font-medium text-[var(--n-600)]">
          {String(getValue())}
        </span>
      ),
    },
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
          {pendingIds.has(row.original.id) && <PendingApprovalBadge />}
        </span>
      ),
    },
    {
      id: "actions",
      header: "Aksi",
      meta: { align: "right" },
      cell: ({ row }) => {
        const branch = row.original;
        const isPending = pendingIds.has(branch.id);
        return (
          <div className="flex justify-end gap-2">
            <Link
              to="/settings/admin/vendors/$vendorId/branches/$branchId/edit"
              params={{ vendorId: String(vendorId), branchId: String(branch.id) }}
            >
              <Button variant="secondary" disabled={isPending}>
                Ubah
              </Button>
            </Link>
            {branch.is_active ? (
              <Button
                variant="danger"
                disabled={isPending}
                onClick={() => setPendingAction({ type: "disable", branch })}
              >
                Nonaktifkan
              </Button>
            ) : (
              <Button
                variant="secondary"
                disabled={isPending}
                onClick={() => setPendingAction({ type: "enable", branch })}
              >
                Aktifkan
              </Button>
            )}
          </div>
        );
      },
    },
  ];

  return (
    <div className="flex flex-col gap-3 rounded-lg border border-[var(--n-200)] bg-[var(--n-0)] p-4">
      <div className="flex justify-end">
        <Button onClick={() => setFormOpen(true)}>Tambah Cabang</Button>
      </div>

      {query.isLoading && <p className="text-sm text-[var(--n-500)]">Memuat…</p>}
      {query.isError && (
        <p role="alert" className="text-sm text-[var(--danger-fg)]">
          Gagal memuat data cabang
        </p>
      )}
      {!query.isLoading && !query.isError && (
        <DataTable data={branches} columns={columns} emptyMessage="Vendor ini belum punya cabang" />
      )}

      <VendorBranchFormDialog
        open={isFormOpen}
        onClose={() => setFormOpen(false)}
        vendorId={vendorId}
      />

      <ConfirmActionDialog
        open={pendingAction !== null}
        title={pendingAction?.type === "disable" ? "Nonaktifkan Cabang" : "Aktifkan Cabang"}
        message={
          pendingAction?.type === "disable"
            ? `Nonaktifkan cabang "${pendingAction.branch.branch_name}"? Ditolak jika cabang masih punya vault, PIC, atau paket aktif.`
            : `Aktifkan kembali cabang "${pendingAction?.branch.branch_name}"?`
        }
        confirmLabel={pendingAction?.type === "disable" ? "Nonaktifkan" : "Aktifkan"}
        isPending={disableMutation.isPending || enableMutation.isPending}
        onConfirm={confirmPendingAction}
        onClose={() => setPendingAction(null)}
      />
    </div>
  );
}
