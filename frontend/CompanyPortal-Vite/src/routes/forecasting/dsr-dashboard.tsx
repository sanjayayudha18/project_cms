import { DsrDashboard } from "@/features/dsr";
import { createRoute } from "@tanstack/react-router";
import { protectedRoute } from "../_protected";

export const dsrDashboardRoute = createRoute({
  path: "/forecasting/dsr-dashboard",
  getParentRoute: () => protectedRoute,
  component: DsrDashboard,
});
