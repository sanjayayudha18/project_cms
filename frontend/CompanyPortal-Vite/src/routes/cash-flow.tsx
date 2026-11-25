import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requirePermission } from "./_protected";

export const cashFlowRoute = createRoute({
  path: "/cash-flow",
  getParentRoute: () => protectedRoute,
  beforeLoad: requirePermission("cash-flow"),
  component: lazyRouteComponent(() => import("@/features/cash-flow"), "CashFlowScreen"),
});
