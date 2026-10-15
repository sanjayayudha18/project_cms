import { Forbidden } from "@/components/pages/Forbidden";
import { AdminHolidaysPage } from "@/features/admin-holidays/AdminHolidaysPage";
import { createRoute } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../../_protected";

// replenishment-realisasi FR1: ADMIN only (Q14); the API re-checks the role.
export const adminHolidaysRoute = createRoute({
  path: "/settings/admin/holidays",
  getParentRoute: () => protectedRoute,
  beforeLoad: requireRoles(["ADMIN"]),
  component: AdminHolidaysRouteComponent,
});

function AdminHolidaysRouteComponent() {
  const context = adminHolidaysRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <AdminHolidaysPage />;
}
