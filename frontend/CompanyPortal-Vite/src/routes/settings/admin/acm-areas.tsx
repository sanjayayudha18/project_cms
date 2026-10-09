import { Forbidden } from "@/components/pages/Forbidden";
import { AdminAcmAreasPage } from "@/features/admin-acm-areas/AdminAcmAreasPage";
import { createRoute } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../../_protected";

// Area ACM (cit-acm-plan FR7): ADMIN only, matching the backend route guard.
export const adminAcmAreasRoute = createRoute({
  path: "/settings/admin/acm-areas",
  getParentRoute: () => protectedRoute,
  beforeLoad: requireRoles(["ADMIN"]),
  component: AdminAcmAreasRouteComponent,
});

function AdminAcmAreasRouteComponent() {
  const context = adminAcmAreasRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <AdminAcmAreasPage />;
}
