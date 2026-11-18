/**
 * Zod schema for the admin Vendor Branch create/edit form, mirroring
 * backend/internal/service/vendor_branch_admin.go's Create/Update validation:
 * branch_code + branch_name required, location_id/region_code optional.
 * region_code (replenish-ticket) is the request-number segment
 * REP-<vendor>-<region_code>-...: picked from the active standard regions
 * (region-standardization Req 3.3); the free-text `region` field is gone (Q5b).
 */

import { z } from "zod";
import { VENDOR_VAULT_CATEGORIES } from "./vendorVaultFormSchema";

export const vendorBranchFormSchema = z.object({
  branch_code: z.string().min(1, "Wajib diisi").max(50, "Maksimal 50 karakter"),
  branch_name: z.string().min(1, "Wajib diisi").max(200, "Maksimal 200 karakter"),
  location_id: z.string().max(20, "Maksimal 20 karakter"),
  region_code: z
    .string()
    .trim()
    .regex(/^([A-Za-z0-9]{2,10})?$/, "Pilih region dari daftar"),
  category: z.enum(VENDOR_VAULT_CATEGORIES, { message: "Wajib dipilih" }),
});

export type VendorBranchFormValues = z.infer<typeof vendorBranchFormSchema>;

/** Form value -> wire value: upper-case, blank = null (backend normalizes the same way). */
export function toRegionCode(value: string): string | null {
  return value.trim().toUpperCase() || null;
}
