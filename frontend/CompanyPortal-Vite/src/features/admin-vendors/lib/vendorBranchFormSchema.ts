/**
 * Zod schema for the admin Vendor Branch create/edit form, mirroring
 * backend/internal/service/vendor_branch_admin.go's Create/Update validation:
 * branch_code + branch_name required, location_id/region optional.
 */

import { z } from "zod";
import { VENDOR_VAULT_CATEGORIES } from "./vendorVaultFormSchema";

export const vendorBranchFormSchema = z.object({
  branch_code: z.string().min(1, "Wajib diisi").max(50, "Maksimal 50 karakter"),
  branch_name: z.string().min(1, "Wajib diisi").max(200, "Maksimal 200 karakter"),
  location_id: z.string().max(20, "Maksimal 20 karakter"),
  region: z.string().max(100, "Maksimal 100 karakter"),
  category: z.enum(VENDOR_VAULT_CATEGORIES, { message: "Wajib dipilih" }),
});

export type VendorBranchFormValues = z.infer<typeof vendorBranchFormSchema>;
