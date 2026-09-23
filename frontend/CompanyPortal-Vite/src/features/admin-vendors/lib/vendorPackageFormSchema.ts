/**
 * Zod schema for the admin Vendor Package create form, mirroring
 * backend/internal/service/vendor_package_admin.go's Create validation.
 * Since migration 010 this is the only editable field (vendor_branch_id is
 * fixed by the branch/PT context the dialog is opened from, not user-typed
 * here) -- there is no Update, both fields are immutable after create.
 */

import { z } from "zod";

export const vendorPackageFormSchema = z.object({
  code: z.string().min(1, "Wajib diisi").max(50, "Maksimal 50 karakter"),
});

export type VendorPackageFormValues = z.infer<typeof vendorPackageFormSchema>;
