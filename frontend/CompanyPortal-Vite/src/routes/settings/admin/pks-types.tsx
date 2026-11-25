import { Forbidden } from "@/components/pages/Forbidden";
import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requirePermission } from "../../_protected";

// Page chunk loads on navigation; the loader preloads it so the guard wrapper never suspends.
const AdminPksTypesPage = lazyRouteComponent(
  () => import("@/features/admin-pks-types"),
  "AdminPksTypesPage",
);

export const adminPksTypesRoute = createRoute({
  path: "/settings/admin/pks-types",
  getParentRoute: () => protectedRoute,
  beforeLoad: requirePermission("settings.pks-types"),
  loader: () => AdminPksTypesPage.preload?.(),
  component: AdminPksTypesRouteComponent,
});

function AdminPksTypesRouteComponent() {
  const context = adminPksTypesRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <AdminPksTypesPage />;
}
