import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute } from "./_protected";

export const cashFlowRoute = createRoute({
  path: "/cash-flow",
  getParentRoute: () => protectedRoute,
  component: lazyRouteComponent(() => import("@/features/cash-flow"), "CashFlowScreen"),
});
