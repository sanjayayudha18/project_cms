import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { useToast } from "@/lib/hooks/useToast";
import { applyServerFieldError } from "@/lib/utils/applyServerFieldError";
import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { useCreateVendor, useUpdateVendor } from "../hooks";
import { type VendorFormValues, vendorFormSchema } from "../lib/vendorFormSchema";
import type { AdminVendor, CreateVendorPayload, UpdateVendorPayload } from "../types";

interface VendorFormDialogProps {
  open: boolean;
  onClose: () => void;
  vendor: AdminVendor | null;
}

function toDefaultValues(vendor: AdminVendor | null): VendorFormValues {
  if (!vendor) {
    return { code: "", name: "", contact_email: "", contact_phone: "", hq_address: "" };
  }
  return {
    code: vendor.code,
    name: vendor.name,
    contact_email: vendor.contact_email,
    contact_phone: vendor.contact_phone,
    hq_address: vendor.hq_address,
  };
}

/** Create/edit form for admin Vendors (Req 7). Kode is immutable once created (Req 7.6). */
export function VendorFormDialog({ open, onClose, vendor }: VendorFormDialogProps) {
  const mode = vendor ? "edit" : "create";
  const { toast } = useToast();
  const createMutation = useCreateVendor();
  const updateMutation = useUpdateVendor();

  const form = useForm<VendorFormValues>({
    resolver: zodResolver(vendorFormSchema),
    defaultValues: toDefaultValues(vendor),
  });

  // biome-ignore lint/correctness/useExhaustiveDependencies: reset only when the dialog (re)opens for a given vendor, not on every render
  useEffect(() => {
    if (open) form.reset(toDefaultValues(vendor));
  }, [open, vendor]);

  const isPending = createMutation.isPending || updateMutation.isPending;

  async function onSubmit(values: VendorFormValues): Promise<void> {
    try {
      if (mode === "create") {
        const payload: CreateVendorPayload = values;
        await createMutation.mutateAsync(payload);
        toast({ type: "success", message: "Vendor berhasil dibuat" });
      } else {
        if (!vendor) return;
        const payload: UpdateVendorPayload = {
          name: values.name,
          contact_email: values.contact_email,
          contact_phone: values.contact_phone,
          hq_address: values.hq_address,
        };
        await updateMutation.mutateAsync({ id: vendor.id, payload });
        toast({ type: "success", message: "Vendor berhasil diperbarui" });
      }
      onClose();
    } catch (err) {
      const handled = applyServerFieldError(err, (field, error) =>
        form.setError(field as keyof VendorFormValues, error),
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
      title={mode === "create" ? "Tambah Vendor" : "Ubah Vendor"}
    >
      <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-col gap-4">
        <Field label="Kode" error={form.formState.errors.code?.message}>
          <input {...form.register("code")} disabled={mode === "edit"} className={inputClass} />
        </Field>

        <Field label="Nama" error={form.formState.errors.name?.message}>
          <input {...form.register("name")} className={inputClass} />
        </Field>

        <Field label="Email Kontak" error={form.formState.errors.contact_email?.message}>
          <input type="email" {...form.register("contact_email")} className={inputClass} />
        </Field>

        <Field label="Telepon" error={form.formState.errors.contact_phone?.message}>
          <input {...form.register("contact_phone")} className={inputClass} />
        </Field>

        <Field label="Alamat Kantor Pusat" error={form.formState.errors.hq_address?.message}>
          <textarea {...form.register("hq_address")} rows={3} className={inputClass} />
        </Field>

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
