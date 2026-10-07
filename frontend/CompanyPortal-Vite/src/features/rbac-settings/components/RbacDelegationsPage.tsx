/**
 * RBAC settings: approval delegations (Requirement 5). Create form (RHF+Zod)
 * plus a revoke action per row. A 409 from the backend (overlapping range for
 * the same from_user_id, approval_delegations_no_overlap constraint) renders
 * as an inline conflict message, icon + text, not a generic error.
 */

import { PageHeader } from "@/components/ui/PageHeader";
import { zodResolver } from "@hookform/resolvers/zod";
import { AlertTriangle, Trash2 } from "lucide-react";
import { useForm } from "react-hook-form";
import { useCreateDelegation, useDelegations, useRevokeDelegation } from "../hooks/useRbacQueries";
import { type DelegationFormValues, type RbacDelegation, delegationSchema } from "../types";
import { FormField, rbacInputClass } from "./FormField";
import { StatusMessage } from "./StatusMessage";
import { rbacRowClass, rbacTdClass, rbacThClass, rbacTheadRowClass } from "./tableStyles";

/** datetime-local's "YYYY-MM-DDTHH:mm" (no seconds/timezone) -> RFC3339. */
function toRfc3339(localDateTime: string): string {
  return new Date(localDateTime).toISOString();
}

export function RbacDelegationsPage() {
  const { data, isLoading, isError } = useDelegations();
  const delegations = data?.delegations ?? [];

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Pengaturan"
        title="Delegasi Persetujuan"
        description="Alihkan wewenang persetujuan sementara saat approver tidak tersedia."
      />

      <CreateDelegationForm />

      <StatusMessage
        isLoading={isLoading}
        isError={isError}
        isEmpty={!isLoading && !isError && delegations.length === 0}
        loadingLabel="Memuat delegasi…"
        errorMessage="Gagal memuat delegasi. Coba lagi."
        emptyMessage="Belum ada delegasi."
      />

      {!isLoading && !isError && delegations.length > 0 && (
        <div className="overflow-x-auto rounded-[var(--radius-lg)] border border-[var(--n-200)]">
          <table className="w-full min-w-[720px] border-collapse text-sm">
            <thead>
              <tr className={rbacTheadRowClass}>
                <th scope="col" className={rbacThClass()}>
                  Dari
                </th>
                <th scope="col" className={rbacThClass()}>
                  Ke
                </th>
                <th scope="col" className={rbacThClass()}>
                  Mulai
                </th>
                <th scope="col" className={rbacThClass()}>
                  Berakhir
                </th>
                <th scope="col" className={rbacThClass()}>
                  Alasan
                </th>
                <th scope="col" className={rbacThClass()}>
                  Aksi
                </th>
              </tr>
            </thead>
            <tbody>
              {delegations.map((delegation) => (
                <DelegationRow key={delegation.id} delegation={delegation} />
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function CreateDelegationForm() {
  const mutation = useCreateDelegation();
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<DelegationFormValues>({
    resolver: zodResolver(delegationSchema),
  });

  const onSubmit = handleSubmit((values) => {
    mutation.mutate(
      { ...values, start_at: toRfc3339(values.start_at), end_at: toRfc3339(values.end_at) },
      { onSuccess: () => reset() },
    );
  });

  const isConflict = mutation.isError && mutation.error.status === 409;

  return (
    <form
      onSubmit={onSubmit}
      className="flex flex-col gap-4 rounded-[var(--radius-lg)] border border-[var(--n-200)] bg-[var(--n-0)] p-[var(--space-5)]"
    >
      <h2 className="text-sm font-semibold text-[var(--n-900)]">Buat Delegasi Baru</h2>
      <div className="flex flex-wrap gap-3">
        <div className="w-32">
          <FormField
            label="Dari (User ID)"
            htmlFor="delegation-from"
            error={errors.from_user_id?.message}
          >
            <input
              id="delegation-from"
              type="number"
              className={rbacInputClass}
              {...register("from_user_id")}
            />
          </FormField>
        </div>
        <div className="w-32">
          <FormField
            label="Ke (User ID)"
            htmlFor="delegation-to"
            error={errors.to_user_id?.message}
          >
            <input
              id="delegation-to"
              type="number"
              className={rbacInputClass}
              {...register("to_user_id")}
            />
          </FormField>
        </div>
        <div className="w-52">
          <FormField label="Mulai" htmlFor="delegation-start" error={errors.start_at?.message}>
            <input
              id="delegation-start"
              type="datetime-local"
              className={rbacInputClass}
              {...register("start_at")}
            />
          </FormField>
        </div>
        <div className="w-52">
          <FormField label="Berakhir" htmlFor="delegation-end" error={errors.end_at?.message}>
            <input
              id="delegation-end"
              type="datetime-local"
              className={rbacInputClass}
              {...register("end_at")}
            />
          </FormField>
        </div>
        <div className="min-w-[200px] flex-1">
          <FormField label="Alasan (opsional)" htmlFor="delegation-reason">
            <input
              id="delegation-reason"
              type="text"
              className={rbacInputClass}
              {...register("reason")}
            />
          </FormField>
        </div>
      </div>

      {isConflict && (
        <p className="flex items-center gap-1.5 rounded-[var(--radius-md)] bg-[var(--danger-bg)] px-3 py-2 text-sm text-[var(--danger-fg)]">
          <AlertTriangle className="h-4 w-4 shrink-0" aria-hidden="true" />
          Rentang delegasi tumpang tindih dengan delegasi lain untuk user ini.
        </p>
      )}
      {mutation.isError && !isConflict && (
        <p className="flex items-center gap-1.5 text-sm text-[var(--danger-fg)]">
          <AlertTriangle className="h-4 w-4 shrink-0" aria-hidden="true" />
          {mutation.error.message}
        </p>
      )}

      <button
        type="submit"
        disabled={mutation.isPending}
        className="inline-flex w-fit items-center rounded-[var(--radius-md)] bg-[var(--red-500)] px-4 py-2 text-sm font-medium text-[var(--n-0)] hover:bg-[var(--red-600)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)] outline-none disabled:cursor-not-allowed disabled:bg-[var(--n-200)] disabled:text-[var(--n-400)]"
      >
        Buat Delegasi
      </button>
    </form>
  );
}

function DelegationRow({ delegation }: { delegation: RbacDelegation }) {
  const mutation = useRevokeDelegation();

  return (
    <tr className={rbacRowClass}>
      <td className={`${rbacTdClass} tabular-nums`}>{delegation.from_user_id}</td>
      <td className={`${rbacTdClass} tabular-nums`}>{delegation.to_user_id}</td>
      <td className={rbacTdClass}>{new Date(delegation.start_at).toLocaleString("id-ID")}</td>
      <td className={rbacTdClass}>{new Date(delegation.end_at).toLocaleString("id-ID")}</td>
      <td className={`${rbacTdClass} text-[var(--n-600)]`}>{delegation.reason ?? "—"}</td>
      <td className={rbacTdClass}>
        <button
          type="button"
          onClick={() => mutation.mutate(delegation.id)}
          disabled={mutation.isPending}
          className="inline-flex items-center gap-1 rounded-[var(--radius-sm)] px-2 py-1 text-xs font-medium text-[var(--danger-fg)] hover:bg-[var(--danger-bg)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)] outline-none disabled:cursor-not-allowed disabled:opacity-50"
        >
          <Trash2 className="h-3.5 w-3.5" aria-hidden="true" />
          Cabut
        </button>
      </td>
    </tr>
  );
}
