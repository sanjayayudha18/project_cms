import { Forbidden } from "@/components/pages/Forbidden";
import { AdminUsersPage } from "@/features/admin-users";
import { ADMIN_USERS_SEARCH_SCHEMA } from "@/features/admin-users/useAdminUsersUrlState";
import { createRoute } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../../_protected";

export const adminUsersRoute = createRoute({
  path: "/settings/admin/users",
  getParentRoute: () => protectedRoute,
  validateSearch: ADMIN_USERS_SEARCH_SCHEMA,
  beforeLoad: requireRoles(["APPACCESS"]),
  component: AdminUsersRouteComponent,
});

function AdminUsersRouteComponent() {
  const context = adminUsersRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <AdminUsersPage />;
}
