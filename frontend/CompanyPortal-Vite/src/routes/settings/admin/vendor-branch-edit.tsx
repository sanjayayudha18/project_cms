import { Forbidden } from "@/components/pages/Forbidden";
import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../../_protected";

// Page chunk loads on navigation; the loader preloads it so the guard wrapper never suspends (FE-04).
const VendorBranchEditPage = lazyRouteComponent(
  () => import("@/features/admin-vendors"),
  "VendorBranchEditPage",
);

export const adminVendorBranchEditRoute = createRoute({
  path: "/settings/admin/vendors/$vendorId/branches/$branchId/edit",
  getParentRoute: () => protectedRoute,
  beforeLoad: requireRoles(["ADMIN", "ADMIN_PARAM"]),
  loader: () => VendorBranchEditPage.preload?.(),
  component: AdminVendorBranchEditRouteComponent,
});

function AdminVendorBranchEditRouteComponent() {
  const context = adminVendorBranchEditRoute.useRouteContext() as { forbidden?: boolean };
  const { vendorId, branchId } = adminVendorBranchEditRoute.useParams();
  if (context.forbidden) return <Forbidden />;
  return <VendorBranchEditPage vendorId={Number(vendorId)} branchId={Number(branchId)} />;
}
