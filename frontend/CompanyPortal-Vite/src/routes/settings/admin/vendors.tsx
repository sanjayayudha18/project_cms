import { Forbidden } from "@/components/pages/Forbidden";
import { ADMIN_VENDORS_SEARCH_SCHEMA } from "@/features/admin-vendors/useAdminVendorsUrlState";
import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../../_protected";

// Page chunk loads on navigation; the loader preloads it so the guard wrapper never suspends (FE-04).
const AdminVendorsPage = lazyRouteComponent(
  () => import("@/features/admin-vendors"),
  "AdminVendorsPage",
);

export const adminVendorsRoute = createRoute({
  path: "/settings/admin/vendors",
  getParentRoute: () => protectedRoute,
  validateSearch: ADMIN_VENDORS_SEARCH_SCHEMA,
  beforeLoad: requireRoles(["ADMIN", "ADMIN_PARAM"]),
  loader: () => AdminVendorsPage.preload?.(),
  component: AdminVendorsRouteComponent,
});

function AdminVendorsRouteComponent() {
  const context = adminVendorsRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <AdminVendorsPage />;
}
