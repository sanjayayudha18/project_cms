import { Forbidden } from "@/components/pages/Forbidden";
import { AdminATMsPage } from "@/features/admin-atms";
import { ADMIN_ATMS_SEARCH_SCHEMA } from "@/features/admin-atms/useAdminATMsUrlState";
import { createRoute } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../../_protected";

export const adminATMsRoute = createRoute({
  path: "/settings/admin/atms",
  getParentRoute: () => protectedRoute,
  validateSearch: ADMIN_ATMS_SEARCH_SCHEMA,
  beforeLoad: requireRoles(["ADMIN", "ADMIN_PARAM"]),
  component: AdminATMsRouteComponent,
});

function AdminATMsRouteComponent() {
  const context = adminATMsRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <AdminATMsPage />;
}
