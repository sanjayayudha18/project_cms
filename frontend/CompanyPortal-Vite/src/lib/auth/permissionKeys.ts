// Screen keys of the permission catalog (menu_features.key). Must match backend
// `rolemgmt/keys.go` and the migration seed; a test compares SCREEN_KEYS with
// __fixtures__/catalog-keys.json (spec permission-catalog-coverage, task 5.3/6.4).

export const SCREEN_KEYS = [
  // existing menus
  "dashboard",
  "cash-flow",
  "monitoring",
  "forecasting",
  "replenish",
  "invoice",
  "cash-count",
  "settings",
  "cit",
  // existing features
  "settings.roles",
  "settings.roles.create",
  "settings.roles.edit",
  "settings.acm-areas",
  "settings.pks",
  "cash-count.parameters",
  "cash-count.pic-mapping",
  "cit.vault-plan",
  // new menus
  "approvals",
  "exports",
  // new features
  "replenish.forecast-browser",
  "replenish.vendor-requests",
  "replenish.schedule",
  "monitoring.atm-portal",
  "monitoring.dsr-late-report",
  "monitoring.eod-monitoring",
  "monitoring.audit-logs",
  "forecasting.dmaa-forecast",
  "forecasting.holidays",
  "settings.users",
  "settings.vendors",
  "settings.atms",
  "settings.regions",
  "settings.pks-types",
  "settings.master-data-io",
  "settings.rbac-users",
  "settings.rbac-policies",
  "settings.rbac-delegations",
  "settings.rbac-leaves",
] as const;

export type ScreenKey = (typeof SCREEN_KEYS)[number];

/** Always allowed for every internal role; cannot be revoked (D7). */
export const ALWAYS_ON_KEYS: readonly ScreenKey[] = ["dashboard", "approvals"];

/** Guarded by static APPACCESS/ADMIN roles; not editable via the catalog (D8). */
export const LOCKED_KEYS: readonly ScreenKey[] = [
  "settings.roles",
  "settings.roles.create",
  "settings.roles.edit",
  "settings.users",
];
