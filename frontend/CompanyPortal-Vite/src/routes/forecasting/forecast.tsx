import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute } from "../_protected";

export const forecastRoute = createRoute({
  path: "/forecasting/forecast",
  getParentRoute: () => protectedRoute,
  component: lazyRouteComponent(() => import("@/features/forecast"), "ForecastView"),
});
