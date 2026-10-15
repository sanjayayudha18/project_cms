import { Forbidden } from "@/components/pages/Forbidden";
import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "./_protected";

// Page chunk loads on navigation; the loader preloads it so the guard wrapper never suspends (FE-04).
const SettingsHubPage = lazyRouteComponent(
  () => import("@/features/rbac-settings"),
  "SettingsHubPage",
);

export const settingsRoute = createRoute({
  path: "/settings",
  getParentRoute: () => protectedRoute,
  beforeLoad: requireRoles(["ADMIN", "ADMIN_PARAM", "APPACCESS"]),
  loader: () => SettingsHubPage.preload?.(),
  component: SettingsRouteComponent,
});

function SettingsRouteComponent() {
  const context = settingsRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <SettingsHubPage />;
}
