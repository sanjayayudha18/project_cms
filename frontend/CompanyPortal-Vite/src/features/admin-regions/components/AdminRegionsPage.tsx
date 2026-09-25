import { Button } from "@/components/ui/Button";
import { ConfirmActionDialog } from "@/components/ui/ConfirmActionDialog";
import { EmptyState } from "@/components/ui/EmptyState";
import { PageHeader } from "@/components/ui/PageHeader";
import { useToast } from "@/lib/hooks/useToast";
import { AlertCircle } from "lucide-react";
import { useState } from "react";
import { useDisableRegion, useEnableRegion, useRegionsList } from "../hooks";
import type { AdminRegion } from "../types";
import { toListParams, useAdminRegionsUrlState } from "../useAdminRegionsUrlState";
import { RegionFilterBar } from "./RegionFilterBar";
import { RegionFormDialog } from "./RegionFormDialog";
import { RegionsTable } from "./RegionsTable";

const SKELETON_ROW_KEYS = ["row-1", "row-2", "row-3", "row-4", "row-5"];

/** Admin Regions screen (.kiro/specs/region-management, Req 1-4): filterable/paginated table + create/edit/disable/enable. */
export function AdminRegionsPage() {
  const { params, searchInput, setSearchInput, setParams } = useAdminRegionsUrlState();
  const { toast } = useToast();

  const regionsQuery = useRegionsList(toListParams(params));
  const disableMutation = useDisableRegion();
  const enableMutation = useEnableRegion();

  const [formRegion, setFormRegion] = useState<AdminRegion | null>(null);
  const [isFormOpen, setFormOpen] = useState(false);
  const [pendingAction, setPendingAction] = useState<{
    type: "disable" | "enable";
    region: AdminRegion;
  } | null>(null);

  const regions = regionsQuery.data?.regions ?? [];
  const total = regionsQuery.data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / params.page_size));

  function openCreate(): void {
    setFormRegion(null);
    setFormOpen(true);
  }

  function openEdit(region: AdminRegion): void {
    setFormRegion(region);
    setFormOpen(true);
  }

  function confirmPendingAction(): void {
    if (!pendingAction) return;
    const mutation = pendingAction.type === "disable" ? disableMutation : enableMutation;
    mutation.mutate(pendingAction.region.id, {
      onSuccess: () => {
        toast({
          type: "success",
          message: `Region "${pendingAction.region.code}" berhasil diubah`,
        });
        setPendingAction(null);
      },
      onError: (err) => {
        toast({ type: "error", message: err.message });
        setPendingAction(null);
      },
    });
  }

  return (
    <div className="flex flex-col gap-6 p-6">
      <PageHeader
        eyebrow="Pengaturan"
        title="Manajemen Region"
        actions={<Button onClick={openCreate}>Tambah Region</Button>}
      />

      <RegionFilterBar
        params={params}
        searchInput={searchInput}
        onSearchInputChange={setSearchInput}
        onParamsChange={setParams}
      />

      {regionsQuery.isLoading && (
        <div className="flex flex-col gap-2">
          {SKELETON_ROW_KEYS.map((key) => (
            <div key={key} className="h-10 w-full animate-pulse rounded bg-[var(--n-200)]" />
          ))}
        </div>
      )}

      {!regionsQuery.isLoading && regionsQuery.isError && (
        <div className="flex flex-col items-center gap-3 py-12">
          <AlertCircle className="h-10 w-10 text-[var(--danger-fg)]" aria-hidden="true" />
          <p className="text-sm text-[var(--n-700)]">Gagal memuat daftar region</p>
          <Button variant="secondary" onClick={() => regionsQuery.refetch()}>
            Coba Lagi
          </Button>
        </div>
      )}

      {!regionsQuery.isLoading && !regionsQuery.isError && regions.length === 0 && (
        <EmptyState message="Tidak ada region yang sesuai dengan filter saat ini" />
      )}

      {!regionsQuery.isLoading && !regionsQuery.isError && regions.length > 0 && (
        <RegionsTable
          regions={regions}
          onEdit={openEdit}
          onDisable={(region) => setPendingAction({ type: "disable", region })}
          onEnable={(region) => setPendingAction({ type: "enable", region })}
        />
      )}

      <div className="flex flex-wrap items-center justify-between gap-4 text-sm text-[var(--n-600)]">
        <span>
          Halaman {params.page} dari {totalPages} ({total} region)
        </span>
        <div className="flex items-center gap-2">
          <Button
            variant="secondary"
            disabled={params.page <= 1}
            onClick={() => setParams({ page: params.page - 1 })}
          >
            Sebelumnya
          </Button>
          <Button
            variant="secondary"
            disabled={params.page >= totalPages}
            onClick={() => setParams({ page: params.page + 1 })}
          >
            Berikutnya
          </Button>
        </div>
      </div>

      <RegionFormDialog open={isFormOpen} onClose={() => setFormOpen(false)} region={formRegion} />

      <ConfirmActionDialog
        open={pendingAction !== null}
        title={pendingAction?.type === "disable" ? "Nonaktifkan Region" : "Aktifkan Region"}
        message={
          pendingAction?.type === "disable"
            ? `Nonaktifkan region "${pendingAction.region.code}"? Region dengan lokasi aktif tidak dapat dinonaktifkan.`
            : `Aktifkan kembali region "${pendingAction?.region.code}"?`
        }
        confirmLabel={pendingAction?.type === "disable" ? "Nonaktifkan" : "Aktifkan"}
        isPending={disableMutation.isPending || enableMutation.isPending}
        onConfirm={confirmPendingAction}
        onClose={() => setPendingAction(null)}
      />
    </div>
  );
}
