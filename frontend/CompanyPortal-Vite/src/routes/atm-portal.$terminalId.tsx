import { ATM_PROFILE_SEARCH_SCHEMA, AtmProfileScreen } from "@/features/atm-portal";
import { createRoute } from "@tanstack/react-router";
import { protectedRoute } from "./_protected";

export const atmProfileRoute = createRoute({
  path: "/atm-portal/$terminalId",
  getParentRoute: () => protectedRoute,
  validateSearch: ATM_PROFILE_SEARCH_SCHEMA,
  component: AtmProfileScreen,
});
