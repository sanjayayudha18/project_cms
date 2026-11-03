import { Forbidden } from "@/components/pages/Forbidden";
import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../../_protected";

// Page chunk loads on navigation; the loader preloads it so the guard wrapper never suspends (FE-04).
const RbacDelegationsPage = lazyRouteComponent(
  () => import("@/features/rbac-settings"),
  "RbacDelegationsPage",
);

export const rbacDelegationsRoute = createRoute({
  path: "/settings/rbac/delegations",
  getParentRoute: () => protectedRoute,
  beforeLoad: requireRoles(["ADMIN", "ADMIN_PARAM", "APPACCESS"]),
  loader: () => RbacDelegationsPage.preload?.(),
  component: RbacDelegationsRouteComponent,
});

function RbacDelegationsRouteComponent() {
  const context = rbacDelegationsRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <RbacDelegationsPage />;
}
