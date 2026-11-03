import { Forbidden } from "@/components/pages/Forbidden";
import { ADMIN_ATMS_SEARCH_SCHEMA } from "@/features/admin-atms/useAdminATMsUrlState";
import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../../_protected";

// Page chunk loads on navigation; the loader preloads it so the guard wrapper never suspends (FE-04).
const AdminATMsPage = lazyRouteComponent(() => import("@/features/admin-atms"), "AdminATMsPage");

export const adminATMsRoute = createRoute({
  path: "/settings/admin/atms",
  getParentRoute: () => protectedRoute,
  validateSearch: ADMIN_ATMS_SEARCH_SCHEMA,
  beforeLoad: requireRoles(["ADMIN", "ADMIN_PARAM"]),
  loader: () => AdminATMsPage.preload?.(),
  component: AdminATMsRouteComponent,
});

function AdminATMsRouteComponent() {
  const context = adminATMsRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <AdminATMsPage />;
}
