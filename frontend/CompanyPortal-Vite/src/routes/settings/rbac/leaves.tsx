import { Forbidden } from "@/components/pages/Forbidden";
import { RbacLeavesPage } from "@/features/rbac-settings";
import { createRoute } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../../_protected";

export const rbacLeavesRoute = createRoute({
  path: "/settings/rbac/leaves",
  getParentRoute: () => protectedRoute,
  beforeLoad: requireRoles(["ADMIN", "ADMIN_PARAM", "APPACCESS"]),
  component: RbacLeavesRouteComponent,
});

function RbacLeavesRouteComponent() {
  const context = rbacLeavesRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <RbacLeavesPage />;
}
