import type { DbRole } from "@/lib/types";

// vendor-party-scope: the three roles the VendorPortal accepts. Anything else (internal
// roles) is rejected at login and token refresh.
export const VENDOR_ROLES: readonly DbRole[] = [
  "VENDOR-USER",
  "VENDOR-ATM-USER",
  "VENDOR-VAULT-USER",
];

export type PartyRoleName = "replenish" | "vault";

export function isVendorRole(role: string): role is DbRole {
  return (VENDOR_ROLES as readonly string[]).includes(role);
}

const PARTY_SCOPE_LABEL: Record<PartyRoleName, string> = {
  replenish: "Pengisian ATM",
  vault: "Penyedia uang",
};

/**
 * Label from `party_scope` of /auth/me (role-management-revamp 8.3, Req 7.6) — never from the
 * role name. null/unknown/empty scope → "" (the scope line then shows no party label).
 */
export function partyScopeLabel(scope: readonly string[] | null | undefined): string {
  return (["replenish", "vault"] as const)
    .filter((p) => scope?.includes(p))
    .map((p) => PARTY_SCOPE_LABEL[p])
    .join(" + ");
}
