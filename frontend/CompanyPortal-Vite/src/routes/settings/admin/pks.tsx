import { Forbidden } from "@/components/pages/Forbidden";
import { ADMIN_PKS_SEARCH_SCHEMA } from "@/features/admin-pks/useAdminPksUrlState";
import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../../_protected";

// Page chunk loads on navigation; the loader preloads it so the guard wrapper never suspends.
const PksListPage = lazyRouteComponent(() => import("@/features/admin-pks"), "PksListPage");

// ADMIN/ADMIN_PARAM pass requireRoles by default; ATM-SPV may read (Req 9.2, 9.3).
export const adminPksRoute = createRoute({
  path: "/settings/admin/pks",
  getParentRoute: () => protectedRoute,
  validateSearch: ADMIN_PKS_SEARCH_SCHEMA,
  beforeLoad: requireRoles(["ATM-SPV"]),
  loader: () => PksListPage.preload?.(),
  component: AdminPksRouteComponent,
});

function AdminPksRouteComponent() {
  const context = adminPksRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <PksListPage />;
}
