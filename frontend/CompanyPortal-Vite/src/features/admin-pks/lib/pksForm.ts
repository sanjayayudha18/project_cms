import { z } from "zod";

const ISO_DATE = /^\d{4}-\d{2}-\d{2}$/;
export const RENEWAL_MIN = 30;
export const RENEWAL_MAX = 3666;
export const RENEWAL_DEFAULT = 270;

/** PKS duration in days (expiry - effective), the upper bound of the reminder. */
export function pksDurationDays(effective: string, expiry: string): number | null {
  if (!ISO_DATE.test(effective) || !ISO_DATE.test(expiry)) return null;
  return Math.round((Date.parse(expiry) - Date.parse(effective)) / 86_400_000);
}

export const renewalDaysField = z
  .number({ error: "Reminder wajib diisi dengan angka" })
  .int("Harus bilangan bulat")
  .min(RENEWAL_MIN, `Minimal ${RENEWAL_MIN} hari`)
  .max(RENEWAL_MAX, `Maksimal ${RENEWAL_MAX} hari`);

export const pksFormSchema = z
  .object({
    vendor_id: z.string().min(1, "Vendor wajib dipilih"),
    pks_type: z.string().trim().min(1, "Type PKS wajib dipilih"),
    wod_number: z.string().trim().min(1, "No. WOD wajib diisi").max(100, "Maksimal 100 karakter"),
    pks_name: z.string().trim().min(1, "Wajib diisi").max(500, "Maksimal 500 karakter"),
    effective_date: z.string().regex(ISO_DATE, "Tanggal wajib diisi"),
    expiry_date: z.string().regex(ISO_DATE, "Tanggal wajib diisi"),
    renewal_days: renewalDaysField,
  })
  .superRefine((v, ctx) => {
    const days = pksDurationDays(v.effective_date, v.expiry_date);
    if (days === null) return;
    if (days < 0) {
      ctx.addIssue({
        code: "custom",
        path: ["expiry_date"],
        message: "Tanggal expired tidak boleh sebelum tanggal efektif",
      });
    } else if (v.renewal_days > days) {
      ctx.addIssue({
        code: "custom",
        path: ["renewal_days"],
        message: `Reminder tidak boleh melebihi masa PKS (${days} hari)`,
      });
    }
  });

export type PksFormValues = z.infer<typeof pksFormSchema>;

export const pksReasonSchema = z.object({
  reason: z.string().trim().min(1, "Alasan wajib diisi").max(500, "Maksimal 500 karakter"),
});
export type PksReasonValues = z.infer<typeof pksReasonSchema>;

export function errorMessage(err: unknown): string {
  return err && typeof err === "object" && "message" in err
    ? String((err as { message: unknown }).message)
    : "Terjadi kesalahan yang tidak diketahui";
}

export const inputClass =
  "min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)] disabled:bg-[var(--n-100)]";
export const labelClass = "text-xs font-medium uppercase tracking-wider text-[var(--n-600)]";
