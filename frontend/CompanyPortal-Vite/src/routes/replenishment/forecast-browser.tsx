import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../_protected";

export const forecastBrowserRoute = createRoute({
  path: "/replenishment/forecast-browser",
  getParentRoute: () => protectedRoute,
  beforeLoad: requireRoles(["ATM-USER", "ATM-SPV", "BRANCH-ATM-USER", "BRANCH-ATM-SPV"]),
  component: lazyRouteComponent(() => import("@/features/vendor-request"), "ForecastBrowser"),
});
