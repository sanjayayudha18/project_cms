/**
 * RBAC settings: user leaves (Requirement 6). Create form (RHF+Zod) plus a
 * read-only list -- no revoke/edit endpoint exists for leaves, unlike
 * delegations (design.md: leaves are create-only).
 */

import { PageHeader } from "@/components/ui/PageHeader";
import { zodResolver } from "@hookform/resolvers/zod";
import { AlertTriangle } from "lucide-react";
import { useForm } from "react-hook-form";
import { useCreateLeave, useLeaves } from "../hooks/useRbacQueries";
import { type LeaveFormValues, type RbacLeave, leaveSchema } from "../types";
import { FormField, rbacInputClass } from "./FormField";
import { StatusMessage } from "./StatusMessage";
import { rbacRowClass, rbacTdClass, rbacThClass, rbacTheadRowClass } from "./tableStyles";

/** datetime-local's "YYYY-MM-DDTHH:mm" (no seconds/timezone) -> RFC3339. */
function toRfc3339(localDateTime: string): string {
  return new Date(localDateTime).toISOString();
}

export function RbacLeavesPage() {
  const { data, isLoading, isError } = useLeaves();
  const leaves = data?.leaves ?? [];

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Pengaturan"
        title="Cuti Pengguna"
        description="Catat periode cuti pengguna untuk mengaktifkan alur persetujuan cadangan."
      />

      <CreateLeaveForm />

      <StatusMessage
        isLoading={isLoading}
        isError={isError}
        isEmpty={!isLoading && !isError && leaves.length === 0}
        loadingLabel="Memuat cuti pengguna…"
        errorMessage="Gagal memuat cuti pengguna. Coba lagi."
        emptyMessage="Belum ada cuti tercatat."
      />

      {!isLoading && !isError && leaves.length > 0 && (
        <div className="overflow-x-auto rounded-[var(--radius-lg)] border border-[var(--n-200)]">
          <table className="w-full min-w-[560px] border-collapse text-sm">
            <thead>
              <tr className={rbacTheadRowClass}>
                <th scope="col" className={rbacThClass()}>
                  User ID
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
              </tr>
            </thead>
            <tbody>
              {leaves.map((leave) => (
                <LeaveRow key={leave.id} leave={leave} />
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function CreateLeaveForm() {
  const mutation = useCreateLeave();
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<LeaveFormValues>({
    resolver: zodResolver(leaveSchema),
  });

  const onSubmit = handleSubmit((values) => {
    mutation.mutate(
      { ...values, start_at: toRfc3339(values.start_at), end_at: toRfc3339(values.end_at) },
      { onSuccess: () => reset() },
    );
  });

  return (
    <form
      onSubmit={onSubmit}
      className="flex flex-col gap-4 rounded-[var(--radius-lg)] border border-[var(--n-200)] bg-[var(--n-0)] p-[var(--space-5)]"
    >
      <h2 className="text-sm font-semibold text-[var(--n-900)]">Catat Cuti Baru</h2>
      <div className="flex flex-wrap gap-3">
        <div className="w-32">
          <FormField label="User ID" htmlFor="leave-user" error={errors.user_id?.message}>
            <input
              id="leave-user"
              type="number"
              className={rbacInputClass}
              {...register("user_id")}
            />
          </FormField>
        </div>
        <div className="w-52">
          <FormField label="Mulai" htmlFor="leave-start" error={errors.start_at?.message}>
            <input
              id="leave-start"
              type="datetime-local"
              className={rbacInputClass}
              {...register("start_at")}
            />
          </FormField>
        </div>
        <div className="w-52">
          <FormField label="Berakhir" htmlFor="leave-end" error={errors.end_at?.message}>
            <input
              id="leave-end"
              type="datetime-local"
              className={rbacInputClass}
              {...register("end_at")}
            />
          </FormField>
        </div>
        <div className="min-w-[200px] flex-1">
          <FormField label="Alasan (opsional)" htmlFor="leave-reason">
            <input
              id="leave-reason"
              type="text"
              className={rbacInputClass}
              {...register("reason")}
            />
          </FormField>
        </div>
      </div>

      {mutation.isError && (
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
        Catat Cuti
      </button>
    </form>
  );
}

function LeaveRow({ leave }: { leave: RbacLeave }) {
  return (
    <tr className={rbacRowClass}>
      <td className={`${rbacTdClass} tabular-nums`}>{leave.user_id}</td>
      <td className={rbacTdClass}>{new Date(leave.start_at).toLocaleString("id-ID")}</td>
      <td className={rbacTdClass}>{new Date(leave.end_at).toLocaleString("id-ID")}</td>
      <td className={`${rbacTdClass} text-[var(--n-600)]`}>{leave.reason ?? "—"}</td>
    </tr>
  );
}
