import { CashFlowScreen } from "@/features/cash-flow";
import { createRoute } from "@tanstack/react-router";
import { protectedRoute } from "./_protected";

export const cashFlowRoute = createRoute({
  path: "/cash-flow",
  getParentRoute: () => protectedRoute,
  component: CashFlowScreen,
});
