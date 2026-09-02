import { NotFound } from "@/components/NotFound";
import { createRoute } from "@tanstack/react-router";
import { rootRoute } from "./__root";

/** Internal-only route masked as NotFound (Requirement 2.5). */
export const forecastingRoute = createRoute({
  path: "/forecasting",
  getParentRoute: () => rootRoute,
  component: NotFound,
});
