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
      vault_name: "",
      category: "ATM",
      currency_code: "IDR",
      alamat: "",
      min_balance: "",
      max_balance: "",
      cis_limit_amount: "",
      escrow_cash: "",
      escrow_atm: "",
      operating_hours: "",
    };
  }
  return {
    vault_name: vault.vault_name ?? vault.vault_code ?? "",
    category: vault.category,
    currency_code: vault.currency_code,
    alamat: vault.alamat ?? "",
    min_balance: vault.min_balance ?? "",
    max_balance: vault.max_balance ?? "",
    cis_limit_amount: vault.cis_limit_amount ?? "",
    escrow_cash: vault.escrow_cash ?? "",
    escrow_atm: vault.escrow_atm ?? "",
    operating_hours: vault.operating_hours ?? "",
  };
}

/** Create/edit form for a vault under one branch. */
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
    const trimmedName = values.vault_name.trim();
    return {
      vault_name: trimmedName || null,
      category: values.category,
      currency_code: values.currency_code.toUpperCase(),
      alamat: values.alamat.trim() || null,
      min_balance: values.min_balance.trim() || null,
      max_balance: values.max_balance.trim() || null,
      cis_limit_amount: values.cis_limit_amount.trim() || null,
      escrow_cash: values.escrow_cash.trim() || null,
      escrow_atm: values.escrow_atm.trim() || null,
      operating_hours: values.operating_hours.trim() || null,
    };
  }

  async function onSubmit(values: VendorVaultFormValues): Promise<void> {
    try {
      if (mode === "create") {
        const updatePayload = toUpdatePayload(values);
        const payload: CreateVendorVaultPayload = {
          vendor_branch_id: branchId,
          vault_code: updatePayload.vault_name ?? "",
          ...updatePayload,
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
      const handled = applyServerFieldError(err, (field, error) => {
        const targetField = field === "vault_code" ? "vault_name" : field;
        form.setError(targetField as keyof VendorVaultFormValues, error);
      });
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
        <Field label="Nama Vault" error={form.formState.errors.vault_name?.message}>
          <input
            {...form.register("vault_name")}
            placeholder="Otomatis jika dikosongkan (mis. ABA_Bali_V001)"
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

        <Field label="Alamat" error={form.formState.errors.alamat?.message}>
          <input {...form.register("alamat")} className={inputClass} />
        </Field>

        <div className="grid grid-cols-2 gap-4">
          <Field label="Min Balance" error={form.formState.errors.min_balance?.message}>
            <input {...form.register("min_balance")} inputMode="decimal" className={inputClass} />
          </Field>
          <Field label="Max Balance" error={form.formState.errors.max_balance?.message}>
            <input {...form.register("max_balance")} inputMode="decimal" className={inputClass} />
          </Field>
        </div>

        <Field label="Limit CIS" error={form.formState.errors.cis_limit_amount?.message}>
          <input
            {...form.register("cis_limit_amount")}
            inputMode="decimal"
            className={inputClass}
          />
        </Field>

        <div className="grid grid-cols-2 gap-4">
          <Field label="Escrow Cash" error={form.formState.errors.escrow_cash?.message}>
            <input
              {...form.register("escrow_cash")}
              inputMode="numeric"
              maxLength={12}
              placeholder="12 digit angka"
              className={`${inputClass} font-mono`}
            />
          </Field>
          <Field label="Escrow ATM" error={form.formState.errors.escrow_atm?.message}>
            <input
              {...form.register("escrow_atm")}
              inputMode="numeric"
              maxLength={12}
              placeholder="12 digit angka"
              className={`${inputClass} font-mono`}
            />
          </Field>
        </div>

        <Field label="Jam Operasional" error={form.formState.errors.operating_hours?.message}>
          <input {...form.register("operating_hours")} className={inputClass} />
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
