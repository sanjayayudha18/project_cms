/**
 * Zod schema for the admin Vendor create/edit form, mirroring
 * backend/internal/service/vendor_admin.go's Create/Update validation:
 * code + name required, contact_email optional but must be a valid format
 * when present, contact_phone/hq_address are free text.
 */

import { z } from "zod";

export const vendorFormSchema = z.object({
  code: z.string().min(1, "Wajib diisi").max(50, "Maksimal 50 karakter"),
  name: z.string().min(1, "Wajib diisi").max(200, "Maksimal 200 karakter"),
  legal_name: z.string().max(200, "Maksimal 200 karakter"),
  npwp: z.string().max(25, "Maksimal 25 karakter"),
  contact_email: z
    .string()
    .max(200, "Maksimal 200 karakter")
    .refine((v) => v === "" || z.string().email().safeParse(v).success, {
      message: "Format email tidak valid",
    }),
  contact_phone: z.string().max(50, "Maksimal 50 karakter"),
  hq_address: z.string().max(500, "Maksimal 500 karakter"),
});

export type VendorFormValues = z.infer<typeof vendorFormSchema>;
