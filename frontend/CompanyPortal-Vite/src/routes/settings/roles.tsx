import { Forbidden } from "@/components/pages/Forbidden";
import { RoleManagementPage } from "@/features/role-management";
import { createRoute } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../_protected";

/**
 * /settings/roles — Role Management (Req 4.5, 7.1). Backend restricts this
 * to APPACCESS/ADMIN only (RequireRoles("APPACCESS", "ADMIN"),
 * cmd/api/main.go). requireRoles()'s ADMIN/ADMIN_PARAM bypass is shared
 * across every settings sub-route (see _protected.tsx), so ADMIN_PARAM will
 * still see this route client-side even though the backend rejects its API
 * calls with 403 — same pre-existing gap every other settings/admin/* route
 * already has, not something this route can fix on its own.
 */
export const rolesRoute = createRoute({
  path: "/settings/roles",
  getParentRoute: () => protectedRoute,
  beforeLoad: requireRoles(["APPACCESS"]),
  component: RolesRouteComponent,
});

function RolesRouteComponent() {
  const context = rolesRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <RoleManagementPage />;
}
