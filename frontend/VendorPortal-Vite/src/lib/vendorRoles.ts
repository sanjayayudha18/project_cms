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

/** Which order parties (replenish / vault) an account role may see. */
export function partyRolesOf(role: DbRole): readonly PartyRoleName[] {
  switch (role) {
    case "VENDOR-ATM-USER":
      return ["replenish"];
    case "VENDOR-VAULT-USER":
      return ["vault"];
    default:
      return ["replenish", "vault"];
  }
}

const PARTY_SCOPE_LABEL: Record<PartyRoleName, string> = {
  replenish: "Pengisian ATM",
  vault: "Penyedia uang",
};

/** "Pengisian ATM" | "Penyedia uang" | "Pengisian ATM + Penyedia uang" (Req 8.4). */
export function partyScopeLabel(role: DbRole): string {
  return partyRolesOf(role)
    .map((p) => PARTY_SCOPE_LABEL[p])
    .join(" + ");
}
