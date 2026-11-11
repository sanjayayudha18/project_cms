import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { useToast } from "@/lib/hooks/useToast";
import { applyServerFieldError } from "@/lib/utils/applyServerFieldError";
import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { pendingApprovalMessage } from "../../master-data/changeRequest";
import { useSubmitVendorPks } from "../hooks";

const ISO_DATE = /^\d{4}-\d{2}-\d{2}$/;

export const pksFormSchema = z
  .object({
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

type PksFormValues = z.infer<typeof pksFormSchema>;

interface PksFormDialogProps {
  open: boolean;
  onClose: () => void;
  vendorId: number;
  /** True when an active PKS exists: the new one will replace it after approval. */
  replacesActive: boolean;
}

function errorMessage(err: unknown): string {
  return err && typeof err === "object" && "message" in err
    ? String((err as { message: unknown }).message)
    : "Terjadi kesalahan yang tidak diketahui";
}

const inputClass =
  "min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]";

/** Catat PKS baru. 202 = diajukan; 422 = pesan per field; 404/409 = banner di dialog. */
export function PksFormDialog({ open, onClose, vendorId, replacesActive }: PksFormDialogProps) {
  const { toast } = useToast();
  const mutation = useSubmitVendorPks(vendorId);
  const [banner, setBanner] = useState<string | null>(null);

  const form = useForm<PksFormValues>({
    resolver: zodResolver(pksFormSchema),
    defaultValues: { pks_name: "", effective_date: "", expiry_date: "" },
  });

  // biome-ignore lint/correctness/useExhaustiveDependencies: reset only when the dialog (re)opens
  useEffect(() => {
    if (open) {
      form.reset({ pks_name: "", effective_date: "", expiry_date: "" });
      setBanner(null);
    }
  }, [open]);

  async function onSubmit(values: PksFormValues): Promise<void> {
    setBanner(null);
    try {
      const res = await mutation.mutateAsync({ ...values, pks_name: values.pks_name.trim() });
      toast({ type: "success", message: pendingApprovalMessage(res) });
      onClose();
    } catch (err) {
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
        {replacesActive && (
          <p className="text-sm text-[var(--n-700)]">
            PKS saat ini pindah ke riwayat setelah PKS baru disetujui.
          </p>
        )}

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
