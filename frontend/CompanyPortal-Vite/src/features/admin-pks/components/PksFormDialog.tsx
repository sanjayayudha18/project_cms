import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import type { ApiError } from "@/lib/api/client";
import { useToast } from "@/lib/hooks/useToast";
import { applyServerFieldError } from "@/lib/utils/applyServerFieldError";
import { zodResolver } from "@hookform/resolvers/zod";
import { useState } from "react";
import { type FieldPath, useForm } from "react-hook-form";
import { usePksTypesList, useVendorsList } from "../../admin-vendors/hooks";
import { pendingApprovalMessage } from "../../master-data/changeRequest";
import { useSubmitVendorPks } from "../hooks";
import {
  type PksFormValues,
  RENEWAL_DEFAULT,
  RENEWAL_MAX,
  RENEWAL_MIN,
  errorMessage,
  inputClass,
  labelClass,
  pksFormSchema,
} from "../lib/pksForm";
import type { PksListItem } from "../types";

const VENDOR_OPTIONS_LIMIT = 100;

interface PksFormDialogProps {
  /** Set = Ganti PKS: Vendor + Type are locked to this row. */
  replace?: PksListItem;
  onClose: () => void;
}

function Field({
  id,
  label,
  error,
  children,
}: {
  id: string;
  label: string;
  error?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-1">
      <label htmlFor={id} className={labelClass}>
        {label}
      </label>
      {children}
      {error && (
        <span id={`${id}-error`} className="text-xs text-[var(--danger-fg)]">
          {error}
        </span>
      )}
    </div>
  );
}

/** Catat PKS baru / Ganti PKS (maker-checker, 202 = diajukan). Mount only while open. */
export function PksFormDialog({ replace, onClose }: PksFormDialogProps) {
  const { toast } = useToast();
  const [banner, setBanner] = useState<string | null>(null);
  const typesQuery = usePksTypesList();
  const vendorsQuery = useVendorsList({
    page: 1,
    page_size: VENDOR_OPTIONS_LIMIT,
    status: "active",
    kind: "FLM_VENDOR",
  });
  const form = useForm<PksFormValues>({
    resolver: zodResolver(pksFormSchema),
    defaultValues: {
      vendor_id: replace ? String(replace.vendor_id) : "",
      pks_type: replace?.pks_type ?? "",
      wod_number: "",
      pks_name: "",
      effective_date: "",
      expiry_date: "",
      renewal_days: RENEWAL_DEFAULT,
    },
  });
  const vendorId = Number(form.watch("vendor_id")) || 0;
  const mutation = useSubmitVendorPks(vendorId);

  const vendors = vendorsQuery.data?.vendors ?? [];
  const types = (typesQuery.data ?? []).filter((t) => t.is_active || t.code === replace?.pks_type);
  const errors = form.formState.errors;
  const locked = Boolean(replace);

  async function onSubmit(values: PksFormValues): Promise<void> {
    setBanner(null);
    try {
      const res = await mutation.mutateAsync({
        pks_type: values.pks_type,
        wod_number: values.wod_number,
        pks_name: values.pks_name,
        effective_date: values.effective_date,
        expiry_date: values.expiry_date,
        renewal_days: values.renewal_days,
      });
      toast({ type: "success", message: pendingApprovalMessage(res) });
      onClose();
    } catch (err) {
      const apiErr = err as Partial<ApiError> | undefined;
      if (apiErr?.status === 409 && /wod|sudah dipakai/i.test(apiErr.message ?? "")) {
        form.setError("wod_number", { message: apiErr.message });
        return;
      }
      const handled = applyServerFieldError(err, (field, e) =>
        form.setError(field as FieldPath<PksFormValues>, e),
      );
      if (!handled) setBanner(errorMessage(err));
    }
  }

  return (
    <Dialog
      open
      onClose={onClose}
      title={replace ? `Ganti PKS: ${replace.pks_name}` : "Catat PKS baru"}
    >
      <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-col gap-4" noValidate>
        {banner && (
          <p role="alert" className="text-sm font-medium text-[var(--danger-fg)]">
            {banner}
          </p>
        )}
        {replace && (
          <p className="text-sm text-[var(--n-700)]">
            PKS saat ini pindah ke riwayat setelah PKS baru disetujui.
          </p>
        )}

        <Field id="pks-vendor" label="Vendor" error={errors.vendor_id?.message}>
          <select
            id="pks-vendor"
            aria-invalid={errors.vendor_id ? true : undefined}
            aria-describedby={errors.vendor_id ? "pks-vendor-error" : undefined}
            disabled={locked || vendorsQuery.isLoading}
            {...form.register("vendor_id")}
            className={inputClass}
          >
            <option value="">Pilih vendor</option>
            {locked && replace && (
              <option value={String(replace.vendor_id)}>
                {replace.vendor_code} - {replace.vendor_name}
              </option>
            )}
            {!locked &&
              vendors.map((v) => (
                <option key={v.id} value={String(v.id)}>
                  {v.code} - {v.name}
                </option>
              ))}
          </select>
        </Field>

        <Field id="pks-type" label="Type PKS" error={errors.pks_type?.message}>
          <select
            id="pks-type"
            aria-invalid={errors.pks_type ? true : undefined}
            aria-describedby={errors.pks_type ? "pks-type-error" : undefined}
            disabled={locked || typesQuery.isLoading}
            {...form.register("pks_type")}
            className={inputClass}
          >
            <option value="">Pilih Type PKS</option>
            {types.map((t) => (
              <option key={t.code} value={t.code}>
                {t.code} - {t.name}
              </option>
            ))}
          </select>
        </Field>

        <Field id="pks-wod" label="No. WOD" error={errors.wod_number?.message}>
          <input
            id="pks-wod"
            aria-invalid={errors.wod_number ? true : undefined}
            aria-describedby={errors.wod_number ? "pks-wod-error" : undefined}
            {...form.register("wod_number")}
            className={`${inputClass} tabular-nums`}
          />
        </Field>

        <Field id="pks-name" label="Judul PKS" error={errors.pks_name?.message}>
          <input
            id="pks-name"
            aria-invalid={errors.pks_name ? true : undefined}
            aria-describedby={errors.pks_name ? "pks-name-error" : undefined}
            {...form.register("pks_name")}
            className={inputClass}
          />
        </Field>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field id="pks-effective" label="Tanggal efektif" error={errors.effective_date?.message}>
            <input
              id="pks-effective"
              type="date"
              aria-invalid={errors.effective_date ? true : undefined}
              aria-describedby={errors.effective_date ? "pks-effective-error" : undefined}
              {...form.register("effective_date")}
              className={`${inputClass} tabular-nums`}
            />
          </Field>
          <Field id="pks-expiry" label="Tanggal expired" error={errors.expiry_date?.message}>
            <input
              id="pks-expiry"
              type="date"
              aria-invalid={errors.expiry_date ? true : undefined}
              aria-describedby={errors.expiry_date ? "pks-expiry-error" : undefined}
              {...form.register("expiry_date")}
              className={`${inputClass} tabular-nums`}
            />
          </Field>
        </div>

        <Field
          id="pks-renewal"
          label="Reminder (hari sebelum expired)"
          error={errors.renewal_days?.message}
        >
          <input
            id="pks-renewal"
            type="number"
            inputMode="numeric"
            min={RENEWAL_MIN}
            max={RENEWAL_MAX}
            aria-invalid={errors.renewal_days ? true : undefined}
            aria-describedby={errors.renewal_days ? "pks-renewal-error" : undefined}
            {...form.register("renewal_days", { valueAsNumber: true })}
            className={`${inputClass} tabular-nums`}
          />
        </Field>

        <div className="flex justify-end gap-3 pt-2">
          <Button type="button" variant="secondary" disabled={mutation.isPending} onClick={onClose}>
            Batal
          </Button>
          <Button type="submit" disabled={mutation.isPending}>
            Ajukan
          </Button>
        </div>
      </form>
    </Dialog>
  );
}
