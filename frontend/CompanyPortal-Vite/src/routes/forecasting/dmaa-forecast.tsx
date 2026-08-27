import { DMAA_FORECAST_SEARCH_SCHEMA, DmaaForecastView } from "@/features/dmaa-forecast";
import { createRoute } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../_protected";

export const dmaaForecastRoute = createRoute({
  path: "/forecasting/dmaa-forecast",
  getParentRoute: () => protectedRoute,
  validateSearch: DMAA_FORECAST_SEARCH_SCHEMA,
  beforeLoad: requireRoles(["ATM-USER", "ATM-SPV", "BRANCH-ATM-USER", "BRANCH-ATM-SPV"]),
  component: DmaaForecastView,
});
