import { Forbidden } from "@/components/pages/Forbidden";
import { RbacUsersPage } from "@/features/rbac-settings";
import { createRoute } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../../_protected";

export const rbacUsersRoute = createRoute({
  path: "/settings/rbac/users",
  getParentRoute: () => protectedRoute,
  beforeLoad: requireRoles(["ADMIN", "ADMIN_PARAM", "APPACCESS"]),
  component: RbacUsersRouteComponent,
});

function RbacUsersRouteComponent() {
  const context = rbacUsersRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <RbacUsersPage />;
}
