/**
 * Zod schema for the admin Vendor PIC create/edit form, mirroring
 * backend/internal/service/vendor_pic_admin.go's validation: name required,
 * position/phone/email optional, email format checked when present.
 * vendor_branch_id is not part of this form -- it's fixed by the surrounding
 * context (the selected branch, or null for a vendor-wide PIC).
 */

import { z } from "zod";

export const vendorPicFormSchema = z.object({
  name: z.string().min(1, "Wajib diisi").max(200, "Maksimal 200 karakter"),
  position: z.string().max(100, "Maksimal 100 karakter"),
  phone: z.string().max(50, "Maksimal 50 karakter"),
  email: z
    .string()
    .max(200, "Maksimal 200 karakter")
    .refine((v) => v === "" || z.string().email().safeParse(v).success, {
      message: "Format email tidak valid",
    }),
  is_notification_recipient: z.boolean(),
});

export type VendorPicFormValues = z.infer<typeof vendorPicFormSchema>;
