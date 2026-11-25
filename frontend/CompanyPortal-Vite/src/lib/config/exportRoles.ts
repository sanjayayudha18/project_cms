/** Union of the DSR-late and replenishment report viewers; no ADMIN/ADMIN_PARAM bypass (requirePermission). */
export const EXPORTS_PAGE_ROLES = [
  "ADMIN",
  "ATM-USER",
  "ATM-SPV",
  "BRANCH-ATM-USER",
  "BRANCH-ATM-SPV",
] as const;
