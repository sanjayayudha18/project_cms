import { Forbidden } from "@/components/pages/Forbidden";
import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../../_protected";

// Page chunk loads on navigation; the loader preloads it so the guard wrapper never suspends (FE-04).
const MasterDataIOPage = lazyRouteComponent(
  () => import("@/features/master-data-io"),
  "MasterDataIOPage",
);

export const adminMasterDataIORoute = createRoute({
  path: "/settings/admin/master-data-io",
  getParentRoute: () => protectedRoute,
  beforeLoad: requireRoles(["ADMIN", "ADMIN_PARAM"]),
  loader: () => MasterDataIOPage.preload?.(),
  component: AdminMasterDataIORouteComponent,
});

function AdminMasterDataIORouteComponent() {
  const context = adminMasterDataIORoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <MasterDataIOPage />;
}
