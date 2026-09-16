import { Forbidden } from "@/components/pages/Forbidden";
import { RbacDelegationsPage } from "@/features/rbac-settings";
import { createRoute } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../../_protected";

export const rbacDelegationsRoute = createRoute({
  path: "/settings/rbac/delegations",
  getParentRoute: () => protectedRoute,
  beforeLoad: requireRoles(["ADMIN", "ADMIN_PARAM", "APPACCESS"]),
  component: RbacDelegationsRouteComponent,
});

function RbacDelegationsRouteComponent() {
  const context = rbacDelegationsRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <RbacDelegationsPage />;
}
