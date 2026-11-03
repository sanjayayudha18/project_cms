import { ATM_PORTAL_SEARCH_SCHEMA } from "@/features/atm-portal/useAtmPortalUrlState";
import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute } from "./_protected";

export const atmPortalRoute = createRoute({
  path: "/atm-portal",
  getParentRoute: () => protectedRoute,
  validateSearch: ATM_PORTAL_SEARCH_SCHEMA,
  component: lazyRouteComponent(() => import("@/features/atm-portal"), "AtmPortalScreen"),
});
