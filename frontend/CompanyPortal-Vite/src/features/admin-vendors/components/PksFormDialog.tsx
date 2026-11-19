import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import type { ApiError } from "@/lib/api/client";
import { useToast } from "@/lib/hooks/useToast";
import { applyServerFieldError } from "@/lib/utils/applyServerFieldError";
import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { pendingApprovalMessage } from "../../master-data/changeRequest";
import { usePksTypesList, useSubmitVendorPks } from "../hooks";
import type { VendorPksGroup } from "../types";

const ISO_DATE = /^\d{4}-\d{2}-\d{2}$/;

export const pksFormSchema = z
  .object({
    pks_type: z.string().trim().min(1, "Type PKS wajib dipilih"),
    wod_number: z.string().trim().min(1, "No. WOD wajib diisi").max(100, "Maksimal 100 karakter"),
    pks_name: z.string().trim().min(1, "Wajib diisi").max(500, "Maksimal 500 karakter"),
    effective_date: z.string().regex(ISO_DATE, "Tanggal wajib diisi"),
    expiry_date: z.string().regex(ISO_DATE, "Tanggal wajib diisi"),
  })
  .refine(
    (v) =>
      !ISO_DATE.test(v.effective_date) ||
      !ISO_DATE.test(v.expiry_date) ||
      v.expiry_date >= v.effective_date,
    { path: ["expiry_date"], message: "Tanggal expired tidak boleh sebelum tanggal efektif" },
  );

export type PksFormValues = z.infer<typeof pksFormSchema>;

export interface PksFormDialogProps {
  open: boolean;
  onClose: () => void;
  vendorId: number;
  initialTypeCode?: string;
  /** True when an active PKS exists: the new one will replace it after approval. */
  replacesActive?: boolean;
  groups?: VendorPksGroup[];
}

function errorMessage(err: unknown): string {
  return err && typeof err === "object" && "message" in err
    ? String((err as { message: unknown }).message)
    : "Terjadi kesalahan yang tidak diketahui";
}

const inputClass =
  "min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]";

/** Catat PKS baru. 202 = diajukan; 422 = pesan per field; 409 = per field (WOD) atau banner di dialog. */
export function PksFormDialog({
  open,
  onClose,
  vendorId,
  initialTypeCode,
  replacesActive,
  groups,
}: PksFormDialogProps) {
  const { toast } = useToast();
  const mutation = useSubmitVendorPks(vendorId);
  const { data: pksTypes = [], isLoading: loadingTypes } = usePksTypesList();
  const [banner, setBanner] = useState<string | null>(null);

  const activeTypes = pksTypes.filter((t) => t.is_active);
  const defaultTypeCode =
    initialTypeCode ?? (activeTypes.length === 1 ? (activeTypes[0]?.code ?? "") : "");

  const form = useForm<PksFormValues>({
    resolver: zodResolver(pksFormSchema),
    defaultValues: {
      pks_type: defaultTypeCode,
      wod_number: "",
      pks_name: "",
      effective_date: "",
      expiry_date: "",
    },
  });

  // biome-ignore lint/correctness/useExhaustiveDependencies: reset only when the dialog (re)opens
  useEffect(() => {
    if (open) {
      form.reset({
        pks_type: defaultTypeCode,
        wod_number: "",
        pks_name: "",
        effective_date: "",
        expiry_date: "",
      });
      setBanner(null);
    }
  }, [open, defaultTypeCode]);

  const watchedType = form.watch("pks_type");
  const willReplace =
    replacesActive ??
    Boolean(groups?.some((g) => g.pks_type.code === watchedType && g.active !== null));

  async function onSubmit(values: PksFormValues): Promise<void> {
    setBanner(null);
    try {
      const res = await mutation.mutateAsync({
        pks_type: values.pks_type.trim(),
        wod_number: values.wod_number.trim(),
        pks_name: values.pks_name.trim(),
        effective_date: values.effective_date,
        expiry_date: values.expiry_date,
      });
      toast({ type: "success", message: pendingApprovalMessage(res) });
      onClose();
    } catch (err) {
      const apiErr = err as Partial<ApiError> | undefined;

      // Handle 409 conflict specifically on wod_number (WOD duplicate)
      if (
        apiErr?.status === 409 &&
        typeof apiErr.message === "string" &&
        (apiErr.message.toLowerCase().includes("wod") ||
          apiErr.message.toLowerCase().includes("sudah dipakai"))
      ) {
        form.setError("wod_number", { message: apiErr.message });
        return;
      }

      // Check details array for field errors (both 422 and 409 with details)
      if (apiErr && Array.isArray(apiErr.details) && apiErr.details.length > 0) {
        const first = apiErr.details[0] as { field?: string; message?: string };
        if (first.field && first.field in form.getValues()) {
          form.setError(first.field as keyof PksFormValues, {
            message: first.message ?? "Tidak valid",
          });
          return;
        }
      }

      const handled = applyServerFieldError(err, (field, e) =>
        form.setError(field as keyof PksFormValues, e),
      );
      if (!handled) setBanner(errorMessage(err));
    }
  }

  const errors = form.formState.errors;

  return (
    <Dialog open={open} onClose={onClose} title="Catat PKS baru">
      <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-col gap-4" noValidate>
        {banner && (
          <p role="alert" className="text-sm font-medium text-[var(--danger-fg)]">
            {banner}
          </p>
        )}
        {willReplace && (
          <p className="text-sm text-[var(--n-700)]">
            PKS saat ini pindah ke riwayat setelah PKS baru disetujui.
          </p>
        )}

        <div className="flex flex-col gap-1">
          <label
            htmlFor="pks-type"
            className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]"
          >
            Type PKS
          </label>
          <select
            id="pks-type"
            aria-invalid={errors.pks_type ? true : undefined}
            aria-describedby={errors.pks_type ? "pks-type-error" : undefined}
            {...form.register("pks_type")}
            className={inputClass}
            disabled={loadingTypes}
          >
            <option value="">Pilih Type PKS</option>
            {activeTypes.map((t) => (
              <option key={t.code} value={t.code}>
                {t.code} - {t.name}
              </option>
            ))}
          </select>
          {errors.pks_type && (
            <span id="pks-type-error" className="text-xs text-[var(--danger-fg)]">
              {errors.pks_type.message}
            </span>
          )}
        </div>

        <div className="flex flex-col gap-1">
          <label
            htmlFor="pks-wod"
            className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]"
          >
            No. WOD
          </label>
          <input
            id="pks-wod"
            aria-invalid={errors.wod_number ? true : undefined}
            aria-describedby={errors.wod_number ? "pks-wod-error" : undefined}
            {...form.register("wod_number")}
            className={`${inputClass} tabular-nums`}
          />
          {errors.wod_number && (
            <span id="pks-wod-error" className="text-xs text-[var(--danger-fg)]">
              {errors.wod_number.message}
            </span>
          )}
        </div>

        <div className="flex flex-col gap-1">
          <label
            htmlFor="pks-name"
            className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]"
          >
            Nama PKS
          </label>
          <input
            id="pks-name"
            aria-invalid={errors.pks_name ? true : undefined}
            aria-describedby={errors.pks_name ? "pks-name-error" : undefined}
            {...form.register("pks_name")}
            className={inputClass}
          />
          {errors.pks_name && (
            <span id="pks-name-error" className="text-xs text-[var(--danger-fg)]">
              {errors.pks_name.message}
            </span>
          )}
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div className="flex flex-col gap-1">
            <label
              htmlFor="pks-effective"
              className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]"
            >
              Tanggal efektif
            </label>
            <input
              id="pks-effective"
              type="date"
              aria-invalid={errors.effective_date ? true : undefined}
              aria-describedby={errors.effective_date ? "pks-effective-error" : undefined}
              {...form.register("effective_date")}
              className={`${inputClass} tabular-nums`}
            />
            {errors.effective_date && (
              <span id="pks-effective-error" className="text-xs text-[var(--danger-fg)]">
                {errors.effective_date.message}
              </span>
            )}
          </div>
          <div className="flex flex-col gap-1">
            <label
              htmlFor="pks-expiry"
              className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]"
            >
              Tanggal expired
            </label>
            <input
              id="pks-expiry"
              type="date"
              aria-invalid={errors.expiry_date ? true : undefined}
              aria-describedby={errors.expiry_date ? "pks-expiry-error" : undefined}
              {...form.register("expiry_date")}
              className={`${inputClass} tabular-nums`}
            />
            {errors.expiry_date && (
              <span id="pks-expiry-error" className="text-xs text-[var(--danger-fg)]">
                {errors.expiry_date.message}
              </span>
            )}
          </div>
        </div>

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
