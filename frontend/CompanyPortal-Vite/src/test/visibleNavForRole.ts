import { ALWAYS_ON_KEYS, SCREEN_KEYS } from "@/lib/auth/permissionKeys";
import type { DbRole } from "@/lib/auth/store";
import { type VisibleNav, getVisibleNav } from "@/lib/config/navigation";

/** Stand-in grants for nav tests: admins/APPACCESS hold every key, others the operational screens. */
export function permissionsForRole(role: DbRole | null): ReadonlySet<string> {
  if (!role) return new Set();
  if (role === "ADMIN" || role === "ADMIN_PARAM" || role === "APPACCESS")
    return new Set(SCREEN_KEYS);
  const ops = SCREEN_KEYS.filter(
    (k) =>
      /^(replenish|cash-flow|monitoring\.dsr|forecasting\.dmaa)/.test(k) ||
      (k === "cit.vault-plan" && /^ACM/.test(role)),
  );
  return new Set([...ALWAYS_ON_KEYS, ...ops]);
}

export function getVisibleNavForRole(role: DbRole | null): VisibleNav {
  const keys = permissionsForRole(role);
  return getVisibleNav((k) => keys.has(k));
}
