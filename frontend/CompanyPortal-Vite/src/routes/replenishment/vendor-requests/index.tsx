import { VENDOR_REQUEST_LIST_SEARCH_SCHEMA } from "@/features/vendor-request/useVendorRequestListUrlState";
import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../../_protected";

export const vendorRequestListRoute = createRoute({
  path: "/replenishment/vendor-requests",
  getParentRoute: () => protectedRoute,
  validateSearch: VENDOR_REQUEST_LIST_SEARCH_SCHEMA,
  beforeLoad: requireRoles(["ATM-USER", "ATM-SPV", "BRANCH-ATM-USER", "BRANCH-ATM-SPV"]),
  component: lazyRouteComponent(() => import("@/features/vendor-request"), "VendorRequestList"),
});
