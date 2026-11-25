import type { ScreenKey } from "@/lib/auth/permissionKeys";
import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requirePermission } from "./_protected";

// Every key a hub card is gated by (keep in sync with SettingsHubPage cards).
export const SETTINGS_HUB_KEYS: readonly ScreenKey[] = [
  "settings.rbac-users",
  "settings.users",
  "settings.roles",
  "settings.rbac-policies",
  "settings.rbac-delegations",
  "settings.rbac-leaves",
  "settings.vendors",
  "settings.pks",
  "settings.pks-types",
  "settings.atms",
  "settings.regions",
  "settings.acm-areas",
  "forecasting.holidays",
  "settings.master-data-io",
  "exports",
];

// Page chunk loads on navigation; the loader preloads it so the guard wrapper never suspends (FE-04).
const SettingsHubPage = lazyRouteComponent(
  () => import("@/features/rbac-settings"),
  "SettingsHubPage",
);

export const settingsRoute = createRoute({
  path: "/settings",
  getParentRoute: () => protectedRoute,
  beforeLoad: requirePermission(SETTINGS_HUB_KEYS),
  loader: () => SettingsHubPage.preload?.(),
  component: SettingsHubPage,
});
