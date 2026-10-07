import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { useVendorsList } from "@/features/admin-vendors/hooks";
import { useToast } from "@/lib/hooks/useToast";
import { applyServerFieldError } from "@/lib/utils/applyServerFieldError";
import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect } from "react";
import { Controller, useForm } from "react-hook-form";
import { useCreateUser, useUpdateUser } from "../hooks";
import { ROLE_OPTIONS, type UserFormValues, buildUserFormSchema } from "../lib/userFormSchema";
import type { AdminUser, CreateUserPayload, UpdateUserPayload } from "../types";

interface UserFormDialogProps {
  open: boolean;
  onClose: () => void;
  user: AdminUser | null;
}

function toDefaultValues(user: AdminUser | null): UserFormValues {
  if (!user) {
    return {
      username: "",
      full_name: "",
      email: "",
      role: "",
      is_karyawan: true,
      auth_source: "ldap",
      temporary_password: "",
      employee_id: "",
      vendor_id: null,
      supervisor_id: null,
      approval_level: null,
    };
  }
  return {
    username: user.username,
    full_name: user.full_name,
    email: user.email,
    role: user.role,
    is_karyawan: user.is_karyawan,
    auth_source: user.auth_source,
    temporary_password: "",
    employee_id: user.employee_id ?? "",
    vendor_id: user.vendor_id,
    supervisor_id: user.supervisor_id,
    approval_level: user.approval_level,
  };
}

/**
 * Create/edit form for admin Users (Req 3, 4). Username + Auth Source are
 * immutable once created (Req 4.2 -- ErrImmutableField), so both are
 * disabled in edit mode; Password Sementara only ever appears in create
 * mode for auth_source=local (Req 3.3, 11.3a).
 */
export function UserFormDialog({ open, onClose, user }: UserFormDialogProps) {
  const mode = user ? "edit" : "create";
  const { toast } = useToast();
  const vendorsQuery = useVendorsList({ page: 1, page_size: 100, status: "all" });
  const createMutation = useCreateUser();
  const updateMutation = useUpdateUser();

  const form = useForm<UserFormValues>({
    resolver: zodResolver(buildUserFormSchema(mode)),
    defaultValues: toDefaultValues(user),
  });

  // biome-ignore lint/correctness/useExhaustiveDependencies: reset only when the dialog (re)opens for a given user, not on every render
  useEffect(() => {
    if (open) form.reset(toDefaultValues(user));
  }, [open, user]);

  const authSource = form.watch("auth_source");
  const isPending = createMutation.isPending || updateMutation.isPending;

  async function onSubmit(values: UserFormValues): Promise<void> {
    try {
      if (mode === "create") {
        const payload: CreateUserPayload = {
          username: values.username,
          full_name: values.full_name,
          email: values.email,
          role: values.role,
          is_karyawan: values.is_karyawan,
          auth_source: values.auth_source,
          temporary_password:
            values.auth_source === "local" ? values.temporary_password : undefined,
          employee_id: values.employee_id,
          vendor_id: values.vendor_id,
          supervisor_id: values.supervisor_id,
          approval_level: values.approval_level,
        };
        await createMutation.mutateAsync(payload);
        toast({ type: "success", message: "Pengguna berhasil dibuat" });
      } else {
        if (!user) return;
        const payload: UpdateUserPayload = {
          full_name: values.full_name,
          email: values.email,
          role: values.role,
          is_karyawan: values.is_karyawan,
          employee_id: values.employee_id,
          vendor_id: values.vendor_id,
          supervisor_id: values.supervisor_id,
          approval_level: values.approval_level,
        };
        await updateMutation.mutateAsync({ id: user.id, payload });
        toast({ type: "success", message: "Pengguna berhasil diperbarui" });
      }
      onClose();
    } catch (err) {
      const handled = applyServerFieldError(err, (field, error) =>
        form.setError(field as keyof UserFormValues, error),
      );
      if (!handled) {
        const message =
          err && typeof err === "object" && "message" in err
            ? String((err as { message: unknown }).message)
            : "Terjadi kesalahan yang tidak diketahui";
        toast({ type: "error", message });
      }
      // Dialog stays open either way so the operator can see/fix the error.
    }
  }

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title={mode === "create" ? "Tambah Pengguna" : "Ubah Pengguna"}
    >
      <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-col gap-4">
        <Field label="Nama Pengguna" error={form.formState.errors.username?.message}>
          <input {...form.register("username")} disabled={mode === "edit"} className={inputClass} />
        </Field>

        <Field label="Nama Lengkap" error={form.formState.errors.full_name?.message}>
          <input {...form.register("full_name")} className={inputClass} />
        </Field>

        <Field label="Email" error={form.formState.errors.email?.message}>
          <input type="email" {...form.register("email")} className={inputClass} />
        </Field>

        <Field label="Role" error={form.formState.errors.role?.message}>
          <select {...form.register("role")} className={inputClass}>
            <option value="">Pilih role</option>
            {ROLE_OPTIONS.map((r) => (
              <option key={r} value={r}>
                {r}
              </option>
            ))}
          </select>
        </Field>

        <Field label="Auth Source" error={form.formState.errors.auth_source?.message}>
          <select
            {...form.register("auth_source")}
            disabled={mode === "edit"}
            className={inputClass}
          >
            <option value="ldap">LDAP (internal)</option>
            <option value="local">Local (vendor)</option>
          </select>
        </Field>

        {mode === "create" && authSource === "local" && (
          <Field
            label="Password Sementara"
            error={form.formState.errors.temporary_password?.message}
            hint="Pengguna akan diminta mengganti password ini saat login pertama."
          >
            <input
              type="password"
              {...form.register("temporary_password")}
              className={inputClass}
            />
          </Field>
        )}

        {authSource === "local" && (
          <Field label="Vendor" error={form.formState.errors.vendor_id?.message}>
            <Controller
              control={form.control}
              name="vendor_id"
              render={({ field }) => (
                <select
                  value={field.value ?? ""}
                  onChange={(e) => field.onChange(e.target.value ? Number(e.target.value) : null)}
                  className={inputClass}
                >
                  <option value="">Pilih vendor</option>
                  {(vendorsQuery.data?.vendors ?? []).map((v) => (
                    <option key={v.id} value={v.id}>
                      {v.name}
                    </option>
                  ))}
                </select>
              )}
            />
          </Field>
        )}

        <Field label="Employee ID" error={form.formState.errors.employee_id?.message}>
          <input {...form.register("employee_id")} className={inputClass} />
        </Field>

        <label className="flex min-h-[44px] items-center gap-2 text-sm text-[var(--n-700)]">
          <input type="checkbox" {...form.register("is_karyawan")} className="h-4 w-4" />
          Karyawan internal
        </label>

        <div className="flex justify-end gap-3 pt-2">
          <Button type="button" variant="secondary" disabled={isPending} onClick={onClose}>
            Batal
          </Button>
          <Button type="submit" disabled={isPending}>
            Simpan
          </Button>
        </div>
      </form>
    </Dialog>
  );
}

const inputClass =
  "min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)] disabled:bg-[var(--n-100)] disabled:text-[var(--n-500)]";

function Field({
  label,
  error,
  hint,
  children,
}: {
  label: string;
  error?: string;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-1">
      <span className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]">
        {label}
      </span>
      {children}
      {hint && !error && <span className="text-xs text-[var(--n-500)]">{hint}</span>}
      {error && <span className="text-xs text-[var(--danger-fg)]">{error}</span>}
    </div>
  );
}
