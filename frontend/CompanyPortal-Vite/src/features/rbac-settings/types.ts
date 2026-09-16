/**
 * RBAC Settings Menu API types (design.md API contract) and form schemas.
 *
 * All endpoints live under /api/v1/admin/approval (backend/internal/handler
 * rbac_list_handler.go). Amounts are serialized as exact decimal strings
 * (money is numeric, never float) -- never coerce them to `number` for
 * display or storage, only for the client-side range comparison below.
 */

import { z } from "zod";

// ─── Entities ──────────────────────────────────────────────────────────────

export interface RbacUser {
  id: number;
  supervisor_id: number | null;
  approval_level: number | null;
  role: string;
  auth_source: string;
}

export interface RbacUsersResponse {
  users: RbacUser[];
}

export interface RbacDelegation {
  id: number;
  from_user_id: number;
  to_user_id: number;
  /** RFC3339 timestamp */
  start_at: string;
  /** RFC3339 timestamp */
  end_at: string;
  reason: string | null;
}

export interface RbacDelegationsResponse {
  delegations: RbacDelegation[];
}

export interface RbacLeave {
  id: number;
  user_id: number;
  /** RFC3339 timestamp */
  start_at: string;
  /** RFC3339 timestamp */
  end_at: string;
  reason: string | null;
}

export interface RbacLeavesResponse {
  leaves: RbacLeave[];
}

export interface RbacPolicy {
  id: number;
  document_type: string;
  /** Exact decimal string, e.g. "1000000.00" */
  min_amount: string;
  /** Exact decimal string */
  max_amount: string;
  required_level: number;
}

export interface RbacPoliciesResponse {
  policies: RbacPolicy[];
}

// ─── Badge variants ──────────────────────────────────────────────────────────

export type BadgeVariant = "success" | "warning" | "danger" | "info" | "neutral";

/**
 * role -> badge variant. All 10 seeded roles (backend/migrations/002, 027).
 * Icon pairing lives in RoleBadge.tsx (Task 8.2) so status is never color-only.
 */
export const ROLE_BADGE_VARIANT: Record<string, BadgeVariant> = {
  ADMIN: "info",
  ADMIN_PARAM: "info",
  APPACCESS: "info",
  "ATM-USER": "neutral",
  "ATM-SPV": "neutral",
  "BRANCH-USER": "neutral",
  "BRANCH-SPV": "neutral",
  "BRANCH-ATM-USER": "neutral",
  "BRANCH-ATM-SPV": "neutral",
  "VENDOR-USER": "warning",
};

// ─── Form schemas ──────────────────────────────────────────────────────────

/**
 * No z.coerce here: an empty input must become null (no supervisor / no
 * level), not 0 -- z.coerce.number() on "" produces 0, which then fails
 * .positive() with a confusing error instead of being accepted as null. The
 * form converts "" -> null via RHF's setValueAs before this schema ever sees
 * it (RbacUsersPage.tsx), which also keeps the field's input and output
 * types identical so useForm/zodResolver don't fight over the field type.
 */
const nullablePositiveInt = z.number().int().positive().nullable();

/**
 * Hierarchy form schema factory: the self-supervisor guard needs the target
 * user's own id, which isn't known until the form is opened for a specific
 * row, so this mirrors the server check (SetHierarchy, admin_approval_handler.go)
 * as a per-user schema rather than a static one.
 */
export function createHierarchySchema(userId: number) {
  return z
    .object({
      supervisor_id: nullablePositiveInt,
      approval_level: nullablePositiveInt,
    })
    .refine((data) => data.supervisor_id === null || data.supervisor_id !== userId, {
      message: "supervisor_id tidak boleh sama dengan user itu sendiri",
      path: ["supervisor_id"],
    });
}

export type HierarchyFormValues = z.infer<ReturnType<typeof createHierarchySchema>>;

const rfc3339Field = (label: string) => z.string().min(1, `${label} wajib diisi`);

export const delegationSchema = z
  .object({
    from_user_id: z.coerce.number().int().positive("from_user_id wajib diisi"),
    to_user_id: z.coerce.number().int().positive("to_user_id wajib diisi"),
    start_at: rfc3339Field("start_at"),
    end_at: rfc3339Field("end_at"),
    reason: z.string().optional(),
  })
  .refine((data) => new Date(data.start_at) < new Date(data.end_at), {
    message: "start_at harus sebelum end_at",
    path: ["end_at"],
  });

export type DelegationFormValues = z.infer<typeof delegationSchema>;

export const leaveSchema = z
  .object({
    user_id: z.coerce.number().int().positive("user_id wajib diisi"),
    start_at: rfc3339Field("start_at"),
    end_at: rfc3339Field("end_at"),
    reason: z.string().optional(),
  })
  .refine((data) => new Date(data.start_at) < new Date(data.end_at), {
    message: "start_at harus sebelum end_at",
    path: ["end_at"],
  });

export type LeaveFormValues = z.infer<typeof leaveSchema>;

const decimalAmount = z
  .string()
  .min(1, "wajib diisi")
  .refine((v) => !Number.isNaN(Number(v)), { message: "harus berupa angka" });

/**
 * min_amount < max_amount strictly (not "<="): the approval_policies table's
 * amount-range CHECK constraint (backend/migrations/022_approval_policies.sql)
 * rejects an equal range, and the handler validates the same way (Task 3.2).
 * This client-side check is UX-only -- the backend is the source of truth for
 * exact-decimal comparison (it uses arbitrary-precision math, not `Number`).
 */
export const policySchema = z
  .object({
    document_type: z.string().min(1, "document_type wajib diisi"),
    min_amount: decimalAmount,
    max_amount: decimalAmount,
    required_level: z.coerce.number().int().positive("required_level harus lebih besar dari 0"),
  })
  .refine((data) => Number(data.min_amount) < Number(data.max_amount), {
    message: "min_amount harus lebih kecil dari max_amount",
    path: ["max_amount"],
  });

export type PolicyFormValues = z.infer<typeof policySchema>;
