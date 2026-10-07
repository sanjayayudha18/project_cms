import { Forbidden } from "@/components/pages/Forbidden";
import { VendorDetailPage } from "@/features/admin-vendors";
import { createRoute } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../../_protected";

export const adminVendorDetailRoute = createRoute({
  path: "/settings/admin/vendors/$vendorId",
  getParentRoute: () => protectedRoute,
  beforeLoad: requireRoles(["ADMIN", "ADMIN_PARAM"]),
  component: AdminVendorDetailRouteComponent,
});

function AdminVendorDetailRouteComponent() {
  const context = adminVendorDetailRoute.useRouteContext() as { forbidden?: boolean };
  const { vendorId } = adminVendorDetailRoute.useParams();
  if (context.forbidden) return <Forbidden />;
  return <VendorDetailPage vendorId={Number(vendorId)} />;
}
