import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requirePermission } from "../../_protected";

export const vendorRequestDetailRoute = createRoute({
  path: "/replenishment/vendor-requests/$id",
  getParentRoute: () => protectedRoute,
  beforeLoad: requirePermission("replenish.vendor-requests"),
  component: lazyRouteComponent(() => import("@/features/vendor-request"), "VendorRequestDetail"),
});
