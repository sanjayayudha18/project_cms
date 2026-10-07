/**
 * Zod schema for the admin ATM create/edit form, mirroring
 * backend/internal/service/atm_admin.go's validateATMEditableFields:
 * required text fields, priority_class enum, and each monetary amount
 * parses as a non-negative exact decimal -- validated here via a regex
 * (numeric(20,2) shape) rather than Number()/parseFloat, so the check never
 * routes the value through a lossy float path. terminal_id is required only
 * on create; the dialog disables (and omits from the update payload) the
 * field in edit mode, so its own immutability is enforced by never being
 * sent, not by a schema rule here.
 */

import { z } from "zod";

// numeric(20,2): up to 18 integer digits, optional 1-2 decimal digits, non-negative.
const DECIMAL_PATTERN = /^\d{1,18}(\.\d{1,2})?$/;

function optionalDecimalField() {
  return z.string().refine((v) => v === "" || DECIMAL_PATTERN.test(v), {
    message: "Harus berupa angka non-negatif dengan maksimal 2 desimal",
  });
}

export const PRIORITY_CLASS_VALUES = ["VIP", "Non VIP", "Industri"] as const;

export const atmFormSchema = z.object({
  terminal_id: z.string().min(1, "Wajib diisi").max(50, "Maksimal 50 karakter"),
  location_id: z.number({ invalid_type_error: "Wajib dipilih" }).int().positive("Wajib dipilih"),
  machine_type: z.string().min(1, "Wajib diisi").max(50, "Maksimal 50 karakter"),
  brand: z.string().min(1, "Wajib diisi").max(50, "Maksimal 50 karakter"),
  model: z.string().min(1, "Wajib diisi").max(100, "Maksimal 100 karakter"),
  operation_hours: z.string().min(1, "Wajib diisi").max(50, "Maksimal 50 karakter"),
  deployment_type: z.string().min(1, "Wajib diisi").max(50, "Maksimal 50 karakter"),
  capacity_amount: optionalDecimalField(),
  low_threshold_amount: optionalDecimalField(),
  critical_threshold_amount: optionalDecimalField(),
  blacklisted: z.boolean(),
  escrow_account: z.string().max(50, "Maksimal 50 karakter"),
  priority_class: z.enum([...PRIORITY_CLASS_VALUES, ""]),
});

export type ATMFormValues = z.infer<typeof atmFormSchema>;
