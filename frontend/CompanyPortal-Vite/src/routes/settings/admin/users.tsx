import { Forbidden } from "@/components/pages/Forbidden";
import { ADMIN_USERS_SEARCH_SCHEMA } from "@/features/admin-users/useAdminUsersUrlState";
import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../../_protected";

// Page chunk loads on navigation; the loader preloads it so the guard wrapper never suspends (FE-04).
const AdminUsersPage = lazyRouteComponent(() => import("@/features/admin-users"), "AdminUsersPage");

export const adminUsersRoute = createRoute({
  path: "/settings/admin/users",
  getParentRoute: () => protectedRoute,
  validateSearch: ADMIN_USERS_SEARCH_SCHEMA,
  beforeLoad: requireRoles(["APPACCESS"]),
  loader: () => AdminUsersPage.preload?.(),
  component: AdminUsersRouteComponent,
});

function AdminUsersRouteComponent() {
  const context = adminUsersRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <AdminUsersPage />;
}
