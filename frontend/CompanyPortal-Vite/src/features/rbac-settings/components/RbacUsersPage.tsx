/**
 * RBAC settings: user hierarchy + role table (Requirements 4, 9). Each row
 * can switch into an inline edit form for supervisor_id/approval_level
 * (RHF+Zod, self-supervisor blocked client-side to mirror the server guard),
 * with entered values retained on rejection and inline icon+text errors.
 */

import { PageHeader } from "@/components/ui/PageHeader";
import { zodResolver } from "@hookform/resolvers/zod";
import { Pencil, X } from "lucide-react";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { useSetHierarchy, useUserHierarchy } from "../hooks/useRbacQueries";
import { type HierarchyFormValues, type RbacUser, createHierarchySchema } from "../types";
import { FormField, rbacInputClass } from "./FormField";
import { RoleBadge } from "./RoleBadge";
import { StatusMessage } from "./StatusMessage";
import { rbacRowClass, rbacTdClass, rbacThClass, rbacTheadRowClass } from "./tableStyles";

export function RbacUsersPage() {
  const { data, isLoading, isError } = useUserHierarchy();
  const [editingId, setEditingId] = useState<number | null>(null);
  const users = data?.users ?? [];

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Pengaturan"
        title="Hierarki Pengguna"
        description="Atur atasan (supervisor) dan level persetujuan setiap pengguna."
      />

      <StatusMessage
        isLoading={isLoading}
        isError={isError}
        isEmpty={!isLoading && !isError && users.length === 0}
        loadingLabel="Memuat hierarki pengguna…"
        errorMessage="Gagal memuat hierarki pengguna. Coba lagi."
        emptyMessage="Belum ada pengguna."
      />

      {!isLoading && !isError && users.length > 0 && (
        <div className="overflow-x-auto rounded-[var(--radius-lg)] border border-[var(--n-200)]">
          <table className="w-full min-w-[720px] border-collapse text-sm">
            <thead>
              <tr className={rbacTheadRowClass}>
                <th scope="col" className={rbacThClass()}>
                  ID
                </th>
                <th scope="col" className={rbacThClass("right")}>
                  Supervisor
                </th>
                <th scope="col" className={rbacThClass("right")}>
                  Level Persetujuan
                </th>
                <th scope="col" className={rbacThClass()}>
                  Peran
                </th>
                <th scope="col" className={rbacThClass()}>
                  Sumber Autentikasi
                </th>
                <th scope="col" className={rbacThClass()}>
                  Aksi
                </th>
              </tr>
            </thead>
            <tbody>
              {users.map((user) =>
                editingId === user.id ? (
                  <HierarchyEditRow key={user.id} user={user} onDone={() => setEditingId(null)} />
                ) : (
                  <HierarchyRow key={user.id} user={user} onEdit={() => setEditingId(user.id)} />
                ),
              )}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function HierarchyRow({ user, onEdit }: { user: RbacUser; onEdit: () => void }) {
  return (
    <tr className={rbacRowClass}>
      <td className={`${rbacTdClass} tabular-nums`}>{user.id}</td>
      <td className={`${rbacTdClass} text-right tabular-nums`}>{user.supervisor_id ?? "—"}</td>
      <td className={`${rbacTdClass} text-right tabular-nums`}>{user.approval_level ?? "—"}</td>
      <td className={rbacTdClass}>
        <RoleBadge role={user.role} />
      </td>
      <td className={`${rbacTdClass} text-[var(--n-600)]`}>{user.auth_source}</td>
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

/** "" -> null, otherwise Number(v) -- see the comment on nullablePositiveInt in types.ts. */
function nullableNumber(value: string): number | null {
  return value === "" ? null : Number(value);
}

function HierarchyEditRow({ user, onDone }: { user: RbacUser; onDone: () => void }) {
  const mutation = useSetHierarchy();
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<HierarchyFormValues>({
    resolver: zodResolver(createHierarchySchema(user.id)),
  });

  const onSubmit = handleSubmit((values) => {
    mutation.mutate(
      { userId: user.id, values },
      {
        onSuccess: onDone,
      },
    );
  });

  return (
    <tr className="border-[var(--n-100)] border-b last:border-0 bg-[var(--n-50)]">
      <td className="px-3 py-3 align-top tabular-nums">{user.id}</td>
      <td colSpan={4} className="px-3 py-3 align-top">
        <form
          id={`hierarchy-form-${user.id}`}
          onSubmit={onSubmit}
          className="flex flex-wrap items-start gap-3"
        >
          <div className="w-36">
            <FormField
              label="Supervisor ID"
              htmlFor={`supervisor_id-${user.id}`}
              error={errors.supervisor_id?.message}
            >
              <input
                id={`supervisor_id-${user.id}`}
                type="number"
                defaultValue={user.supervisor_id ?? ""}
                className={rbacInputClass}
                {...register("supervisor_id", { setValueAs: nullableNumber })}
              />
            </FormField>
          </div>
          <div className="w-36">
            <FormField
              label="Level Persetujuan"
              htmlFor={`approval_level-${user.id}`}
              error={errors.approval_level?.message}
            >
              <input
                id={`approval_level-${user.id}`}
                type="number"
                defaultValue={user.approval_level ?? ""}
                className={rbacInputClass}
                {...register("approval_level", { setValueAs: nullableNumber })}
              />
            </FormField>
          </div>
          {mutation.isError && (
            <p className="flex items-center gap-1 text-xs text-[var(--danger-fg)] self-center">
              {mutation.error.message}
            </p>
          )}
        </form>
      </td>
      <td className="px-3 py-3 align-top">
        <div className="flex items-center gap-1">
          <button
            type="submit"
            form={`hierarchy-form-${user.id}`}
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
