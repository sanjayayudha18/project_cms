import { VendorRequestCreate } from "@/features/vendor-request";
import { useAuthStore } from "@/lib/auth/store";
import { createRoute, redirect } from "@tanstack/react-router";
import { protectedRoute } from "../../_protected";

// Req 12.10: non-maker roles redirect to the dashboard. Not using
// requireRoles() here because its {forbidden:true} return value has no
// consumer anywhere in this codebase (routes using it currently render
// through regardless of role) — this page needs an actual redirect.
const MAKER_ROLES = ["ADMIN", "ATM-USER", "BRANCH-ATM-USER"];

export const vendorRequestNewRoute = createRoute({
  path: "/replenishment/vendor-requests/new",
  getParentRoute: () => protectedRoute,
  beforeLoad: () => {
    const { user } = useAuthStore.getState();
    if (!user || !MAKER_ROLES.includes(user.role)) {
      throw redirect({ to: "/" });
    }
  },
  component: VendorRequestCreate,
});
