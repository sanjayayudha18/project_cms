/**
 * Zod schema for the admin User create/edit form, mirroring the server-side
 * rules in backend/internal/auth/user_admin.go's Create/Update: auth_source
 * "local" requires vendor_id + temporary_password, "ldap" forbids both
 * (ErrLdapFieldsNotAllowed). temporary_password is only ever required in
 * create mode -- edit mode never renders/sends it (Req 11.3a).
 */

import { type DbRole, isVendorRole } from "@/lib/auth/store";
import { z } from "zod";

export const ROLE_OPTIONS: DbRole[] = [
  "ADMIN",
  "ADMIN_PARAM",
  "APPACCESS",
  "ATM-USER",
  "ATM-SPV",
  "BRANCH-USER",
  "BRANCH-SPV",
  "BRANCH-ATM-USER",
  "BRANCH-ATM-SPV",
  "VENDOR-USER",
  "VENDOR-ATM-USER",
  "VENDOR-VAULT-USER",
];

/** Penjelasan peran pihak untuk role vendor (vendor-party-scope Req 2.8). */
export const ROLE_LABELS: Partial<Record<string, string>> = {
  "VENDOR-USER": "VENDOR-USER — Pengisian ATM + Penyedia uang",
  "VENDOR-ATM-USER": "VENDOR-ATM-USER — Pengisian ATM saja",
  "VENDOR-VAULT-USER": "VENDOR-VAULT-USER — Penyedia uang saja",
};

export function roleLabel(role: string): string {
  return ROLE_LABELS[role] ?? role;
}

export type UserFormMode = "create" | "edit";

export const userFormBaseSchema = z.object({
  username: z.string().min(1, "Wajib diisi").max(50, "Maksimal 50 karakter"),
  full_name: z.string().min(1, "Wajib diisi").max(200, "Maksimal 200 karakter"),
  email: z.string().min(1, "Wajib diisi").email("Format email tidak valid"),
  role: z.string().min(1, "Wajib dipilih"),
  is_karyawan: z.boolean(),
  auth_source: z.enum(["ldap", "local"]),
  temporary_password: z.string(),
  employee_id: z.string().min(1, "Wajib diisi").max(50, "Maksimal 50 karakter"),
  vendor_id: z.number().int().nullable(),
  vendor_branch_id: z.number().int().nullable(),
  supervisor_id: z.number().int().nullable(),
  approval_level: z.number().int().nullable(),
});

export type UserFormValues = z.infer<typeof userFormBaseSchema>;

export function buildUserFormSchema(mode: UserFormMode) {
  return userFormBaseSchema.superRefine((values, ctx) => {
    const isVendor = isVendorRole(values.role);
    if (isVendor && values.auth_source !== "local") {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["auth_source"],
        message: "Role vendor wajib memakai auth_source local",
      });
    }
    if (!isVendor && values.vendor_branch_id !== null) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["vendor_branch_id"],
        message: "Cabang hanya untuk role vendor",
      });
    }
    if (values.auth_source === "local") {
      if (values.vendor_id === null) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ["vendor_id"],
          message: "Wajib diisi untuk auth_source local",
        });
      }
      if (mode === "create" && values.temporary_password === "") {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ["temporary_password"],
          message: "Wajib diisi untuk auth_source local",
        });
      }
    } else {
      if (values.vendor_id !== null) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ["vendor_id"],
          message: "Tidak boleh diisi untuk auth_source ldap",
        });
      }
      if (mode === "create" && values.temporary_password !== "") {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ["temporary_password"],
          message: "Tidak boleh diisi untuk auth_source ldap",
        });
      }
    }
  });
}
