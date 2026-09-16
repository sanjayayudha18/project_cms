import { Forbidden } from "@/components/pages/Forbidden";
import { AdminVendorsPage } from "@/features/admin-vendors";
import { ADMIN_VENDORS_SEARCH_SCHEMA } from "@/features/admin-vendors/useAdminVendorsUrlState";
import { createRoute } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../../_protected";

export const adminVendorsRoute = createRoute({
  path: "/settings/admin/vendors",
  getParentRoute: () => protectedRoute,
  validateSearch: ADMIN_VENDORS_SEARCH_SCHEMA,
  beforeLoad: requireRoles(["ADMIN", "ADMIN_PARAM"]),
  component: AdminVendorsRouteComponent,
});

function AdminVendorsRouteComponent() {
  const context = adminVendorsRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <AdminVendorsPage />;
}
