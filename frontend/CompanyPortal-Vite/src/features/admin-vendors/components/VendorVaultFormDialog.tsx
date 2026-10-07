import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { useToast } from "@/lib/hooks/useToast";
import { applyServerFieldError } from "@/lib/utils/applyServerFieldError";
import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { pendingApprovalMessage } from "../../master-data/changeRequest";
import { useCreateVendorVault, useUpdateVendorVault } from "../hooks";
import {
  VENDOR_VAULT_CATEGORIES,
  type VendorVaultFormValues,
  vendorVaultFormSchema,
} from "../lib/vendorVaultFormSchema";
import type {
  AdminVendorVault,
  CreateVendorVaultPayload,
  UpdateVendorVaultPayload,
} from "../types";

interface VendorVaultFormDialogProps {
  open: boolean;
  onClose: () => void;
  vendorId: number;
  /** The branch this vault belongs to -- fixed by context, not user-editable. */
  branchId: number;
  vault: AdminVendorVault | null;
}

function toDefaultValues(vault: AdminVendorVault | null): VendorVaultFormValues {
  if (!vault) {
    return {
      vault_code: "",
      category: "ATM",
      currency_code: "IDR",
      min_capacity_amount: "",
      max_capacity_amount: "",
      latitude: "",
      longitude: "",
      operating_hours: "",
      location_id: "",
    };
  }
  return {
    vault_code: vault.vault_code,
    category: vault.category,
    currency_code: vault.currency_code,
    min_capacity_amount: vault.min_capacity_amount ?? "",
    max_capacity_amount: vault.max_capacity_amount ?? "",
    latitude: vault.latitude ?? "",
    longitude: vault.longitude ?? "",
    operating_hours: vault.operating_hours ?? "",
    location_id: vault.location_id ? String(vault.location_id) : "",
  };
}

/** Create/edit form for a vault under one branch (Kode vault immutable once created). */
export function VendorVaultFormDialog({
  open,
  onClose,
  vendorId,
  branchId,
  vault,
}: VendorVaultFormDialogProps) {
  const mode = vault ? "edit" : "create";
  const { toast } = useToast();
  const createMutation = useCreateVendorVault(vendorId);
  const updateMutation = useUpdateVendorVault(vendorId);

  const form = useForm<VendorVaultFormValues>({
    resolver: zodResolver(vendorVaultFormSchema),
    defaultValues: toDefaultValues(vault),
  });

  // biome-ignore lint/correctness/useExhaustiveDependencies: reset only when the dialog (re)opens for a given vault, not on every render
  useEffect(() => {
    if (open) form.reset(toDefaultValues(vault));
  }, [open, vault]);

  const isPending = createMutation.isPending || updateMutation.isPending;

  function toUpdatePayload(values: VendorVaultFormValues): UpdateVendorVaultPayload {
    return {
      category: values.category,
      currency_code: values.currency_code.toUpperCase(),
      min_capacity_amount: values.min_capacity_amount || null,
      max_capacity_amount: values.max_capacity_amount || null,
      latitude: values.latitude || null,
      longitude: values.longitude || null,
      operating_hours: values.operating_hours || null,
      location_id: values.location_id ? Number(values.location_id) : null,
    };
  }

  async function onSubmit(values: VendorVaultFormValues): Promise<void> {
    try {
      if (mode === "create") {
        const payload: CreateVendorVaultPayload = {
          vendor_branch_id: branchId,
          vault_code: values.vault_code,
          ...toUpdatePayload(values),
        };
        const res = await createMutation.mutateAsync(payload);
        toast({ type: "success", message: pendingApprovalMessage(res) });
      } else {
        if (!vault) return;
        const res = await updateMutation.mutateAsync({
          vaultId: vault.id,
          payload: toUpdatePayload(values),
        });
        toast({ type: "success", message: pendingApprovalMessage(res) });
      }
      onClose();
    } catch (err) {
      const handled = applyServerFieldError(err, (field, error) =>
        form.setError(field as keyof VendorVaultFormValues, error),
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
    <Dialog open={open} onClose={onClose} title={mode === "create" ? "Tambah Vault" : "Ubah Vault"}>
      <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-col gap-4">
        <Field label="Kode Vault" error={form.formState.errors.vault_code?.message}>
          <input
            {...form.register("vault_code")}
            disabled={mode === "edit"}
            className={inputClass}
          />
        </Field>

        <Field label="Kategori" error={form.formState.errors.category?.message}>
          <select {...form.register("category")} className={inputClass}>
            {VENDOR_VAULT_CATEGORIES.map((value) => (
              <option key={value} value={value}>
                {value}
              </option>
            ))}
          </select>
        </Field>

        <Field label="Mata Uang" error={form.formState.errors.currency_code?.message}>
          <input {...form.register("currency_code")} className={inputClass} />
        </Field>

        <div className="grid grid-cols-2 gap-4">
          <Field label="Kapasitas Min" error={form.formState.errors.min_capacity_amount?.message}>
            <input {...form.register("min_capacity_amount")} className={inputClass} />
          </Field>
          <Field label="Kapasitas Maks" error={form.formState.errors.max_capacity_amount?.message}>
            <input {...form.register("max_capacity_amount")} className={inputClass} />
          </Field>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <Field label="Latitude" error={form.formState.errors.latitude?.message}>
            <input {...form.register("latitude")} className={inputClass} />
          </Field>
          <Field label="Longitude" error={form.formState.errors.longitude?.message}>
            <input {...form.register("longitude")} className={inputClass} />
          </Field>
        </div>

        <Field label="Jam Operasional" error={form.formState.errors.operating_hours?.message}>
          <input {...form.register("operating_hours")} className={inputClass} />
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
