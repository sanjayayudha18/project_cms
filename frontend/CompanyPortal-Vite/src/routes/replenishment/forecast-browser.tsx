import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requirePermission } from "../_protected";

export const forecastBrowserRoute = createRoute({
  path: "/replenishment/forecast-browser",
  getParentRoute: () => protectedRoute,
  beforeLoad: requirePermission("replenish.forecast-browser"),
  component: lazyRouteComponent(() => import("@/features/vendor-request"), "ForecastBrowser"),
});
