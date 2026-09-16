/**
 * RBAC settings: approval policies (Requirement 7). Table with right-aligned
 * tabular-nums amounts (money is numeric, never float -- amounts stay exact
 * decimal strings end to end, never coerced to `number` for display), plus a
 * create form and a per-row inline edit form (RHF+Zod, same
 * min_amount < max_amount strict-inequality guard as the backend).
 */

import { PageHeader } from "@/components/ui/PageHeader";
import { zodResolver } from "@hookform/resolvers/zod";
import { AlertTriangle, Pencil, X } from "lucide-react";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { useCreatePolicy, usePolicies, useUpdatePolicy } from "../hooks/useRbacQueries";
import { type PolicyFormValues, type RbacPolicy, policySchema } from "../types";
import { FormField, rbacInputClass } from "./FormField";
import { StatusMessage } from "./StatusMessage";
import { rbacRowClass, rbacTdClass, rbacThClass, rbacTheadRowClass } from "./tableStyles";

export function RbacPoliciesPage() {
  const { data, isLoading, isError } = usePolicies();
  const [editingId, setEditingId] = useState<number | null>(null);
  const policies = data?.policies ?? [];

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Pengaturan"
        title="Kebijakan Persetujuan"
        description="Lihat dan atur batas nominal serta level persetujuan yang berlaku per jenis dokumen."
      />

      <CreatePolicyForm />

      <StatusMessage
        isLoading={isLoading}
        isError={isError}
        isEmpty={!isLoading && !isError && policies.length === 0}
        loadingLabel="Memuat kebijakan persetujuan…"
        errorMessage="Gagal memuat kebijakan persetujuan. Coba lagi."
        emptyMessage="Belum ada kebijakan persetujuan."
      />

      {!isLoading && !isError && policies.length > 0 && (
        <div className="overflow-x-auto rounded-[var(--radius-lg)] border border-[var(--n-200)]">
          <table className="w-full min-w-[720px] border-collapse text-sm">
            <thead>
              <tr className={rbacTheadRowClass}>
                <th scope="col" className={rbacThClass()}>
                  Jenis Dokumen
                </th>
                <th scope="col" className={rbacThClass("right")}>
                  Nominal Minimum
                </th>
                <th scope="col" className={rbacThClass("right")}>
                  Nominal Maksimum
                </th>
                <th scope="col" className={rbacThClass("right")}>
                  Level Persetujuan
                </th>
                <th scope="col" className={rbacThClass()}>
                  Aksi
                </th>
              </tr>
            </thead>
            <tbody>
              {policies.map((policy) =>
                editingId === policy.id ? (
                  <PolicyEditRow
                    key={policy.id}
                    policy={policy}
                    onDone={() => setEditingId(null)}
                  />
                ) : (
                  <PolicyRow
                    key={policy.id}
                    policy={policy}
                    onEdit={() => setEditingId(policy.id)}
                  />
                ),
              )}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function PolicyRow({ policy, onEdit }: { policy: RbacPolicy; onEdit: () => void }) {
  return (
    <tr className={rbacRowClass}>
      <td className={rbacTdClass}>{policy.document_type}</td>
      <td className={`${rbacTdClass} text-right tabular-nums`}>{policy.min_amount}</td>
      <td className={`${rbacTdClass} text-right tabular-nums`}>{policy.max_amount}</td>
      <td className={`${rbacTdClass} text-right tabular-nums`}>{policy.required_level}</td>
      <td className={rbacTdClass}>
        <button
          type="button"
          onClick={onEdit}
          className="inline-flex items-center gap-1 rounded-[var(--radius-sm)] px-2 py-1 text-xs font-medium text-[var(--red-600)] hover:bg-[var(--red-50)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)] outline-none"
        >
          <Pencil className="h-3.5 w-3.5" aria-hidden="true" />
          Ubah
        </button>
      </td>
    </tr>
  );
}

function PolicyFormFields({
  idPrefix,
  register,
  errors,
}: {
  idPrefix: string;
  register: ReturnType<typeof useForm<PolicyFormValues>>["register"];
  errors: ReturnType<typeof useForm<PolicyFormValues>>["formState"]["errors"];
}) {
  return (
    <div className="flex flex-wrap gap-3">
      <div className="min-w-[160px] flex-1">
        <FormField
          label="Jenis Dokumen"
          htmlFor={`${idPrefix}-document_type`}
          error={errors.document_type?.message}
        >
          <input
            id={`${idPrefix}-document_type`}
            type="text"
            className={rbacInputClass}
            {...register("document_type")}
          />
        </FormField>
      </div>
      <div className="w-44">
        <FormField
          label="Nominal Minimum"
          htmlFor={`${idPrefix}-min_amount`}
          error={errors.min_amount?.message}
        >
          <input
            id={`${idPrefix}-min_amount`}
            type="text"
            inputMode="decimal"
            placeholder="0"
            className={`${rbacInputClass} text-right tabular-nums`}
            {...register("min_amount")}
          />
        </FormField>
      </div>
      <div className="w-44">
        <FormField
          label="Nominal Maksimum"
          htmlFor={`${idPrefix}-max_amount`}
          error={errors.max_amount?.message}
        >
          <input
            id={`${idPrefix}-max_amount`}
            type="text"
            inputMode="decimal"
            placeholder="0"
            className={`${rbacInputClass} text-right tabular-nums`}
            {...register("max_amount")}
          />
        </FormField>
      </div>
      <div className="w-36">
        <FormField
          label="Level Persetujuan"
          htmlFor={`${idPrefix}-required_level`}
          error={errors.required_level?.message}
        >
          <input
            id={`${idPrefix}-required_level`}
            type="number"
            className={`${rbacInputClass} text-right tabular-nums`}
            {...register("required_level")}
          />
        </FormField>
      </div>
    </div>
  );
}

function CreatePolicyForm() {
  const mutation = useCreatePolicy();
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<PolicyFormValues>({
    resolver: zodResolver(policySchema),
  });

  const onSubmit = handleSubmit((values) => {
    mutation.mutate(values, { onSuccess: () => reset() });
  });

  return (
    <form
      onSubmit={onSubmit}
      className="flex flex-col gap-4 rounded-[var(--radius-lg)] border border-[var(--n-200)] bg-[var(--n-0)] p-[var(--space-5)]"
    >
      <h2 className="text-sm font-semibold text-[var(--n-900)]">Buat Kebijakan Baru</h2>
      <PolicyFormFields idPrefix="create-policy" register={register} errors={errors} />

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
        Buat Kebijakan
      </button>
    </form>
  );
}

function PolicyEditRow({ policy, onDone }: { policy: RbacPolicy; onDone: () => void }) {
  const mutation = useUpdatePolicy();
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<PolicyFormValues>({
    resolver: zodResolver(policySchema),
    defaultValues: {
      document_type: policy.document_type,
      min_amount: policy.min_amount,
      max_amount: policy.max_amount,
      required_level: policy.required_level,
    },
  });

  const onSubmit = handleSubmit((values) => {
    mutation.mutate({ id: policy.id, values }, { onSuccess: onDone });
  });

  return (
    <tr className="border-[var(--n-100)] border-b last:border-0 bg-[var(--n-50)]">
      <td colSpan={4} className="px-3 py-3 align-top">
        <form id={`policy-form-${policy.id}`} onSubmit={onSubmit} className="flex flex-col gap-3">
          <PolicyFormFields idPrefix={`policy-${policy.id}`} register={register} errors={errors} />
          {mutation.isError && (
            <p className="flex items-center gap-1.5 text-sm text-[var(--danger-fg)]">
              <AlertTriangle className="h-4 w-4 shrink-0" aria-hidden="true" />
              {mutation.error.message}
            </p>
          )}
        </form>
      </td>
      <td className="px-3 py-3 align-top">
        <div className="flex items-center gap-1">
          <button
            type="submit"
            form={`policy-form-${policy.id}`}
            disabled={mutation.isPending}
            className="inline-flex items-center rounded-[var(--radius-sm)] bg-[var(--red-500)] px-3 py-1.5 text-xs font-medium text-[var(--n-0)] hover:bg-[var(--red-600)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)] outline-none disabled:cursor-not-allowed disabled:bg-[var(--n-200)] disabled:text-[var(--n-400)]"
          >
            Simpan
          </button>
          <button
            type="button"
            onClick={onDone}
            className="inline-flex items-center gap-1 rounded-[var(--radius-sm)] px-2 py-1.5 text-xs font-medium text-[var(--n-600)] hover:bg-[var(--n-100)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)] outline-none"
          >
            <X className="h-3.5 w-3.5" aria-hidden="true" />
            Batal
          </button>
        </div>
      </td>
    </tr>
  );
}
