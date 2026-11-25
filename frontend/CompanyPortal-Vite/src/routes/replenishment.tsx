import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requirePermission } from "./_protected";

export const replenishmentRoute = createRoute({
  path: "/replenishment",
  getParentRoute: () => protectedRoute,
  beforeLoad: requirePermission("replenish.schedule"),
  component: lazyRouteComponent(() => import("@/features/replenishment"), "ReplenishmentScreen"),
});
