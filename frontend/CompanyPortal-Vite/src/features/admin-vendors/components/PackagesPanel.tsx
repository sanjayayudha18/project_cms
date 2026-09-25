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
import { useDisableVendorPackage, useVendorPackages } from "../hooks";
import type { AdminVendorPackage } from "../types";
import { VendorPackageFormDialog } from "./VendorPackageFormDialog";

interface PackagesPanelProps {
  vendorId: number;
  branchId: number;
}

function tierLabel(pkg: AdminVendorPackage): string {
  return pkg.tier_max === null ? `${pkg.tier_min}+` : `${pkg.tier_min}-${pkg.tier_max}`;
}

function money(value: string | null): string {
  return value === null ? "—" : `IDR ${value}`;
}

/**
 * Branch special-price list ("harga khusus cabang", migration 016):
 * create/edit/disable, scoped to one branch. No Enable: a row is
 * effective-dated history, not a togglable entity -- Disable just closes
 * the period, same convention as PackagePricesPanel.
 */
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

  const [formPackage, setFormPackage] = useState<AdminVendorPackage | null>(null);
  const [isFormOpen, setFormOpen] = useState(false);
  const [pendingDisable, setPendingDisable] = useState<AdminVendorPackage | null>(null);

  const packages = query.data?.packages ?? [];

  function confirmDisable(): void {
    if (!pendingDisable) return;
    disableMutation.mutate(pendingDisable.id, {
      onSuccess: (res) => {
        toast({ type: "success", message: pendingApprovalMessage(res) });
        setPendingDisable(null);
      },
      onError: (err) => {
        toast({ type: "error", message: err.message });
        setPendingDisable(null);
      },
    });
  }

  const columns: ColumnDef<AdminVendorPackage, unknown>[] = [
    { accessorKey: "package_code", header: "Paket" },
    {
      id: "machine_class",
      header: "Mesin / Kelas",
      cell: ({ row }) => (
        <span>
          {row.original.machine_group === "ATM" ? "ATM" : "CDM/CRM"} ·{" "}
          {row.original.price_class === "REGULAR" ? "Regular" : "VIP/Industri"}
        </span>
      ),
    },
    { id: "tier", header: "Tingkat", cell: ({ row }) => tierLabel(row.original) },
    {
      id: "base_price",
      header: "Harga Khusus",
      meta: { align: "right" },
      cell: ({ row }) => <span className="tabular-nums">{money(row.original.base_price)}</span>,
    },
    { accessorKey: "effective_start_date", header: "Mulai" },
    {
      id: "effective_end_date",
      header: "Berakhir",
      cell: ({ row }) => row.original.effective_end_date ?? "Terbuka",
    },
    {
      id: "status",
      header: "Status",
      cell: ({ row }) => {
        const isOpenEnded =
          row.original.effective_end_date === null ||
          row.original.effective_end_date >= new Date().toISOString().slice(0, 10);
        return (
          <span className="inline-flex flex-wrap items-center gap-1">
            {isOpenEnded ? (
              <Badge variant="success" icon={CheckCircle} label="Berlaku" />
            ) : (
              <Badge variant="danger" icon={XCircle} label="Berakhir" />
            )}
            {pendingIds.has(row.original.id) && <PendingApprovalBadge />}
          </span>
        );
      },
    },
    {
      id: "actions",
      header: "Aksi",
      meta: { align: "right" },
      cell: ({ row }) => {
        const pkg = row.original;
        const isPending = pendingIds.has(pkg.id);
        const isOpenEnded =
          pkg.effective_end_date === null ||
          pkg.effective_end_date >= new Date().toISOString().slice(0, 10);
        return (
          <div className="flex justify-end gap-2">
            <Button
              variant="secondary"
              disabled={isPending || !isOpenEnded}
              onClick={() => {
                setFormPackage(pkg);
                setFormOpen(true);
              }}
            >
              Ubah
            </Button>
            {isOpenEnded && (
              <Button
                variant="danger"
                disabled={isPending}
                onClick={() => setPendingDisable(pkg)}
              >
                Akhiri
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
            setFormPackage(null);
            setFormOpen(true);
          }}
        >
          Tambah Harga Khusus
        </Button>
      </div>

      {query.isLoading && <p className="text-sm text-[var(--n-500)]">Memuat…</p>}
      {query.isError && (
        <p role="alert" className="text-sm text-[var(--danger-fg)]">
          Gagal memuat data paket cabang
        </p>
      )}
      {!query.isLoading && !query.isError && (
        <DataTable
          data={packages}
          columns={columns}
          emptyMessage="Cabang ini belum punya harga khusus"
        />
      )}

      <VendorPackageFormDialog
        open={isFormOpen}
        onClose={() => setFormOpen(false)}
        vendorId={vendorId}
        branchId={branchId}
        pkg={formPackage}
      />

      <ConfirmActionDialog
        open={pendingDisable !== null}
        title="Akhiri Harga Khusus"
        message={`Akhiri harga khusus paket "${pendingDisable?.package_code}" per hari ini? Periode ini menjadi riwayat, tidak bisa diaktifkan kembali -- buat baris baru untuk periode berikutnya.`}
        confirmLabel="Akhiri"
        isPending={disableMutation.isPending}
        onConfirm={confirmDisable}
        onClose={() => setPendingDisable(null)}
      />
    </div>
  );
}
