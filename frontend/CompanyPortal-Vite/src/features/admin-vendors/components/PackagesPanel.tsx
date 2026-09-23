import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { ConfirmActionDialog } from "@/components/ui/ConfirmActionDialog";
import { DataTable } from "@/components/ui/DataTable";
import { useToast } from "@/lib/hooks/useToast";
import type { ColumnDef } from "@tanstack/react-table";
import { CheckCircle, XCircle } from "lucide-react";
import { useState } from "react";
import { PendingApprovalBadge } from "../../master-data/PendingApprovalBadge";
import { pendingApprovalMessage } from "../../master-data/changeRequest";
import { usePendingEntityIds } from "../../master-data/pending";
import { useDisableVendorPackage, useEnableVendorPackage, useVendorPackages } from "../hooks";
import type { AdminVendorPackage } from "../types";
import { VendorPackageFormDialog } from "./VendorPackageFormDialog";

interface PackagesPanelProps {
  vendorId: number;
  branchId: number;
}

/** Package list scoped to one branch: create/disable/enable (code is immutable, no edit). */
export function PackagesPanel({ vendorId, branchId }: PackagesPanelProps) {
  const { toast } = useToast();
  const query = useVendorPackages(vendorId, {
    page: 1,
    page_size: 100,
    status: "all",
    branch_id: branchId,
  });
  const pendingIds = usePendingEntityIds("vendor_package");
  const disableMutation = useDisableVendorPackage(vendorId);
  const enableMutation = useEnableVendorPackage(vendorId);

  const [isFormOpen, setFormOpen] = useState(false);
  const [pendingAction, setPendingAction] = useState<{
    type: "disable" | "enable";
    pkg: AdminVendorPackage;
  } | null>(null);

  const packages = query.data?.packages ?? [];

  function confirmPendingAction(): void {
    if (!pendingAction) return;
    const mutation = pendingAction.type === "disable" ? disableMutation : enableMutation;
    mutation.mutate(pendingAction.pkg.id, {
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

  const columns: ColumnDef<AdminVendorPackage, unknown>[] = [
    { accessorKey: "code", header: "Kode" },
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
        const pkg = row.original;
        const isPending = pendingIds.has(pkg.id);
        return (
          <div className="flex justify-end gap-2">
            {pkg.is_active ? (
              <Button
                variant="danger"
                disabled={isPending}
                onClick={() => setPendingAction({ type: "disable", pkg })}
              >
                Nonaktifkan
              </Button>
            ) : (
              <Button
                variant="secondary"
                disabled={isPending}
                onClick={() => setPendingAction({ type: "enable", pkg })}
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
    <div className="flex flex-col gap-3">
      <div className="flex justify-end">
        <Button onClick={() => setFormOpen(true)}>Tambah Paket</Button>
      </div>

      {query.isLoading && <p className="text-sm text-[var(--n-500)]">Memuat…</p>}
      {query.isError && (
        <p role="alert" className="text-sm text-[var(--danger-fg)]">
          Gagal memuat data paket
        </p>
      )}
      {!query.isLoading && !query.isError && (
        <DataTable data={packages} columns={columns} emptyMessage="Cabang ini belum punya paket" />
      )}

      <VendorPackageFormDialog
        open={isFormOpen}
        onClose={() => setFormOpen(false)}
        vendorId={vendorId}
        branchId={branchId}
      />

      <ConfirmActionDialog
        open={pendingAction !== null}
        title={pendingAction?.type === "disable" ? "Nonaktifkan Paket" : "Aktifkan Paket"}
        message={
          pendingAction?.type === "disable"
            ? `Nonaktifkan paket "${pendingAction.pkg.code}"?`
            : `Aktifkan kembali paket "${pendingAction?.pkg.code}"?`
        }
        confirmLabel={pendingAction?.type === "disable" ? "Nonaktifkan" : "Aktifkan"}
        isPending={disableMutation.isPending || enableMutation.isPending}
        onConfirm={confirmPendingAction}
        onClose={() => setPendingAction(null)}
      />
    </div>
  );
}
