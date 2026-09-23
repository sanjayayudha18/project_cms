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
import { useDisableVendorVault, useEnableVendorVault, useVendorVaults } from "../hooks";
import type { AdminVendorVault } from "../types";
import { VendorVaultFormDialog } from "./VendorVaultFormDialog";

interface VaultsPanelProps {
  vendorId: number;
  branchId: number;
}

/** Vault list scoped to one branch: create/edit/disable/enable. */
export function VaultsPanel({ vendorId, branchId }: VaultsPanelProps) {
  const { toast } = useToast();
  const query = useVendorVaults(vendorId, {
    page: 1,
    page_size: 100,
    status: "all",
    branch_id: branchId,
  });
  const pendingIds = usePendingEntityIds("vendor_vault");
  const disableMutation = useDisableVendorVault(vendorId);
  const enableMutation = useEnableVendorVault(vendorId);

  const [formVault, setFormVault] = useState<AdminVendorVault | null>(null);
  const [isFormOpen, setFormOpen] = useState(false);
  const [pendingAction, setPendingAction] = useState<{
    type: "disable" | "enable";
    vault: AdminVendorVault;
  } | null>(null);

  const vaults = query.data?.vaults ?? [];

  function confirmPendingAction(): void {
    if (!pendingAction) return;
    const mutation = pendingAction.type === "disable" ? disableMutation : enableMutation;
    mutation.mutate(pendingAction.vault.id, {
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

  const columns: ColumnDef<AdminVendorVault, unknown>[] = [
    { accessorKey: "vault_code", header: "Kode" },
    { accessorKey: "category", header: "Kategori" },
    {
      accessorKey: "max_capacity_amount",
      header: "Kapasitas Maks",
      meta: { align: "right" },
      cell: ({ getValue }) => <span className="tabular-nums">{String(getValue() ?? "—")}</span>,
    },
    {
      accessorKey: "operating_hours",
      header: "Jam Operasional",
      cell: ({ getValue }) => String(getValue() ?? "—"),
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
        const vault = row.original;
        const isPending = pendingIds.has(vault.id);
        return (
          <div className="flex justify-end gap-2">
            <Button
              variant="secondary"
              disabled={isPending}
              onClick={() => {
                setFormVault(vault);
                setFormOpen(true);
              }}
            >
              Ubah
            </Button>
            {vault.is_active ? (
              <Button
                variant="danger"
                disabled={isPending}
                onClick={() => setPendingAction({ type: "disable", vault })}
              >
                Nonaktifkan
              </Button>
            ) : (
              <Button
                variant="secondary"
                disabled={isPending}
                onClick={() => setPendingAction({ type: "enable", vault })}
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
        <Button
          onClick={() => {
            setFormVault(null);
            setFormOpen(true);
          }}
        >
          Tambah Vault
        </Button>
      </div>

      {query.isLoading && <p className="text-sm text-[var(--n-500)]">Memuat…</p>}
      {query.isError && (
        <p role="alert" className="text-sm text-[var(--danger-fg)]">
          Gagal memuat data vault
        </p>
      )}
      {!query.isLoading && !query.isError && (
        <DataTable data={vaults} columns={columns} emptyMessage="Cabang ini belum punya vault" />
      )}

      <VendorVaultFormDialog
        open={isFormOpen}
        onClose={() => setFormOpen(false)}
        vendorId={vendorId}
        branchId={branchId}
        vault={formVault}
      />

      <ConfirmActionDialog
        open={pendingAction !== null}
        title={pendingAction?.type === "disable" ? "Nonaktifkan Vault" : "Aktifkan Vault"}
        message={
          pendingAction?.type === "disable"
            ? `Nonaktifkan vault "${pendingAction.vault.vault_code}"?`
            : `Aktifkan kembali vault "${pendingAction?.vault.vault_code}"?`
        }
        confirmLabel={pendingAction?.type === "disable" ? "Nonaktifkan" : "Aktifkan"}
        isPending={disableMutation.isPending || enableMutation.isPending}
        onConfirm={confirmPendingAction}
        onClose={() => setPendingAction(null)}
      />
    </div>
  );
}
