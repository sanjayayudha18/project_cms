import { Button } from "@/components/ui/Button";
import { ConfirmActionDialog } from "@/components/ui/ConfirmActionDialog";
import { EmptyState } from "@/components/ui/EmptyState";
import { PageHeader } from "@/components/ui/PageHeader";
import { useToast } from "@/lib/hooks/useToast";
import { AlertCircle } from "lucide-react";
import { useState } from "react";
import { pendingApprovalMessage } from "../../master-data/changeRequest";
import { useATMsList, useDisableATM, useEnableATM } from "../hooks";
import type { AdminATM } from "../types";
import { toListParams, useAdminATMsUrlState } from "../useAdminATMsUrlState";
import { ATMAssignmentsDialog } from "./ATMAssignmentsDialog";
import { ATMFilterBar } from "./ATMFilterBar";
import { ATMFormDialog } from "./ATMFormDialog";
import { ATMsTable } from "./ATMsTable";

const SKELETON_ROW_KEYS = ["row-1", "row-2", "row-3", "row-4", "row-5"];

/** Admin ATMs screen (Req 3-4, 8): filterable/paginated table + create/edit/disable/enable. */
export function AdminATMsPage() {
  const { params, searchInput, setSearchInput, setParams } = useAdminATMsUrlState();
  const { toast } = useToast();

  const atmsQuery = useATMsList(toListParams(params));
  const disableMutation = useDisableATM();
  const enableMutation = useEnableATM();

  const [formATM, setFormATM] = useState<AdminATM | null>(null);
  const [isFormOpen, setFormOpen] = useState(false);
  const [assignmentsATM, setAssignmentsATM] = useState<AdminATM | null>(null);
  const [pendingAction, setPendingAction] = useState<{
    type: "disable" | "enable";
    atm: AdminATM;
  } | null>(null);

  const atms = atmsQuery.data?.atms ?? [];
  const total = atmsQuery.data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / params.page_size));

  function openCreate(): void {
    setFormATM(null);
    setFormOpen(true);
  }

  function openEdit(atm: AdminATM): void {
    setFormATM(atm);
    setFormOpen(true);
  }

  function confirmPendingAction(): void {
    if (!pendingAction) return;
    const mutation = pendingAction.type === "disable" ? disableMutation : enableMutation;
    mutation.mutate(pendingAction.atm.id, {
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

  return (
    <div className="flex flex-col gap-6 p-6">
      <PageHeader
        eyebrow="Pengaturan"
        title="Manajemen ATM"
        actions={<Button onClick={openCreate}>Tambah ATM</Button>}
      />

      <ATMFilterBar
        params={params}
        searchInput={searchInput}
        onSearchInputChange={setSearchInput}
        onParamsChange={setParams}
      />

      {atmsQuery.isLoading && (
        <div className="flex flex-col gap-2">
          {SKELETON_ROW_KEYS.map((key) => (
            <div key={key} className="h-10 w-full animate-pulse rounded bg-[var(--n-200)]" />
          ))}
        </div>
      )}

      {!atmsQuery.isLoading && atmsQuery.isError && (
        <div className="flex flex-col items-center gap-3 py-12">
          <AlertCircle className="h-10 w-10 text-[var(--danger-fg)]" aria-hidden="true" />
          <p className="text-sm text-[var(--n-700)]">Gagal memuat daftar ATM</p>
          <Button variant="secondary" onClick={() => atmsQuery.refetch()}>
            Coba Lagi
          </Button>
        </div>
      )}

      {!atmsQuery.isLoading && !atmsQuery.isError && atms.length === 0 && (
        <EmptyState message="Tidak ada ATM yang sesuai dengan filter saat ini" />
      )}

      {!atmsQuery.isLoading && !atmsQuery.isError && atms.length > 0 && (
        <ATMsTable
          atms={atms}
          onEdit={openEdit}
          onDisable={(atm) => setPendingAction({ type: "disable", atm })}
          onEnable={(atm) => setPendingAction({ type: "enable", atm })}
          onAssignments={setAssignmentsATM}
        />
      )}

      <div className="flex flex-wrap items-center justify-between gap-4 text-sm text-[var(--n-600)]">
        <span>
          Halaman {params.page} dari {totalPages} ({total} ATM)
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

      <ATMAssignmentsDialog atm={assignmentsATM} onClose={() => setAssignmentsATM(null)} />
      <ATMFormDialog open={isFormOpen} onClose={() => setFormOpen(false)} atm={formATM} />

      <ConfirmActionDialog
        open={pendingAction !== null}
        title={pendingAction?.type === "disable" ? "Nonaktifkan ATM" : "Aktifkan ATM"}
        message={
          pendingAction?.type === "disable"
            ? `Nonaktifkan ATM "${pendingAction.atm.terminal_id}"?`
            : `Aktifkan kembali ATM "${pendingAction?.atm.terminal_id}"?`
        }
        confirmLabel={pendingAction?.type === "disable" ? "Nonaktifkan" : "Aktifkan"}
        isPending={disableMutation.isPending || enableMutation.isPending}
        onConfirm={confirmPendingAction}
        onClose={() => setPendingAction(null)}
      />
    </div>
  );
}
