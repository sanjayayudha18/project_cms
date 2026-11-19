import { Button } from "@/components/ui/Button";
import { EmptyState } from "@/components/ui/EmptyState";
import { useAuthStore } from "@/lib/auth/store";
import { Link } from "@tanstack/react-router";
import { AlertCircle } from "lucide-react";
import { PksTable } from "../../admin-pks/components/PksTable";
import { usePksList } from "../../admin-pks/hooks";
import type { AdminVendor } from "../types";

const TAB_PAGE_SIZE = 100;

/** Read-only PKS history of one vendor (all statuses); writes live in Daftar PKS (Req 8.2, 8.3). */
export function VendorPksTab({ vendor }: { vendor: AdminVendor }) {
  const role = useAuthStore((s) => s.user?.role);
  const canManage = role === "ADMIN" || role === "ADMIN_PARAM";
  const query = usePksList({ vendor: vendor.code, status: "semua", page_size: TAB_PAGE_SIZE });
  const items = query.data?.items ?? [];

  return (
    <div className="flex flex-col gap-4">
      {canManage && (
        <div>
          <Link to="/settings/admin/pks" search={{ vendor: vendor.code } as never}>
            <Button variant="secondary">Kelola di Daftar PKS</Button>
          </Link>
        </div>
      )}
      {query.isLoading && <div className="h-10 w-full animate-pulse rounded bg-[var(--n-200)]" />}
      {query.isError && (
        <div className="flex flex-col items-center gap-3 py-8">
          <AlertCircle className="h-10 w-10 text-[var(--danger-fg)]" aria-hidden="true" />
          <p className="text-sm text-[var(--n-700)]">Gagal memuat PKS vendor</p>
          <Button variant="secondary" onClick={() => query.refetch()}>
            Coba Lagi
          </Button>
        </div>
      )}
      {!query.isLoading && !query.isError && items.length === 0 && (
        <EmptyState message="Vendor ini belum memiliki PKS" />
      )}
      {!query.isLoading && !query.isError && items.length > 0 && (
        <PksTable items={items} vendorCode={vendor.code} />
      )}
    </div>
  );
}
