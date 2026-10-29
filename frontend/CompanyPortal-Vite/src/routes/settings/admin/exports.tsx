import { Forbidden } from "@/components/pages/Forbidden";
import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../../_protected";

// Page chunk loads on navigation; the loader preloads it so the guard wrapper never suspends (FE-04).
const ExportDataPage = lazyRouteComponent(
  () => import("@/features/exports/components/ExportDataPage"),
  "ExportDataPage",
);

export const adminExportsRoute = createRoute({
  path: "/settings/admin/exports",
  getParentRoute: () => protectedRoute,
  beforeLoad: requireRoles(["ADMIN", "ADMIN_PARAM"]),
  loader: () => ExportDataPage.preload?.(),
  component: AdminExportsRouteComponent,
});

function AdminExportsRouteComponent() {
  const context = adminExportsRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <ExportDataPage />;
}
