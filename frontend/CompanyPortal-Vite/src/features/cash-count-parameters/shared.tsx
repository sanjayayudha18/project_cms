import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import type { ApiError } from "@/lib/api/client";
import { useToast } from "@/lib/hooks/useToast";
import type { UseMutationResult } from "@tanstack/react-query";
import { CheckCircle, XCircle } from "lucide-react";
import { type ReactNode, useState } from "react";
import { PendingApprovalBadge } from "../master-data/PendingApprovalBadge";
import { type ChangeRequestAccepted, pendingApprovalMessage } from "../master-data/changeRequest";
import type { Mutation, ToggleOp } from "./api";

export const inputClass =
  "min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)] disabled:bg-[var(--n-100)] disabled:text-[var(--n-500)]";

export function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    // biome-ignore lint/a11y/noLabelWithoutControl: the control is always passed in as children
    <label className="flex flex-col gap-1 text-sm font-medium text-[var(--n-700)]">
      {label}
      {children}
    </label>
  );
}

export function ActiveBadge({ isActive }: { isActive: boolean }) {
  return isActive ? (
    <Badge variant="success" icon={CheckCircle} label="Aktif" />
  ) : (
    <Badge variant="danger" icon={XCircle} label="Nonaktif" />
  );
}

/** Active/inactive badge + "Menunggu approval" for rows submitted in this session. */
export function StatusCell({ isActive, pending }: { isActive: boolean; pending: boolean }) {
  return (
    <span className="inline-flex flex-wrap items-center gap-1">
      <ActiveBadge isActive={isActive} />
      {pending && <PendingApprovalBadge />}
    </span>
  );
}

export function RowActions({
  isActive,
  busy,
  onEdit,
  onToggle,
}: {
  isActive: boolean;
  busy: boolean;
  onEdit: () => void;
  onToggle: (op: ToggleOp) => void;
}) {
  return (
    <div className="flex justify-end gap-2">
      {isActive && (
        <Button variant="secondary" disabled={busy} onClick={onEdit}>
          Ubah
        </Button>
      )}
      <Button
        variant={isActive ? "danger" : "secondary"}
        disabled={busy}
        onClick={() => onToggle(isActive ? "disable" : "enable")}
      >
        {isActive ? "Nonaktifkan" : "Aktifkan"}
      </Button>
    </div>
  );
}

/**
 * Wraps a maker-checker mutation: a 202 toasts "diajukan, menunggu persetujuan"
 * and remembers the target row so it shows the pending badge until reload
 * (creates have no row yet, so they only toast).
 */
export function useSubmit<C, U>(
  mutation: UseMutationResult<ChangeRequestAccepted, ApiError, Mutation<C, U>>,
) {
  const { toast } = useToast();
  const [pendingIds, setPendingIds] = useState<ReadonlySet<number>>(new Set());

  function run(m: Mutation<C, U>, onDone?: () => void): void {
    mutation.mutate(m, {
      onSuccess: (res) => {
        toast({ type: "success", message: pendingApprovalMessage(res) });
        if (m.op !== "create") setPendingIds((prev) => new Set(prev).add(m.id));
        onDone?.();
      },
      // 422 carries the Indonesian validation message from the API.
      onError: (err) => toast({ type: "error", message: err.message }),
    });
  }
  return { run, pendingIds, isPending: mutation.isPending };
}

export function FormActions({
  onClose,
  canSubmit,
}: {
  onClose: () => void;
  canSubmit: boolean;
}) {
  return (
    <div className="flex justify-end gap-2">
      <Button type="button" variant="secondary" onClick={onClose}>
        Batal
      </Button>
      <Button type="submit" disabled={!canSubmit}>
        Ajukan
      </Button>
    </div>
  );
}

export function LoadState({
  isLoading,
  isError,
  label,
}: {
  isLoading: boolean;
  isError: boolean;
  label: string;
}) {
  return (
    <>
      {isLoading && <p className="text-sm text-[var(--n-500)]">Memuat…</p>}
      {isError && (
        <p role="alert" className="text-sm text-[var(--danger-fg)]">
          Gagal memuat {label}
        </p>
      )}
    </>
  );
}
