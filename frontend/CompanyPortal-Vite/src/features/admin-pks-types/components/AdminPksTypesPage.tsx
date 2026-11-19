import { BackLink } from "@/components/ui/BackLink";
import { Button } from "@/components/ui/Button";
import { ConfirmActionDialog } from "@/components/ui/ConfirmActionDialog";
import { PageHeader } from "@/components/ui/PageHeader";
import { pendingApprovalMessage } from "@/features/master-data/changeRequest";
import { usePendingEntityIds } from "@/features/master-data/pending";
import { useToast } from "@/lib/hooks/useToast";
import { AlertCircle, Search } from "lucide-react";
import { useMemo, useState } from "react";
import {
  useDisablePksType,
  useEnablePksType,
  usePksTypesList,
  useSubmitPksType,
  useUpdatePksType,
} from "../hooks";
import type { PksType } from "../types";
import { PksTypeFormDialog } from "./PksTypeFormDialog";
import { PksTypesTable } from "./PksTypesTable";

const SKELETON_ROW_KEYS = ["skeleton-1", "skeleton-2", "skeleton-3", "skeleton-4"];

export function AdminPksTypesPage() {
  const { toast } = useToast();
  const typesQuery = usePksTypesList();
  const pendingIds = usePendingEntityIds("vendor_pks_type");

  const submitMutation = useSubmitPksType();
  const updateMutation = useUpdatePksType();
  const disableMutation = useDisablePksType();
  const enableMutation = useEnablePksType();

  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState<"all" | "active" | "disabled">("all");

  const [isFormOpen, setIsFormOpen] = useState(false);
  const [editingType, setEditingType] = useState<PksType | null>(null);

  const [confirmAction, setConfirmAction] = useState<{
    op: "disable" | "enable";
    type: PksType;
  } | null>(null);

  const rawList = typesQuery.data ?? [];

  const filteredList = useMemo(() => {
    let result = rawList;

    if (statusFilter === "active") {
      result = result.filter((item) => item.is_active);
    } else if (statusFilter === "disabled") {
      result = result.filter((item) => !item.is_active);
    }

    const q = search.trim().toLowerCase();
    if (q) {
      result = result.filter(
        (item) => item.code.toLowerCase().includes(q) || item.name.toLowerCase().includes(q),
      );
    }

    return result;
  }, [rawList, statusFilter, search]);

  function handleOpenCreate() {
    setEditingType(null);
    setIsFormOpen(true);
  }

  function handleOpenEdit(item: PksType) {
    setEditingType(item);
    setIsFormOpen(true);
  }

  async function handleFormSubmit(values: { code: string; name: string }) {
    if (editingType) {
      const res = await updateMutation.mutateAsync({
        id: editingType.id,
        payload: { name: values.name },
      });
      toast({ type: "success", message: pendingApprovalMessage(res) });
    } else {
      const res = await submitMutation.mutateAsync({
        code: values.code,
        name: values.name,
      });
      toast({ type: "success", message: pendingApprovalMessage(res) });
    }
  }

  function handleOpenConfirm(op: "disable" | "enable", type: PksType) {
    setConfirmAction({ op, type });
  }

  async function handleConfirmAction() {
    if (!confirmAction) return;
    const { op, type } = confirmAction;

    try {
      if (op === "disable") {
        const res = await disableMutation.mutateAsync(type.id);
        toast({ type: "success", message: pendingApprovalMessage(res) });
      } else {
        const res = await enableMutation.mutateAsync(type.id);
        toast({ type: "success", message: pendingApprovalMessage(res) });
      }
      setConfirmAction(null);
    } catch (err: unknown) {
      const msg =
        err && typeof err === "object" && "message" in err && typeof err.message === "string"
          ? err.message
          : `Gagal ${op === "disable" ? "menonaktifkan" : "mengaktifkan"} type PKS`;
      toast({ type: "error", message: msg });
      setConfirmAction(null);
    }
  }

  const isMutating =
    submitMutation.isPending ||
    updateMutation.isPending ||
    disableMutation.isPending ||
    enableMutation.isPending;

  return (
    <div className="flex flex-col gap-6 p-6">
      <div className="self-start">
        <BackLink to="/settings" label="Kembali ke Pengaturan" />
      </div>

      <PageHeader
        eyebrow="Pengaturan"
        title="Type PKS"
        description="Kelola master type perjanjian kerja sama (PKS) vendor."
        actions={<Button onClick={handleOpenCreate}>Tambah Type PKS</Button>}
      />

      <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-4">
        <div className="relative flex-1 max-w-md">
          <label htmlFor="pks-types-search" className="sr-only">
            Cari berdasarkan kode atau nama type PKS
          </label>
          <Search
            className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-[var(--n-400)] pointer-events-none"
            aria-hidden="true"
          />
          <input
            id="pks-types-search"
            type="search"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Cari kode atau nama..."
            className="w-full rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] pl-9 pr-3 py-2 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
          />
        </div>

        <div className="flex items-center gap-2">
          <label htmlFor="status-filter" className="text-sm font-medium text-[var(--n-600)]">
            Status:
          </label>
          <select
            id="status-filter"
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value as "all" | "active" | "disabled")}
            className="rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 py-2 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
          >
            <option value="all">Semua</option>
            <option value="active">Aktif</option>
            <option value="disabled">Nonaktif</option>
          </select>
        </div>
      </div>

      {typesQuery.isLoading && (
        <div className="flex flex-col gap-2" aria-busy="true" aria-label="Memuat data type PKS">
          {SKELETON_ROW_KEYS.map((key) => (
            <div key={key} className="h-12 w-full animate-pulse rounded bg-[var(--n-200)]" />
          ))}
        </div>
      )}

      {!typesQuery.isLoading && typesQuery.isError && (
        <div className="flex flex-col items-center justify-center gap-3 rounded-[var(--radius-lg)] border border-[var(--n-200)] bg-[var(--n-0)] p-12 text-center">
          <AlertCircle className="size-10 text-[var(--danger-fg)]" aria-hidden="true" />
          <h3 className="text-base font-semibold text-[var(--n-900)]">
            Gagal memuat daftar type PKS
          </h3>
          <p className="max-w-md text-sm text-[var(--n-600)]">
            {typesQuery.error?.message || "Terjadi kendala saat menghubungi server."}
          </p>
          <Button variant="secondary" onClick={() => typesQuery.refetch()}>
            Coba Lagi
          </Button>
        </div>
      )}

      {!typesQuery.isLoading && !typesQuery.isError && (
        <PksTypesTable
          types={filteredList}
          pendingIds={pendingIds}
          busy={isMutating}
          onEdit={handleOpenEdit}
          onDisable={(type) => handleOpenConfirm("disable", type)}
          onEnable={(type) => handleOpenConfirm("enable", type)}
          emptyMessage={
            rawList.length === 0
              ? "Belum ada data type PKS"
              : "Tidak ada data type PKS yang sesuai dengan filter atau pencarian saat ini"
          }
        />
      )}

      {isFormOpen && (
        <PksTypeFormDialog
          open={isFormOpen}
          editing={editingType}
          isPending={isMutating}
          onClose={() => setIsFormOpen(false)}
          onSubmit={handleFormSubmit}
        />
      )}

      {confirmAction && (
        <ConfirmActionDialog
          open={confirmAction !== null}
          title={confirmAction.op === "disable" ? "Nonaktifkan Type PKS" : "Aktifkan Type PKS"}
          message={
            confirmAction.op === "disable"
              ? `Apakah Anda yakin ingin menonaktifkan type PKS "${confirmAction.type.code}"? Perubahan ini akan diajukan ke checker.`
              : `Apakah Anda yakin ingin mengaktifkan type PKS "${confirmAction.type.code}"? Perubahan ini akan diajukan ke checker.`
          }
          confirmLabel={confirmAction.op === "disable" ? "Nonaktifkan" : "Aktifkan"}
          isPending={isMutating}
          onConfirm={handleConfirmAction}
          onClose={() => setConfirmAction(null)}
        />
      )}
    </div>
  );
}
