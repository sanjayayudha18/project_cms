import { DMAA_FORECAST_SEARCH_SCHEMA } from "@/features/dmaa-forecast/useDmaaForecastUrlState";
import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../_protected";

export const dmaaForecastRoute = createRoute({
  path: "/forecasting/dmaa-forecast",
  getParentRoute: () => protectedRoute,
  validateSearch: DMAA_FORECAST_SEARCH_SCHEMA,
  beforeLoad: requireRoles(["ATM-USER", "ATM-SPV", "BRANCH-ATM-USER", "BRANCH-ATM-SPV"]),
  component: lazyRouteComponent(() => import("@/features/dmaa-forecast"), "DmaaForecastView"),
});
