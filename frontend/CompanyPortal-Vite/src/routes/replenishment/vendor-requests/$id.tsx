import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../../_protected";

export const vendorRequestDetailRoute = createRoute({
  path: "/replenishment/vendor-requests/$id",
  getParentRoute: () => protectedRoute,
  beforeLoad: requireRoles(["ATM-USER", "ATM-SPV", "BRANCH-ATM-USER", "BRANCH-ATM-SPV"]),
  component: lazyRouteComponent(() => import("@/features/vendor-request"), "VendorRequestDetail"),
});
