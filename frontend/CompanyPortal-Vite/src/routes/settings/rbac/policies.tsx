import { Forbidden } from "@/components/pages/Forbidden";
import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../../_protected";

// Page chunk loads on navigation; the loader preloads it so the guard wrapper never suspends (FE-04).
const RbacPoliciesPage = lazyRouteComponent(
  () => import("@/features/rbac-settings"),
  "RbacPoliciesPage",
);

export const rbacPoliciesRoute = createRoute({
  path: "/settings/rbac/policies",
  getParentRoute: () => protectedRoute,
  beforeLoad: requireRoles(["ADMIN", "ADMIN_PARAM", "APPACCESS"]),
  loader: () => RbacPoliciesPage.preload?.(),
  component: RbacPoliciesRouteComponent,
});

function RbacPoliciesRouteComponent() {
  const context = rbacPoliciesRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <RbacPoliciesPage />;
}
