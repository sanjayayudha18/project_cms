import { Forbidden } from "@/components/pages/Forbidden";
import { SettingsHubPage } from "@/features/rbac-settings";
import { createRoute } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "./_protected";

export const settingsRoute = createRoute({
  path: "/settings",
  getParentRoute: () => protectedRoute,
  beforeLoad: requireRoles(["ADMIN", "ADMIN_PARAM", "APPACCESS"]),
  component: SettingsRouteComponent,
});

function SettingsRouteComponent() {
  const context = settingsRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <SettingsHubPage />;
}
