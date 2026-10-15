import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute } from "./_protected";

export const indexRoute = createRoute({
  path: "/",
  getParentRoute: () => protectedRoute,
  component: lazyRouteComponent(() => import("@/features/dashboard"), "DashboardScreen"),
});
