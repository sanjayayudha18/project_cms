import { ReplenishmentScreen } from "@/features/replenishment";
import { createRoute } from "@tanstack/react-router";
import { protectedRoute } from "./_protected";

export const replenishmentRoute = createRoute({
  path: "/replenishment",
  getParentRoute: () => protectedRoute,
  component: ReplenishmentScreen,
});
