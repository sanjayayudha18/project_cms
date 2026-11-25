import { useAuthStore } from "@/lib/auth/store";
import { createRoute, lazyRouteComponent, redirect } from "@tanstack/react-router";
import { protectedRoute, requirePermission } from "../../_protected";

// Req 12.10: non-maker roles redirect to the dashboard (an actual redirect, not the 403 page).
// The permission guard runs first; it throws the 403 for roles without the screen key.
const MAKER_ROLES = ["ADMIN", "ATM-USER", "BRANCH-ATM-USER"];
const requireScreen = requirePermission("replenish.vendor-requests");

export const vendorRequestNewRoute = createRoute({
  path: "/replenishment/vendor-requests/new",
  getParentRoute: () => protectedRoute,
  beforeLoad: async () => {
    await requireScreen();
    const { user } = useAuthStore.getState();
    if (!user || !MAKER_ROLES.includes(user.role)) {
      throw redirect({ to: "/" });
    }
  },
  component: lazyRouteComponent(() => import("@/features/vendor-request"), "VendorRequestCreate"),
});
