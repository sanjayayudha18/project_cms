import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { useToast } from "@/lib/hooks/useToast";
import { applyServerFieldError } from "@/lib/utils/applyServerFieldError";
import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { pendingApprovalMessage } from "../../master-data/changeRequest";
import { useCreateVendorPackage } from "../hooks";
import {
  type VendorPackageFormValues,
  vendorPackageFormSchema,
} from "../lib/vendorPackageFormSchema";
import type { CreateVendorPackagePayload } from "../types";

interface VendorPackageFormDialogProps {
  open: boolean;
  onClose: () => void;
  vendorId: number;
  /** The branch this package belongs to -- fixed by context, not user-editable. */
  branchId: number;
}

const DEFAULT_VALUES: VendorPackageFormValues = { code: "" };

/**
 * Create-only form for a package under one branch. Migration 010 left
 * vendor_branch_id + code as the package's only fields, both immutable after
 * create (a code/branch change is a new package) -- so there is no edit mode.
 */
export function VendorPackageFormDialog({
  open,
  onClose,
  vendorId,
  branchId,
}: VendorPackageFormDialogProps) {
  const { toast } = useToast();
  const createMutation = useCreateVendorPackage(vendorId);

  const form = useForm<VendorPackageFormValues>({
    resolver: zodResolver(vendorPackageFormSchema),
    defaultValues: DEFAULT_VALUES,
  });

  // biome-ignore lint/correctness/useExhaustiveDependencies: reset only when the dialog (re)opens, not on every render
  useEffect(() => {
    if (open) form.reset(DEFAULT_VALUES);
  }, [open]);

  async function onSubmit(values: VendorPackageFormValues): Promise<void> {
    try {
      const payload: CreateVendorPackagePayload = { vendor_branch_id: branchId, code: values.code };
      const res = await createMutation.mutateAsync(payload);
      toast({ type: "success", message: pendingApprovalMessage(res) });
      onClose();
    } catch (err) {
      const handled = applyServerFieldError(err, (field, error) =>
        form.setError(field as keyof VendorPackageFormValues, error),
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
    <Dialog open={open} onClose={onClose} title="Tambah Paket">
      <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-col gap-4">
        <Field label="Kode Paket" error={form.formState.errors.code?.message}>
          <input {...form.register("code")} className={inputClass} />
        </Field>

        <div className="flex justify-end gap-3 pt-2">
          <Button
            type="button"
            variant="secondary"
            disabled={createMutation.isPending}
            onClick={onClose}
          >
            Batal
          </Button>
          <Button type="submit" disabled={createMutation.isPending}>
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
