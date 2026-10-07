import { VENDOR_REQUEST_LIST_SEARCH_SCHEMA, VendorRequestList } from "@/features/vendor-request";
import { createRoute } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../../_protected";

export const vendorRequestListRoute = createRoute({
  path: "/replenishment/vendor-requests",
  getParentRoute: () => protectedRoute,
  validateSearch: VENDOR_REQUEST_LIST_SEARCH_SCHEMA,
  beforeLoad: requireRoles(["ATM-USER", "ATM-SPV", "BRANCH-ATM-USER", "BRANCH-ATM-SPV"]),
  component: VendorRequestList,
});
