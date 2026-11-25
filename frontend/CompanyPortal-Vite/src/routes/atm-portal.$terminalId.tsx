import { ATM_PROFILE_SEARCH_SCHEMA } from "@/features/atm-portal/useAtmProfileUrlState";
import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requirePermission } from "./_protected";

export const atmProfileRoute = createRoute({
  path: "/atm-portal/$terminalId",
  getParentRoute: () => protectedRoute,
  beforeLoad: requirePermission("monitoring.atm-portal"),
  validateSearch: ATM_PROFILE_SEARCH_SCHEMA,
  component: lazyRouteComponent(() => import("@/features/atm-portal"), "AtmProfileScreen"),
});
