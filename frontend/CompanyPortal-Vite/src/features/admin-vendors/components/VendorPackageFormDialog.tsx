import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { useToast } from "@/lib/hooks/useToast";
import { applyServerFieldError } from "@/lib/utils/applyServerFieldError";
import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { pendingApprovalMessage } from "../../master-data/changeRequest";
import { useCreateVendorPackage, useUpdateVendorPackage } from "../hooks";
import {
  type VendorPackageFormValues,
  vendorPackageFormSchema,
} from "../lib/vendorPackageFormSchema";
import type {
  AdminVendorPackage,
  CreateVendorPackagePayload,
  UpdateVendorPackagePayload,
} from "../types";

interface VendorPackageFormDialogProps {
  open: boolean;
  onClose: () => void;
  vendorId: number;
  /** The branch this special price belongs to -- fixed by context, not user-editable. */
  branchId: number;
  /** Present = edit mode; null/omitted = create mode. */
  pkg?: AdminVendorPackage | null;
}

function toDefaultValues(pkg: AdminVendorPackage | null | undefined): VendorPackageFormValues {
  if (!pkg) {
    return {
      package_code: "",
      machine_group: "ATM",
      price_class: "REGULAR",
      tier_min: "1",
      tier_max: "",
      atm_id: "",
      currency: "IDR",
      effective_start_date: "",
      base_price: "",
      sla_note: "",
      effective_end_date: "",
    };
  }
  return {
    package_code: pkg.package_code,
    machine_group: pkg.machine_group,
    price_class: pkg.price_class,
    tier_min: String(pkg.tier_min),
    tier_max: pkg.tier_max === null ? "" : String(pkg.tier_max),
    atm_id: pkg.atm_id === null ? "" : String(pkg.atm_id),
    currency: pkg.currency,
    effective_start_date: pkg.effective_start_date,
    base_price: pkg.base_price ?? "",
    sla_note: pkg.sla_note ?? "",
    effective_end_date: pkg.effective_end_date ?? "",
  };
}

/**
 * Create/edit form for one branch's own special price ("harga khusus
 * cabang", migration 016). Grain fields (paket, kelompok mesin, kelas,
 * tingkat, ATM, mata uang, tanggal mulai) are immutable once created --
 * disabled in edit mode, same convention as VendorPackagePriceFormDialog.
 * Only the content fields (harga, catatan SLA, tanggal berakhir) can be
 * edited afterward. vendor_branch_id comes from branchId (URL context), not
 * a form field.
 */
export function VendorPackageFormDialog({
  open,
  onClose,
  vendorId,
  branchId,
  pkg = null,
}: VendorPackageFormDialogProps) {
  const mode = pkg ? "edit" : "create";
  const { toast } = useToast();
  const createMutation = useCreateVendorPackage(vendorId);
  const updateMutation = useUpdateVendorPackage(vendorId);

  const form = useForm<VendorPackageFormValues>({
    resolver: zodResolver(vendorPackageFormSchema),
    defaultValues: toDefaultValues(pkg),
  });

  // biome-ignore lint/correctness/useExhaustiveDependencies: reset only when the dialog (re)opens for a given row, not on every render
  useEffect(() => {
    if (open) form.reset(toDefaultValues(pkg));
  }, [open, pkg]);

  const isPending = createMutation.isPending || updateMutation.isPending;

  function toNullableString(v: string): string | null {
    return v === "" ? null : v;
  }

  function toNullableNumber(v: string): number | null {
    return v === "" ? null : Number(v);
  }

  async function onSubmit(values: VendorPackageFormValues): Promise<void> {
    const content = {
      base_price: toNullableString(values.base_price),
      sla_note: toNullableString(values.sla_note),
      effective_end_date: toNullableString(values.effective_end_date),
    };
    try {
      if (mode === "create") {
        const payload: CreateVendorPackagePayload = {
          vendor_branch_id: branchId,
          package_code: values.package_code,
          machine_group: values.machine_group,
          price_class: values.price_class,
          tier_min: Number(values.tier_min),
          tier_max: toNullableNumber(values.tier_max),
          atm_id: toNullableNumber(values.atm_id),
          currency: values.currency,
          effective_start_date: values.effective_start_date,
          ...content,
        };
        const res = await createMutation.mutateAsync(payload);
        toast({ type: "success", message: pendingApprovalMessage(res) });
      } else {
        if (!pkg) return;
        const payload: UpdateVendorPackagePayload = content;
        const res = await updateMutation.mutateAsync({ packageId: pkg.id, payload });
        toast({ type: "success", message: pendingApprovalMessage(res) });
      }
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
    <Dialog
      open={open}
      onClose={onClose}
      title={mode === "create" ? "Tambah Harga Khusus Cabang" : "Ubah Harga Khusus Cabang"}
      className="max-h-[85vh] overflow-y-auto"
    >
      <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-col gap-4">
        <div className="grid grid-cols-2 gap-4">
          <Field label="Kode Paket" error={form.formState.errors.package_code?.message}>
            <input
              {...form.register("package_code")}
              disabled={mode === "edit"}
              className={inputClass}
            />
          </Field>
          <Field label="Kelompok Mesin" error={form.formState.errors.machine_group?.message}>
            <select
              {...form.register("machine_group")}
              disabled={mode === "edit"}
              className={inputClass}
            >
              <option value="ATM">ATM</option>
              <option value="CDM_CRM">CDM/CRM</option>
            </select>
          </Field>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <Field label="Kelas Harga" error={form.formState.errors.price_class?.message}>
            <select
              {...form.register("price_class")}
              disabled={mode === "edit"}
              className={inputClass}
            >
              <option value="REGULAR">Nasional Regular</option>
              <option value="VIP_INDUSTRI">VIP &amp; Industri</option>
            </select>
          </Field>
          <Field label="Mata Uang" error={form.formState.errors.currency?.message}>
            <input
              {...form.register("currency")}
              disabled={mode === "edit"}
              className={inputClass}
            />
          </Field>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <Field
            label="Tingkat Min (jumlah kelolaan)"
            error={form.formState.errors.tier_min?.message}
          >
            <input
              {...form.register("tier_min")}
              inputMode="numeric"
              disabled={mode === "edit"}
              className={inputClass}
            />
          </Field>
          <Field
            label="Tingkat Maks (kosong = terbuka)"
            error={form.formState.errors.tier_max?.message}
          >
            <input
              {...form.register("tier_max")}
              inputMode="numeric"
              disabled={mode === "edit"}
              className={inputClass}
            />
          </Field>
        </div>

        <Field
          label="ID ATM (opsional, khusus satu ATM di cabang ini)"
          error={form.formState.errors.atm_id?.message}
        >
          <input
            {...form.register("atm_id")}
            inputMode="numeric"
            disabled={mode === "edit"}
            className={inputClass}
          />
        </Field>

        <div className="grid grid-cols-2 gap-4">
          <Field label="Mulai Berlaku" error={form.formState.errors.effective_start_date?.message}>
            <input
              type="date"
              {...form.register("effective_start_date")}
              disabled={mode === "edit"}
              className={inputClass}
            />
          </Field>
          <Field
            label="Berakhir (kosong = terbuka)"
            error={form.formState.errors.effective_end_date?.message}
          >
            <input type="date" {...form.register("effective_end_date")} className={inputClass} />
          </Field>
        </div>

        <Field
          label="Harga Khusus (kosong = ikut harga vendor biasa)"
          error={form.formState.errors.base_price?.message}
        >
          <input {...form.register("base_price")} inputMode="decimal" className={inputClass} />
        </Field>

        <Field label="Catatan SLA (opsional)" error={form.formState.errors.sla_note?.message}>
          <input {...form.register("sla_note")} className={inputClass} />
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
