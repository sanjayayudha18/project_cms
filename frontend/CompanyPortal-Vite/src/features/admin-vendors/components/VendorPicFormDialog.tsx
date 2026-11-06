import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { useToast } from "@/lib/hooks/useToast";
import { applyServerFieldError } from "@/lib/utils/applyServerFieldError";
import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { pendingApprovalMessage } from "../../master-data/changeRequest";
import { useCreateVendorPic, useUpdateVendorPic } from "../hooks";
import { ALL_PIC_ROLES, PIC_ROLE_OPTIONS } from "../lib/picRoles";
import { type VendorPicFormValues, vendorPicFormSchema } from "../lib/vendorPicFormSchema";
import type { AdminVendorPic, VendorPicFormPayload } from "../types";

interface VendorPicFormDialogProps {
  open: boolean;
  onClose: () => void;
  vendorId: number;
  /** Which branch this PIC belongs to, or null for a vendor-wide PIC -- fixed by context. */
  branchId: number | null;
  pic: AdminVendorPic | null;
}

function toDefaultValues(pic: AdminVendorPic | null): VendorPicFormValues {
  if (!pic) {
    return {
      name: "",
      position: "",
      phone: "",
      email: "",
      is_notification_recipient: false,
      is_director: false,
      roles: [...ALL_PIC_ROLES],
    };
  }
  return {
    name: pic.name,
    position: pic.position ?? "",
    phone: pic.phone ?? "",
    email: pic.email ?? "",
    is_notification_recipient: pic.is_notification_recipient,
    is_director: pic.is_director ?? false,
    roles: pic.roles,
  };
}

/** Create/edit form for a PIC, scoped to a branch or vendor-wide per `branchId`. */
export function VendorPicFormDialog({
  open,
  onClose,
  vendorId,
  branchId,
  pic,
}: VendorPicFormDialogProps) {
  const mode = pic ? "edit" : "create";
  const { toast } = useToast();
  const createMutation = useCreateVendorPic(vendorId);
  const updateMutation = useUpdateVendorPic(vendorId);

  const form = useForm<VendorPicFormValues>({
    resolver: zodResolver(vendorPicFormSchema),
    defaultValues: toDefaultValues(pic),
  });

  const [conflict, setConflict] = useState<string | null>(null);
  const isDirector = form.watch("is_director");

  // biome-ignore lint/correctness/useExhaustiveDependencies: reset only when the dialog (re)opens for a given pic, not on every render
  useEffect(() => {
    if (open) {
      form.reset(toDefaultValues(pic));
      setConflict(null);
    }
  }, [open, pic]);

  function onDirectorChange(checked: boolean): void {
    form.setValue("is_director", checked);
    if (checked) {
      form.setValue("is_notification_recipient", false);
      form.setValue("roles", []);
    } else if (mode === "create") {
      form.setValue("roles", [...ALL_PIC_ROLES]);
    }
  }

  const isPending = createMutation.isPending || updateMutation.isPending;

  function toPayload(values: VendorPicFormValues): VendorPicFormPayload {
    return {
      vendor_branch_id: branchId,
      name: values.name,
      position: values.position || null,
      phone: values.phone || null,
      email: values.email || null,
      is_notification_recipient: values.is_notification_recipient,
      // Absent = false on the server; only send the flag when set.
      is_director: values.is_director || undefined,
    };
  }

  async function onSubmit(values: VendorPicFormValues): Promise<void> {
    setConflict(null);
    try {
      if (mode === "create") {
        const res = await createMutation.mutateAsync({ ...toPayload(values), roles: values.roles });
        toast({ type: "success", message: pendingApprovalMessage(res) });
      } else {
        if (!pic) return;
        const res = await updateMutation.mutateAsync({ picId: pic.id, payload: toPayload(values) });
        toast({ type: "success", message: pendingApprovalMessage(res) });
      }
      onClose();
    } catch (err) {
      if (err && typeof err === "object" && (err as { status?: unknown }).status === 409) {
        setConflict(String((err as { message?: unknown }).message ?? "Data bentrok"));
        return;
      }
      const handled = applyServerFieldError(err, (field, error) =>
        form.setError(field as keyof VendorPicFormValues, error),
      );
      if (!handled) {
        const message =
          err && typeof err === "object" && "message" in err
            ? String((err as { message: unknown }).message)
            : "Terjadi kesalahan yang tidak diketahui";
        toast({ type: "error", message });
      }
    }
  }

  return (
    <Dialog open={open} onClose={onClose} title={mode === "create" ? "Tambah PIC" : "Ubah PIC"}>
      <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-col gap-4">
        <p className="text-xs text-[var(--n-500)]">
          {branchId === null ? "PIC berlaku untuk seluruh vendor" : "PIC untuk cabang ini"}
        </p>

        <Field label="Nama" error={form.formState.errors.name?.message}>
          <input {...form.register("name")} className={inputClass} />
        </Field>

        <Field label="Jabatan" error={form.formState.errors.position?.message}>
          <input {...form.register("position")} className={inputClass} />
        </Field>

        <Field label="Telepon" error={form.formState.errors.phone?.message}>
          <input {...form.register("phone")} className={inputClass} />
        </Field>

        <Field label="Email" error={form.formState.errors.email?.message}>
          <input type="email" {...form.register("email")} className={inputClass} />
        </Field>

        {conflict && (
          <p
            role="alert"
            className="rounded-[var(--radius-md)] border border-[var(--danger-fg)] px-3 py-2 text-sm text-[var(--danger-fg)]"
          >
            {conflict}
          </p>
        )}

        {branchId === null && (
          <div className="flex flex-col gap-1">
            <label className="flex min-h-[44px] items-center gap-2 text-sm text-[var(--n-800)]">
              <input
                type="checkbox"
                checked={isDirector}
                onChange={(e) => onDirectorChange(e.target.checked)}
              />
              Direktur Utama
            </label>
            {isDirector && (
              <span className="text-xs text-[var(--n-500)]">
                Hanya untuk pencatatan: tanpa login, tanpa peran PIC, tanpa notifikasi.
              </span>
            )}
          </div>
        )}

        <label className="flex items-center gap-2 text-sm text-[var(--n-800)]">
          <input
            type="checkbox"
            disabled={isDirector}
            {...form.register("is_notification_recipient")}
          />
          Penerima notifikasi
        </label>

        {mode === "create" && !isDirector && (
          <fieldset className="flex flex-col gap-1">
            <legend className="mb-1 text-xs font-medium uppercase tracking-wider text-[var(--n-600)]">
              Peran
            </legend>
            {PIC_ROLE_OPTIONS.map((opt) => (
              <label
                key={opt.value}
                className="flex min-h-[44px] items-center gap-2 text-sm text-[var(--n-800)]"
              >
                <input type="checkbox" value={opt.value} {...form.register("roles")} />
                {opt.label}
              </label>
            ))}
            <span className="text-xs text-[var(--n-500)]">
              Peran PIC yang sudah ada diubah lewat aksi "Ubah peran".
            </span>
          </fieldset>
        )}

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
  children,
}: {
  label: string;
  error?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-1">
      <span className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]">
        {label}
      </span>
      {children}
      {error && <span className="text-xs text-[var(--danger-fg)]">{error}</span>}
    </div>
  );
}
