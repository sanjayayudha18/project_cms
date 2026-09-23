import { Forbidden } from "@/components/pages/Forbidden";
import { VendorBranchDetailPage } from "@/features/admin-vendors";
import { createRoute } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../../_protected";

export const adminVendorBranchDetailRoute = createRoute({
  path: "/settings/admin/vendors/$vendorId/branches/$branchId",
  getParentRoute: () => protectedRoute,
  beforeLoad: requireRoles(["ADMIN", "ADMIN_PARAM"]),
  component: AdminVendorBranchDetailRouteComponent,
});

function AdminVendorBranchDetailRouteComponent() {
  const context = adminVendorBranchDetailRoute.useRouteContext() as { forbidden?: boolean };
  const { vendorId, branchId } = adminVendorBranchDetailRoute.useParams();
  if (context.forbidden) return <Forbidden />;
  return <VendorBranchDetailPage vendorId={Number(vendorId)} branchId={Number(branchId)} />;
}
