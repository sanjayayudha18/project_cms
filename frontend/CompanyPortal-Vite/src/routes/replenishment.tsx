import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute } from "./_protected";

export const replenishmentRoute = createRoute({
  path: "/replenishment",
  getParentRoute: () => protectedRoute,
  component: lazyRouteComponent(() => import("@/features/replenishment"), "ReplenishmentScreen"),
});
