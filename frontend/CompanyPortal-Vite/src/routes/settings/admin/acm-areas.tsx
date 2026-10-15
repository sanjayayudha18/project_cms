import { Forbidden } from "@/components/pages/Forbidden";
import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../../_protected";

// Page chunk loads on navigation; the loader preloads it so the guard wrapper never suspends (FE-04).
const AdminAcmAreasPage = lazyRouteComponent(
  () => import("@/features/admin-acm-areas/AdminAcmAreasPage"),
  "AdminAcmAreasPage",
);

// Area ACM (cit-acm-plan FR7): ADMIN only, matching the backend route guard.
export const adminAcmAreasRoute = createRoute({
  path: "/settings/admin/acm-areas",
  getParentRoute: () => protectedRoute,
  beforeLoad: requireRoles(["ADMIN"]),
  loader: () => AdminAcmAreasPage.preload?.(),
  component: AdminAcmAreasRouteComponent,
});

function AdminAcmAreasRouteComponent() {
  const context = adminAcmAreasRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <AdminAcmAreasPage />;
}
