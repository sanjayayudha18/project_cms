import { Forbidden } from "@/components/pages/Forbidden";
import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../../_protected";

// Page chunk loads on navigation; the loader preloads it so the guard wrapper never suspends (FE-04).
const VendorDetailPage = lazyRouteComponent(
  () => import("@/features/admin-vendors"),
  "VendorDetailPage",
);

export const adminVendorDetailRoute = createRoute({
  path: "/settings/admin/vendors/$vendorId",
  getParentRoute: () => protectedRoute,
  beforeLoad: requireRoles(["ADMIN", "ADMIN_PARAM"]),
  loader: () => VendorDetailPage.preload?.(),
  component: AdminVendorDetailRouteComponent,
});

function AdminVendorDetailRouteComponent() {
  const context = adminVendorDetailRoute.useRouteContext() as { forbidden?: boolean };
  const { vendorId } = adminVendorDetailRoute.useParams();
  if (context.forbidden) return <Forbidden />;
  return <VendorDetailPage vendorId={Number(vendorId)} />;
}
