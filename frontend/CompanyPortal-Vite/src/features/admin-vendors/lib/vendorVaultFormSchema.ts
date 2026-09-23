/**
 * Zod schema for the admin Vendor Vault create/edit form. Decimal amounts and
 * coordinates are validated server-side (backend/internal/service/
 * vendor_vault_admin.go's validateVaultFields, big.Rat parsing) and mapped
 * back to the matching field via applyServerFieldError -- this schema only
 * enforces the checks that don't need that parser (required fields, category
 * enum, currency format).
 */

import { z } from "zod";

export const VENDOR_VAULT_CATEGORIES = ["ATM", "CASH", "ATM_CASH"] as const;

export const vendorVaultFormSchema = z.object({
  vault_code: z.string().min(1, "Wajib diisi").max(50, "Maksimal 50 karakter"),
  category: z.enum(VENDOR_VAULT_CATEGORIES, { message: "Wajib dipilih" }),
  currency_code: z
    .string()
    .min(1, "Wajib diisi")
    .max(3, "Wajib 3 huruf")
    .regex(/^[A-Za-z]*$/, "Hanya huruf"),
  min_capacity_amount: z.string(),
  max_capacity_amount: z.string(),
  latitude: z.string(),
  longitude: z.string(),
  operating_hours: z.string().max(200, "Maksimal 200 karakter"),
  location_id: z.string().max(20, "Maksimal 20 karakter"),
});

export type VendorVaultFormValues = z.infer<typeof vendorVaultFormSchema>;
