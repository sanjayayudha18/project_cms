import { Forbidden } from "@/components/pages/Forbidden";
import { ADMIN_REGIONS_SEARCH_SCHEMA } from "@/features/admin-regions/useAdminRegionsUrlState";
import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../../_protected";

// Page chunk loads on navigation; the loader preloads it so the guard wrapper never suspends (FE-04).
const AdminRegionsPage = lazyRouteComponent(
  () => import("@/features/admin-regions"),
  "AdminRegionsPage",
);

export const adminRegionsRoute = createRoute({
  path: "/settings/admin/regions",
  getParentRoute: () => protectedRoute,
  validateSearch: ADMIN_REGIONS_SEARCH_SCHEMA,
  beforeLoad: requireRoles(["ADMIN", "ADMIN_PARAM"]),
  loader: () => AdminRegionsPage.preload?.(),
  component: AdminRegionsRouteComponent,
});

function AdminRegionsRouteComponent() {
  const context = adminRegionsRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <AdminRegionsPage />;
}
