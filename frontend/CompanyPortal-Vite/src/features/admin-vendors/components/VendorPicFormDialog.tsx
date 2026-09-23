import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { useToast } from "@/lib/hooks/useToast";
import { applyServerFieldError } from "@/lib/utils/applyServerFieldError";
import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { pendingApprovalMessage } from "../../master-data/changeRequest";
import { useCreateVendorPic, useUpdateVendorPic } from "../hooks";
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
    return { name: "", position: "", phone: "", email: "", is_notification_recipient: false };
  }
  return {
    name: pic.name,
    position: pic.position ?? "",
    phone: pic.phone ?? "",
    email: pic.email ?? "",
    is_notification_recipient: pic.is_notification_recipient,
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

  // biome-ignore lint/correctness/useExhaustiveDependencies: reset only when the dialog (re)opens for a given pic, not on every render
  useEffect(() => {
    if (open) form.reset(toDefaultValues(pic));
  }, [open, pic]);

  const isPending = createMutation.isPending || updateMutation.isPending;

  function toPayload(values: VendorPicFormValues): VendorPicFormPayload {
    return {
      vendor_branch_id: branchId,
      name: values.name,
      position: values.position || null,
      phone: values.phone || null,
      email: values.email || null,
      is_notification_recipient: values.is_notification_recipient,
    };
  }

  async function onSubmit(values: VendorPicFormValues): Promise<void> {
    try {
      if (mode === "create") {
        const res = await createMutation.mutateAsync(toPayload(values));
        toast({ type: "success", message: pendingApprovalMessage(res) });
      } else {
        if (!pic) return;
        const res = await updateMutation.mutateAsync({ picId: pic.id, payload: toPayload(values) });
        toast({ type: "success", message: pendingApprovalMessage(res) });
      }
      onClose();
    } catch (err) {
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

        <label className="flex items-center gap-2 text-sm text-[var(--n-800)]">
          <input type="checkbox" {...form.register("is_notification_recipient")} />
          Penerima notifikasi
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
