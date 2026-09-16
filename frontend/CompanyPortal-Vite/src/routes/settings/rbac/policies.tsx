import { Forbidden } from "@/components/pages/Forbidden";
import { RbacPoliciesPage } from "@/features/rbac-settings";
import { createRoute } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../../_protected";

export const rbacPoliciesRoute = createRoute({
  path: "/settings/rbac/policies",
  getParentRoute: () => protectedRoute,
  beforeLoad: requireRoles(["ADMIN", "ADMIN_PARAM", "APPACCESS"]),
  component: RbacPoliciesRouteComponent,
});

function RbacPoliciesRouteComponent() {
  const context = rbacPoliciesRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <RbacPoliciesPage />;
}
