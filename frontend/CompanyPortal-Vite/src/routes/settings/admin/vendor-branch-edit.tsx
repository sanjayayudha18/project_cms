import { Forbidden } from "@/components/pages/Forbidden";
import { VendorBranchEditPage } from "@/features/admin-vendors";
import { createRoute } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../../_protected";

export const adminVendorBranchEditRoute = createRoute({
  path: "/settings/admin/vendors/$vendorId/branches/$branchId/edit",
  getParentRoute: () => protectedRoute,
  beforeLoad: requireRoles(["ADMIN", "ADMIN_PARAM"]),
  component: AdminVendorBranchEditRouteComponent,
});

function AdminVendorBranchEditRouteComponent() {
  const context = adminVendorBranchEditRoute.useRouteContext() as { forbidden?: boolean };
  const { vendorId, branchId } = adminVendorBranchEditRoute.useParams();
  if (context.forbidden) return <Forbidden />;
  return <VendorBranchEditPage vendorId={Number(vendorId)} branchId={Number(branchId)} />;
}
