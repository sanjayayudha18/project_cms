/**
 * RBAC settings: user hierarchy (Requirements 4, 9), redesigned after the
 * UI-design/hierarki-pengguna-crown prototype -- supervisors (checker roles,
 * edit approval level) and regular users (maker roles, edit supervisor by
 * name) split into their own tables, plus a read-only admin & support
 * section, with a name/role search and a domain filter on top. Unlike the
 * prototype's fictional CAM/Parameter domains and approval tiers, domain and
 * level here are derived from CROWN's real seeded roles (types.ts
 * roleDomain/roleLevel) -- see backend/migrations/002_cms_tables.sql.
 */

import { FilterSelect } from "@/components/ui/FilterSelect";
import { PageHeader } from "@/components/ui/PageHeader";
import { zodResolver } from "@hookform/resolvers/zod";
import { Pencil, Search, Shield, User, Wrench, X } from "lucide-react";
import { useMemo, useState } from "react";
import type { ReactNode } from "react";
import { useForm } from "react-hook-form";
import { useSetHierarchy, useUserHierarchy } from "../hooks/useRbacQueries";
import {
  DOMAIN_LABEL,
  type HierarchyFormValues,
  type RbacUser,
  createHierarchySchema,
  roleDomain,
  roleLevel,
} from "../types";
import { FormField, rbacInputClass } from "./FormField";
import { RoleBadge } from "./RoleBadge";
import { StatusMessage } from "./StatusMessage";
import { rbacRowClass, rbacTdClass, rbacThClass, rbacTheadRowClass } from "./tableStyles";

const DOMAIN_OPTIONS = Object.entries(DOMAIN_LABEL)
  .filter(([domain]) => domain !== "admin")
  .map(([domain, label]) => ({ value: domain, label }));

function matchesQuery(user: RbacUser, supervisorName: string | undefined, query: string): boolean {
  if (!query) return true;
  const haystack = `${user.full_name} ${user.username} ${user.role} ${supervisorName ?? ""}`.toLowerCase();
  return haystack.includes(query.toLowerCase());
}

export function RbacUsersPage() {
  const { data, isLoading, isError } = useUserHierarchy();
  const [editingId, setEditingId] = useState<number | null>(null);
  const [query, setQuery] = useState("");
  const [domain, setDomain] = useState<string | null>(null);
  const users = data?.users ?? [];

  const nameOf = (id: number | null) => users.find((u) => u.id === id)?.full_name;
  const teamSize = (id: number) => users.filter((u) => u.supervisor_id === id).length;

  const visible = users.filter(
    (u) =>
      (domain === null || roleDomain(u.role) === domain) &&
      matchesQuery(u, nameOf(u.supervisor_id) ?? undefined, query),
  );
  const checkers = visible.filter((u) => roleLevel(u.role) === "checker");
  const makers = visible.filter((u) => roleLevel(u.role) === "maker");
  const admins = visible.filter((u) => roleLevel(u.role) === "admin");
  const allCheckers = users.filter((u) => roleLevel(u.role) === "checker");

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Pengaturan"
        title="Hierarki Pengguna"
        description="Supervisor menyetujui pengajuan; pengguna biasa membuat pengajuan. Atur level persetujuan supervisor, dan atasan pengguna biasa."
      />

      <div className="flex flex-wrap items-end gap-3">
        <div className="flex min-w-[240px] flex-1 flex-col gap-1">
          <label
            htmlFor="rbac-users-search"
            className="text-xs font-medium text-[var(--n-600)] uppercase tracking-wider"
          >
            Cari pengguna
          </label>
          <div className="relative">
            <Search
              className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--n-400)]"
              aria-hidden="true"
            />
            <input
              id="rbac-users-search"
              type="text"
              placeholder="Nama, peran, atau atasan"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              className={`${rbacInputClass} pl-9`}
            />
          </div>
        </div>
        <FilterSelect
          label="Domain"
          options={DOMAIN_OPTIONS}
          value={domain}
          onChange={setDomain}
          placeholder="Semua domain"
        />
      </div>

      <StatusMessage
        isLoading={isLoading}
        isError={isError}
        isEmpty={!isLoading && !isError && users.length === 0}
        loadingLabel="Memuat hierarki pengguna…"
        errorMessage="Gagal memuat hierarki pengguna. Coba lagi."
        emptyMessage="Belum ada pengguna."
      />

      {!isLoading && !isError && users.length > 0 && !visible.length && (
        <p className="py-8 text-center text-sm text-[var(--n-500)]">
          Pengguna tidak ditemukan untuk pencarian atau domain ini.
        </p>
      )}

      {!isLoading && !isError && visible.length > 0 && (
        <>
          <UserSection
            icon={Shield}
            title="Supervisor"
            description="Menyetujui pengajuan. Yang bisa diatur: level persetujuan."
          >
            {checkers.length > 0 ? (
              <table className="w-full min-w-[720px] border-collapse text-sm">
                <thead>
                  <tr className={rbacTheadRowClass}>
                    <th scope="col" className={rbacThClass()}>
                      Supervisor
                    </th>
                    <th scope="col" className={rbacThClass()}>
                      Peran
                    </th>
                    <th scope="col" className={rbacThClass("right")}>
                      Level Persetujuan
                    </th>
                    <th scope="col" className={rbacThClass("right")}>
                      Anggota
                    </th>
                    <th scope="col" className={rbacThClass()}>
                      Aksi
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {checkers.map((user) =>
                    editingId === user.id ? (
                      <ApprovalLevelEditRow key={user.id} user={user} onDone={() => setEditingId(null)} />
                    ) : (
                      <tr key={user.id} className={rbacRowClass}>
                        <td className={rbacTdClass}>
                          <UserCell user={user} />
                        </td>
                        <td className={rbacTdClass}>
                          <RoleBadge role={user.role} />
                        </td>
                        <td className={`${rbacTdClass} text-right tabular-nums`}>
                          {user.approval_level ?? "—"}
                        </td>
                        <td className={`${rbacTdClass} text-right tabular-nums`}>{teamSize(user.id)}</td>
                        <td className={rbacTdClass}>
                          <EditButton user={user} onClick={() => setEditingId(user.id)} />
                        </td>
                      </tr>
                    ),
                  )}
                </tbody>
              </table>
            ) : (
              <p className="p-4 text-sm text-[var(--n-500)]">Tidak ada supervisor pada filter ini.</p>
            )}
          </UserSection>

          <UserSection
            icon={User}
            title="Pengguna biasa"
            description="Membuat pengajuan. Yang bisa diatur: atasan."
          >
            {makers.length > 0 ? (
              <table className="w-full min-w-[720px] border-collapse text-sm">
                <thead>
                  <tr className={rbacTheadRowClass}>
                    <th scope="col" className={rbacThClass()}>
                      Pengguna
                    </th>
                    <th scope="col" className={rbacThClass()}>
                      Peran
                    </th>
                    <th scope="col" className={rbacThClass()}>
                      Atasan
                    </th>
                    <th scope="col" className={rbacThClass()}>
                      Aksi
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {makers.map((user) =>
                    editingId === user.id ? (
                      <SupervisorEditRow
                        key={user.id}
                        user={user}
                        candidates={allCheckers}
                        onDone={() => setEditingId(null)}
                      />
                    ) : (
                      <tr key={user.id} className={rbacRowClass}>
                        <td className={rbacTdClass}>
                          <UserCell user={user} />
                        </td>
                        <td className={rbacTdClass}>
                          <RoleBadge role={user.role} />
                        </td>
                        <td className={rbacTdClass}>
                          {user.supervisor_id ? (
                            <span>{nameOf(user.supervisor_id) ?? user.supervisor_id}</span>
                          ) : (
                            <span className="text-[var(--n-500)]">Belum diatur</span>
                          )}
                        </td>
                        <td className={rbacTdClass}>
                          <EditButton user={user} onClick={() => setEditingId(user.id)} />
                        </td>
                      </tr>
                    ),
                  )}
                </tbody>
              </table>
            ) : (
              <p className="p-4 text-sm text-[var(--n-500)]">Tidak ada pengguna biasa pada filter ini.</p>
            )}
          </UserSection>

          {admins.length > 0 && (
            <UserSection
              icon={Wrench}
              title="Admin & support"
              description="Di luar rantai persetujuan, tidak punya atasan."
            >
              <table className="w-full min-w-[720px] border-collapse text-sm">
                <thead>
                  <tr className={rbacTheadRowClass}>
                    <th scope="col" className={rbacThClass()}>
                      Pengguna
                    </th>
                    <th scope="col" className={rbacThClass()}>
                      Peran
                    </th>
                    <th scope="col" className={rbacThClass()}>
                      Keterangan
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {admins.map((user) => (
                    <tr key={user.id} className={rbacRowClass}>
                      <td className={rbacTdClass}>
                        <UserCell user={user} />
                      </td>
                      <td className={rbacTdClass}>
                        <RoleBadge role={user.role} />
                      </td>
                      <td className={`${rbacTdClass} text-[var(--n-500)]`}>Tidak ikut maker-checker</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </UserSection>
          )}
        </>
      )}
    </div>
  );
}

function UserSection({
  icon: Icon,
  title,
  description,
  children,
}: {
  icon: typeof Shield;
  title: string;
  description: string;
  children: ReactNode;
}) {
  return (
    <section className="flex flex-col gap-3" aria-labelledby={`rbac-section-${title}`}>
      <div className="flex items-center gap-2">
        <Icon className="h-5 w-5 text-[var(--red-500)]" aria-hidden="true" />
        <div>
          <h2 id={`rbac-section-${title}`} className="text-sm font-semibold text-[var(--n-900)]">
            {title}
          </h2>
          <p className="text-xs text-[var(--n-500)]">{description}</p>
        </div>
      </div>
      <div className="overflow-x-auto rounded-[var(--radius-lg)] border border-[var(--n-200)]">{children}</div>
    </section>
  );
}

function UserCell({ user }: { user: RbacUser }) {
  return (
    <div className="flex flex-col">
      <span className="font-medium text-[var(--n-900)]">{user.full_name}</span>
      <span className="text-xs text-[var(--n-500)]">
        {user.username}
        {user.vendor_name ? ` · ${user.vendor_name}` : ""}
      </span>
    </div>
  );
}

function EditButton({ user, onClick }: { user: RbacUser; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="inline-flex items-center gap-1 rounded-[var(--radius-sm)] px-2 py-1 text-xs font-medium text-[var(--red-600)] hover:bg-[var(--red-50)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)] outline-none"
    >
      <Pencil className="h-3.5 w-3.5" aria-hidden="true" />
      Ubah {user.full_name}
    </button>
  );
}

/** "" -> null, otherwise Number(v) -- see the comment on nullablePositiveInt in types.ts. */
function nullableNumber(value: string): number | null {
  return value === "" ? null : Number(value);
}

function ApprovalLevelEditRow({ user, onDone }: { user: RbacUser; onDone: () => void }) {
  const mutation = useSetHierarchy();
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<HierarchyFormValues>({
    resolver: zodResolver(createHierarchySchema(user.id)),
    defaultValues: { supervisor_id: user.supervisor_id, approval_level: user.approval_level },
  });

  const onSubmit = handleSubmit((values) => {
    mutation.mutate(
      { userId: user.id, values: { ...values, supervisor_id: user.supervisor_id } },
      { onSuccess: onDone },
    );
  });

  return (
    <EditRow user={user} colSpan={4} mutation={mutation} onSubmit={onSubmit} onCancel={onDone}>
      <div className="w-40">
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
    </EditRow>
  );
}

function SupervisorEditRow({
  user,
  candidates,
  onDone,
}: {
  user: RbacUser;
  candidates: RbacUser[];
  onDone: () => void;
}) {
  const mutation = useSetHierarchy();
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<HierarchyFormValues>({
    resolver: zodResolver(createHierarchySchema(user.id)),
    defaultValues: { supervisor_id: user.supervisor_id, approval_level: user.approval_level },
  });

  const eligible = useMemo(
    () =>
      candidates.filter(
        (c) =>
          c.id !== user.id &&
          roleDomain(c.role) === roleDomain(user.role) &&
          (roleDomain(user.role) !== "VENDOR" || c.vendor_id === user.vendor_id),
      ),
    [candidates, user],
  );

  const onSubmit = handleSubmit((values) => {
    mutation.mutate(
      { userId: user.id, values: { ...values, approval_level: user.approval_level } },
      { onSuccess: onDone },
    );
  });

  return (
    <EditRow user={user} colSpan={3} mutation={mutation} onSubmit={onSubmit} onCancel={onDone}>
      <div className="w-52">
        <FormField label="Supervisor ID" htmlFor={`supervisor_id-${user.id}`} error={errors.supervisor_id?.message}>
          <select
            id={`supervisor_id-${user.id}`}
            defaultValue={user.supervisor_id ?? ""}
            className={rbacInputClass}
            {...register("supervisor_id", { setValueAs: nullableNumber })}
          >
            <option value="">Belum diatur</option>
            {eligible.map((c) => (
              <option key={c.id} value={c.id}>
                {c.full_name}
              </option>
            ))}
          </select>
        </FormField>
      </div>
    </EditRow>
  );
}

interface EditRowMutation {
  isPending: boolean;
  isError: boolean;
  error: { message: string } | null;
}

function EditRow({
  user,
  colSpan,
  mutation,
  onSubmit,
  onCancel,
  children,
}: {
  user: RbacUser;
  colSpan: number;
  mutation: EditRowMutation;
  onSubmit: (e: React.FormEvent) => void;
  onCancel: () => void;
  children: ReactNode;
}) {
  return (
    <tr className="border-[var(--n-100)] border-b last:border-0 bg-[var(--n-50)]">
      <td className="px-3 py-3 align-top">
        <UserCell user={user} />
      </td>
      <td colSpan={colSpan - 1} className="px-3 py-3 align-top">
        <form id={`hierarchy-form-${user.id}`} onSubmit={onSubmit} className="flex flex-wrap items-start gap-3">
          {children}
          {mutation.isError && (
            <p className="flex items-center gap-1 text-xs text-[var(--danger-fg)] self-center">
              {mutation.error?.message}
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
            onClick={onCancel}
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
