import { ForecastView } from "@/features/forecast";
import { createRoute } from "@tanstack/react-router";
import { protectedRoute } from "../_protected";

export const forecastRoute = createRoute({
  path: "/forecasting/forecast",
  getParentRoute: () => protectedRoute,
  component: ForecastView,
});
