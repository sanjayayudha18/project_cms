import { DashboardScreen } from "@/features/dashboard";
import { createRoute } from "@tanstack/react-router";
import { protectedRoute } from "./_protected";

export const indexRoute = createRoute({
  path: "/",
  getParentRoute: () => protectedRoute,
  component: DashboardScreen,
});
