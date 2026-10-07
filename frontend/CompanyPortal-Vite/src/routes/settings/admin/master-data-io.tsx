import { Forbidden } from "@/components/pages/Forbidden";
import { MasterDataIOPage } from "@/features/master-data-io";
import { createRoute } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../../_protected";

export const adminMasterDataIORoute = createRoute({
  path: "/settings/admin/master-data-io",
  getParentRoute: () => protectedRoute,
  beforeLoad: requireRoles(["ADMIN", "ADMIN_PARAM"]),
  component: AdminMasterDataIORouteComponent,
});

function AdminMasterDataIORouteComponent() {
  const context = adminMasterDataIORoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <MasterDataIOPage />;
}
