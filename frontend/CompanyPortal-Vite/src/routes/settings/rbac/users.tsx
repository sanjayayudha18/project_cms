import { Forbidden } from "@/components/pages/Forbidden";
import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requirePermission } from "../../_protected";

// Page chunk loads on navigation; the loader preloads it so the guard wrapper never suspends (FE-04).
const RbacUsersPage = lazyRouteComponent(() => import("@/features/rbac-settings"), "RbacUsersPage");

export const rbacUsersRoute = createRoute({
  path: "/settings/rbac/users",
  getParentRoute: () => protectedRoute,
  beforeLoad: requirePermission("settings.rbac-users"),
  loader: () => RbacUsersPage.preload?.(),
  component: RbacUsersRouteComponent,
});

function RbacUsersRouteComponent() {
  const context = rbacUsersRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <RbacUsersPage />;
}
