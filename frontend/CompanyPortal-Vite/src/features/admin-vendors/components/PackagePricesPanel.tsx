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
import { useDisableVendorPackagePrice, useVendorPackagePrices } from "../hooks";
import type { AdminVendorPackagePrice } from "../types";
import { VendorPackagePriceFormDialog } from "./VendorPackagePriceFormDialog";

interface PackagePricesPanelProps {
  vendorId: number;
}

function levelLabel(price: AdminVendorPackagePrice): string {
  if (price.atm_id !== null) return `ATM #${price.atm_id}`;
  if (price.vendor_branch_id !== null) return `Cabang #${price.vendor_branch_id}`;
  return "PT (dasar)";
}

function tierLabel(price: AdminVendorPackagePrice): string {
  return price.tier_max === null ? `${price.tier_min}+` : `${price.tier_min}-${price.tier_max}`;
}

function money(value: string | null): string {
  return value === null ? "—" : `IDR ${value}`;
}

/**
 * Vendor package price list: create/edit/disable, vendor-scoped (not
 * per-branch -- vendor_branch_id/atm_id are optional override columns shown
 * per row). No Enable: a price row is effective-dated history, not a
 * togglable entity -- Disable just closes the period.
 */
export function PackagePricesPanel({ vendorId }: PackagePricesPanelProps) {
  const { toast } = useToast();
  const query = useVendorPackagePrices(vendorId, { page: 1, page_size: 100, status: "all" });
  const pendingIds = usePendingEntityIds("vendor_package_price");
  const disableMutation = useDisableVendorPackagePrice(vendorId);

  const [formPrice, setFormPrice] = useState<AdminVendorPackagePrice | null>(null);
  const [isFormOpen, setFormOpen] = useState(false);
  const [pendingDisable, setPendingDisable] = useState<AdminVendorPackagePrice | null>(null);

  const prices = query.data?.package_prices ?? [];

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

  const columns: ColumnDef<AdminVendorPackagePrice, unknown>[] = [
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
    { id: "level", header: "Tingkat Harga", cell: ({ row }) => levelLabel(row.original) },
    {
      id: "base_price",
      header: "Harga Dasar",
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
        const price = row.original;
        const isPending = pendingIds.has(price.id);
        const isOpenEnded =
          price.effective_end_date === null ||
          price.effective_end_date >= new Date().toISOString().slice(0, 10);
        return (
          <div className="flex justify-end gap-2">
            <Button
              variant="secondary"
              disabled={isPending || !isOpenEnded}
              onClick={() => {
                setFormPrice(price);
                setFormOpen(true);
              }}
            >
              Ubah
            </Button>
            {isOpenEnded && (
              <Button
                variant="danger"
                disabled={isPending}
                onClick={() => setPendingDisable(price)}
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
            setFormPrice(null);
            setFormOpen(true);
          }}
        >
          Tambah Harga Paket
        </Button>
      </div>

      {query.isLoading && <p className="text-sm text-[var(--n-500)]">Memuat…</p>}
      {query.isError && (
        <p role="alert" className="text-sm text-[var(--danger-fg)]">
          Gagal memuat data harga paket
        </p>
      )}
      {!query.isLoading && !query.isError && (
        <DataTable
          data={prices}
          columns={columns}
          emptyMessage="Vendor ini belum punya harga paket"
        />
      )}

      <VendorPackagePriceFormDialog
        open={isFormOpen}
        onClose={() => setFormOpen(false)}
        vendorId={vendorId}
        price={formPrice}
      />

      <ConfirmActionDialog
        open={pendingDisable !== null}
        title="Akhiri Periode Harga"
        message={`Akhiri harga paket "${pendingDisable?.package_code}" per hari ini? Periode ini menjadi riwayat, tidak bisa diaktifkan kembali -- buat baris baru untuk periode berikutnya.`}
        confirmLabel="Akhiri"
        isPending={disableMutation.isPending}
        onConfirm={confirmDisable}
        onClose={() => setPendingDisable(null)}
      />
    </div>
  );
}
