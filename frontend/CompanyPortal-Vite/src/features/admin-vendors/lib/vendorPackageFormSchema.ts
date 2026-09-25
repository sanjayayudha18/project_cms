/**
 * Zod schema for the admin branch special-price ("harga khusus cabang",
 * migration 016) create/edit form, mirroring
 * backend/internal/service/vendor_package_admin.go's
 * validatePackageGrain/validatePriceContent. Grain fields (package_code,
 * machine_group, price_class, tier_min/max, atm_id, currency,
 * effective_start_date) are immutable after create -- the dialog disables
 * their inputs in edit mode rather than using a separate schema.
 * vendor_branch_id is fixed by the branch context the dialog is opened from
 * (not a form field here, unlike vendorPackagePriceFormSchema).
 */

import { z } from "zod";

const optionalDigits = z.string().refine((v) => v === "" || /^\d+$/.test(v), "Harus angka bulat");

const optionalDecimal = z
  .string()
  .refine((v) => v === "" || /^\d{1,18}(\.\d{1,2})?$/.test(v), "Harus angka desimal tidak negatif");

export const vendorPackageFormSchema = z
  .object({
    package_code: z.string().min(1, "Wajib diisi").max(50, "Maksimal 50 karakter"),
    machine_group: z.enum(["ATM", "CDM_CRM"]),
    price_class: z.enum(["REGULAR", "VIP_INDUSTRI"]),
    tier_min: z.string().regex(/^\d+$/, "Wajib angka bulat, minimal 1"),
    tier_max: optionalDigits,
    atm_id: optionalDigits,
    currency: z.string().length(3, "Kode mata uang 3 huruf"),
    effective_start_date: z.string().min(1, "Wajib diisi"),
    base_price: optionalDecimal,
    sla_note: z.string(),
    effective_end_date: z.string(),
  })
  .superRefine((values, ctx) => {
    if (
      values.effective_end_date !== "" &&
      values.effective_end_date < values.effective_start_date
    ) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["effective_end_date"],
        message: "Tidak boleh sebelum tanggal mulai",
      });
    }
  });

export type VendorPackageFormValues = z.infer<typeof vendorPackageFormSchema>;
