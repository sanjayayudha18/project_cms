import { Forbidden } from "@/components/pages/Forbidden";
import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../../_protected";

// Page chunk loads on navigation; the loader preloads it so the guard wrapper never suspends (FE-04).
const RbacLeavesPage = lazyRouteComponent(
  () => import("@/features/rbac-settings"),
  "RbacLeavesPage",
);

export const rbacLeavesRoute = createRoute({
  path: "/settings/rbac/leaves",
  getParentRoute: () => protectedRoute,
  beforeLoad: requireRoles(["ADMIN", "ADMIN_PARAM", "APPACCESS"]),
  loader: () => RbacLeavesPage.preload?.(),
  component: RbacLeavesRouteComponent,
});

function RbacLeavesRouteComponent() {
  const context = rbacLeavesRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <RbacLeavesPage />;
}
