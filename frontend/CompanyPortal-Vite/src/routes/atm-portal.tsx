import { ATM_PORTAL_SEARCH_SCHEMA, AtmPortalScreen } from "@/features/atm-portal";
import { createRoute } from "@tanstack/react-router";
import { protectedRoute } from "./_protected";

export const atmPortalRoute = createRoute({
  path: "/atm-portal",
  getParentRoute: () => protectedRoute,
  validateSearch: ATM_PORTAL_SEARCH_SCHEMA,
  component: AtmPortalScreen,
});
