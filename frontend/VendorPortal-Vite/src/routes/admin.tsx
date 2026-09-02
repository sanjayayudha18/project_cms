import { NotFound } from "@/components/NotFound";
import { createRoute } from "@tanstack/react-router";
import { rootRoute } from "./__root";

/**
 * Internal-only route masked as NotFound so its existence is not revealed in the
 * Vendor Portal — output identical to an unknown path (Requirement 2.5).
 */
export const adminRoute = createRoute({
  path: "/admin",
  getParentRoute: () => rootRoute,
  component: NotFound,
});
