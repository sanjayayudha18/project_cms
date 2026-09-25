import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { ConfirmActionDialog } from "@/components/ui/ConfirmActionDialog";
import { DataTable } from "@/components/ui/DataTable";
import { useToast } from "@/lib/hooks/useToast";
import type { ColumnDef } from "@tanstack/react-table";
import { CalendarClock, CheckCircle, XCircle } from "lucide-react";
import { useMemo, useState } from "react";
import { PendingApprovalBadge } from "../../master-data/PendingApprovalBadge";
import { pendingApprovalMessage } from "../../master-data/changeRequest";
import { usePendingEntityIds } from "../../master-data/pending";
import { useDisableVendorPackagePrice, useVendorPackagePrices } from "../hooks";
import {
  type PriceStatus,
  classLabel,
  formatIDR,
  levelLabel,
  machineLabel,
  periodLabel,
  priceStatus,
  tierLabel,
} from "../lib/packagePrice";
import type { AdminVendorPackagePrice } from "../types";
import { PriceFilterBar, type PriceFilters } from "./PriceFilterBar";
import { VendorPackagePriceFormDialog } from "./VendorPackagePriceFormDialog";

interface PackagePricesPanelProps {
  vendorId: number;
}

const STATUS_BADGE: Record<
  PriceStatus,
  { variant: "success" | "warning" | "neutral"; icon: typeof CheckCircle }
> = {
  Berlaku: { variant: "success", icon: CheckCircle },
  Dijadwalkan: { variant: "warning", icon: CalendarClock },
  Berakhir: { variant: "neutral", icon: XCircle },
};

function todayISO(): string {
  return new Date().toISOString().slice(0, 10);
}

function matchesFilters(p: AdminVendorPackagePrice, filters: PriceFilters, today: string): boolean {
  if (filters.package !== null && p.package !== filters.package) return false;
  if (filters.machine_group !== null && p.machine_group !== filters.machine_group) return false;
  if (filters.status !== null && priceStatus(p, today) !== filters.status) return false;
  return true;
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
  const [filters, setFilters] = useState<PriceFilters>({
    package: null,
    machine_group: null,
    status: null,
  });

  const prices = query.data?.package_prices ?? [];
  const today = todayISO();

  const packageOptions = useMemo(
    () => Array.from(new Set(prices.map((p) => p.package))).sort(),
    [prices],
  );
  const machineOptions = useMemo(
    () => Array.from(new Set(prices.map((p) => p.machine_group))).sort() as ("ATM" | "CDM_CRM")[],
    [prices],
  );

  const filtered = useMemo(
    () => prices.filter((p) => matchesFilters(p, filters, today)),
    [prices, filters, today],
  );

  const sorted = useMemo(
    () =>
      [...filtered].sort((a, b) => {
        if (a.package !== b.package) return a.package.localeCompare(b.package);
        if (a.machine_group !== b.machine_group)
          return a.machine_group.localeCompare(b.machine_group);
        if (a.price_class !== b.price_class) return a.price_class.localeCompare(b.price_class);
        return a.tier_min - b.tier_min;
      }),
    [filtered],
  );

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
    {
      accessorKey: "package_code",
      header: "Kode Paket",
      cell: ({ row }) => (
        <span className="font-mono tabular-nums">{row.original.package_code}</span>
      ),
    },
    { accessorKey: "package", header: "Paket" },
    { id: "machine", header: "Mesin", cell: ({ row }) => machineLabel(row.original.machine_group) },
    { id: "class", header: "Kelas", cell: ({ row }) => classLabel(row.original.price_class) },
    { id: "level", header: "Tingkat Harga", cell: ({ row }) => levelLabel(row.original) },
    { id: "tier", header: "Tingkat", cell: ({ row }) => tierLabel(row.original) },
    {
      id: "base_price",
      header: "Harga Dasar",
      meta: { align: "right" },
      cell: ({ row }) => (
        <span className="tabular-nums">
          {row.original.base_price === null ? "—" : formatIDR(row.original.base_price)}
        </span>
      ),
    },
    {
      id: "period",
      header: "Periode",
      cell: ({ row }) => {
        const period = periodLabel(row.original);
        return (
          <span>
            {period.start} → {period.end}
          </span>
        );
      },
    },
    {
      id: "status",
      header: "Status",
      cell: ({ row }) => {
        const status = priceStatus(row.original, today);
        const { variant, icon } = STATUS_BADGE[status];
        return (
          <span className="inline-flex flex-wrap items-center gap-1">
            <Badge variant={variant} icon={icon} label={status} />
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
        const status = priceStatus(price, today);

        if (status === "Berakhir") {
          return (
            <span className="text-sm text-[var(--n-500)]">Diakhiri {price.effective_end_date}</span>
          );
        }

        return (
          <div className="flex justify-end gap-2">
            <Button
              variant="secondary"
              disabled={isPending}
              onClick={() => {
                setFormPrice(price);
                setFormOpen(true);
              }}
            >
              Ubah
            </Button>
            {status === "Berlaku" && (
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

  const noMatchesFromNonEmptyList = prices.length > 0 && filtered.length === 0;

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

      <PriceFilterBar
        packageOptions={packageOptions}
        machineOptions={machineOptions}
        value={filters}
        onChange={setFilters}
        matchCount={filtered.length}
      />

      {query.isLoading && <p className="text-sm text-[var(--n-500)]">Memuat…</p>}
      {query.isError && (
        <p role="alert" className="text-sm text-[var(--danger-fg)]">
          Gagal memuat data harga paket
        </p>
      )}
      {!query.isLoading && !query.isError && (
        <DataTable
          data={sorted}
          columns={columns}
          emptyMessage={
            noMatchesFromNonEmptyList
              ? "Tidak ada tingkat yang cocok dengan filter."
              : "Vendor ini belum punya harga paket"
          }
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
        message={`Akhiri harga paket "${pendingDisable?.package}" per hari ini? Periode ini menjadi riwayat, tidak bisa diaktifkan kembali -- buat baris baru untuk periode berikutnya.`}
        confirmLabel="Akhiri"
        isPending={disableMutation.isPending}
        onConfirm={confirmDisable}
        onClose={() => setPendingDisable(null)}
      />
    </div>
  );
}
