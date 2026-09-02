import { createRoute, redirect } from "@tanstack/react-router";
import { rootRoute } from "./__root";

/** `/dashboard` redirects to /orders (Requirement 2.4). */
export const dashboardRoute = createRoute({
  path: "/dashboard",
  getParentRoute: () => rootRoute,
  beforeLoad: () => {
    throw redirect({ to: "/orders" });
  },
});
