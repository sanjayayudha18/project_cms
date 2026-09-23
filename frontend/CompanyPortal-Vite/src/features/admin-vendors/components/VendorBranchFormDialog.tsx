import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { useToast } from "@/lib/hooks/useToast";
import { applyServerFieldError } from "@/lib/utils/applyServerFieldError";
import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { pendingApprovalMessage } from "../../master-data/changeRequest";
import { useCreateVendorBranch } from "../hooks";
import { type VendorBranchFormValues, vendorBranchFormSchema } from "../lib/vendorBranchFormSchema";
import { VENDOR_VAULT_CATEGORIES } from "../lib/vendorVaultFormSchema";
import type { CreateVendorBranchPayload } from "../types";

interface VendorBranchFormDialogProps {
  open: boolean;
  onClose: () => void;
  vendorId: number;
}

const EMPTY_VALUES: VendorBranchFormValues = {
  branch_code: "",
  branch_name: "",
  location_id: "",
  region: "",
  category: "ATM",
};

/**
 * Create form for a vendor branch. Editing an existing branch is a separate
 * page (/settings/admin/vendors/$vendorId/branches/$branchId/edit), not this
 * dialog -- see BranchesPanel's "Ubah" link.
 */
export function VendorBranchFormDialog({ open, onClose, vendorId }: VendorBranchFormDialogProps) {
  const { toast } = useToast();
  const createMutation = useCreateVendorBranch(vendorId);

  const form = useForm<VendorBranchFormValues>({
    resolver: zodResolver(vendorBranchFormSchema),
    defaultValues: EMPTY_VALUES,
  });

  // biome-ignore lint/correctness/useExhaustiveDependencies: reset only when the dialog (re)opens, not on every render
  useEffect(() => {
    if (open) form.reset(EMPTY_VALUES);
  }, [open]);

  const isPending = createMutation.isPending;

  async function onSubmit(values: VendorBranchFormValues): Promise<void> {
    try {
      const payload: CreateVendorBranchPayload = {
        branch_code: values.branch_code,
        branch_name: values.branch_name,
        location_id: values.location_id ? Number(values.location_id) : null,
        region: values.region || null,
        category: values.category,
      };
      const res = await createMutation.mutateAsync(payload);
      toast({ type: "success", message: pendingApprovalMessage(res) });
      onClose();
    } catch (err) {
      const handled = applyServerFieldError(err, (field, error) =>
        form.setError(field as keyof VendorBranchFormValues, error),
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
    <Dialog open={open} onClose={onClose} title="Tambah Cabang">
      <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-col gap-4">
        <Field label="Kode Cabang" error={form.formState.errors.branch_code?.message}>
          <input {...form.register("branch_code")} className={inputClass} />
        </Field>

        <Field label="Nama Cabang" error={form.formState.errors.branch_name?.message}>
          <input {...form.register("branch_name")} className={inputClass} />
        </Field>

        <Field label="Wilayah" error={form.formState.errors.region?.message}>
          <input {...form.register("region")} className={inputClass} />
        </Field>

        <Field label="Tipe" error={form.formState.errors.category?.message}>
          <select {...form.register("category")} className={inputClass}>
            {VENDOR_VAULT_CATEGORIES.map((value) => (
              <option key={value} value={value}>
                {value}
              </option>
            ))}
          </select>
        </Field>

        <Field label="ID Lokasi (opsional)" error={form.formState.errors.location_id?.message}>
          <input {...form.register("location_id")} inputMode="numeric" className={inputClass} />
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
