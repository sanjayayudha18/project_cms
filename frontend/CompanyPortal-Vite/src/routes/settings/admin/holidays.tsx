import { Forbidden } from "@/components/pages/Forbidden";
import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../../_protected";

// Page chunk loads on navigation; the loader preloads it so the guard wrapper never suspends (FE-04).
const AdminHolidaysPage = lazyRouteComponent(
  () => import("@/features/admin-holidays/AdminHolidaysPage"),
  "AdminHolidaysPage",
);

// replenishment-realisasi FR1: ADMIN only (Q14); the API re-checks the role.
export const adminHolidaysRoute = createRoute({
  path: "/settings/admin/holidays",
  getParentRoute: () => protectedRoute,
  beforeLoad: requireRoles(["ADMIN"]),
  loader: () => AdminHolidaysPage.preload?.(),
  component: AdminHolidaysRouteComponent,
});

function AdminHolidaysRouteComponent() {
  const context = adminHolidaysRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <AdminHolidaysPage />;
}
