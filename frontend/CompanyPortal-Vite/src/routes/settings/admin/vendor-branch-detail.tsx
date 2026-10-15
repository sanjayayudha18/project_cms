import { Forbidden } from "@/components/pages/Forbidden";
import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../../_protected";

// Page chunk loads on navigation; the loader preloads it so the guard wrapper never suspends (FE-04).
const VendorBranchDetailPage = lazyRouteComponent(
  () => import("@/features/admin-vendors"),
  "VendorBranchDetailPage",
);

export const adminVendorBranchDetailRoute = createRoute({
  path: "/settings/admin/vendors/$vendorId/branches/$branchId",
  getParentRoute: () => protectedRoute,
  beforeLoad: requireRoles(["ADMIN", "ADMIN_PARAM"]),
  loader: () => VendorBranchDetailPage.preload?.(),
  component: AdminVendorBranchDetailRouteComponent,
});

function AdminVendorBranchDetailRouteComponent() {
  const context = adminVendorBranchDetailRoute.useRouteContext() as { forbidden?: boolean };
  const { vendorId, branchId } = adminVendorBranchDetailRoute.useParams();
  if (context.forbidden) return <Forbidden />;
  return <VendorBranchDetailPage vendorId={Number(vendorId)} branchId={Number(branchId)} />;
}
