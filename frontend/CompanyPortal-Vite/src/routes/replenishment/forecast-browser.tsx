import { ForecastBrowser } from "@/features/vendor-request";
import { createRoute } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../_protected";

export const forecastBrowserRoute = createRoute({
  path: "/replenishment/forecast-browser",
  getParentRoute: () => protectedRoute,
  beforeLoad: requireRoles(["ATM-USER", "ATM-SPV", "BRANCH-ATM-USER", "BRANCH-ATM-SPV"]),
  component: ForecastBrowser,
});
