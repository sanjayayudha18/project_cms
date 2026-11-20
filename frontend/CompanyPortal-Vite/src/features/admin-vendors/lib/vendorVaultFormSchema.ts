/**
 * Zod schema for the admin Vendor Vault create/edit form.
 * - vault_name is optional on create (server auto-generates <vendorPrefix>_<area>_V<3-digit> when blank).
 * - min_balance and max_balance are decimal strings validated server-side via big.Rat (mapped via applyServerFieldError).
 * - cis_limit_amount is an optional positive numeric(20,2) decimal string (> 0, at most 2 decimals).
 * - escrow_cash and escrow_atm are optional 12-digit numeric strings (^[0-9]{12}$).
 */

import { z } from "zod";

export const VENDOR_VAULT_CATEGORIES = ["ATM", "CASH", "ATM_CASH"] as const;

const optionalCisLimit = z
  .string()
  .trim()
  .refine(
    (v) => v === "" || (/^\d{1,18}(\.\d{1,2})?$/.test(v) && /[1-9]/.test(v)),
    "Harus angka desimal > 0, maksimal 2 desimal",
  );

const optionalEscrowAccount = z
  .string()
  .trim()
  .refine((v) => v === "" || /^\d{12}$/.test(v), "Harus tepat 12 digit angka");

export const vendorVaultFormSchema = z.object({
  vault_name: z.string().trim().max(100, "Maksimal 100 karakter"),
  category: z.enum(VENDOR_VAULT_CATEGORIES, { message: "Wajib dipilih" }),
  currency_code: z
    .string()
    .min(1, "Wajib diisi")
    .max(3, "Wajib 3 huruf")
    .regex(/^[A-Za-z]*$/, "Hanya huruf"),
  alamat: z.string().max(500, "Maksimal 500 karakter"),
  min_balance: z.string(),
  max_balance: z.string(),
  cis_limit_amount: optionalCisLimit,
  escrow_cash: optionalEscrowAccount,
  escrow_atm: optionalEscrowAccount,
  operating_hours: z.string().max(200, "Maksimal 200 karakter"),
});

export type VendorVaultFormValues = z.infer<typeof vendorVaultFormSchema>;
