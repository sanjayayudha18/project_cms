import { Forbidden } from "@/components/pages/Forbidden";
import { AdminRegionsPage } from "@/features/admin-regions";
import { ADMIN_REGIONS_SEARCH_SCHEMA } from "@/features/admin-regions/useAdminRegionsUrlState";
import { createRoute } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../../_protected";

export const adminRegionsRoute = createRoute({
  path: "/settings/admin/regions",
  getParentRoute: () => protectedRoute,
  validateSearch: ADMIN_REGIONS_SEARCH_SCHEMA,
  beforeLoad: requireRoles(["ADMIN", "ADMIN_PARAM"]),
  component: AdminRegionsRouteComponent,
});

function AdminRegionsRouteComponent() {
  const context = adminRegionsRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <AdminRegionsPage />;
}
