import { createRoute, redirect } from "@tanstack/react-router";
import { rootRoute } from "./__root";

/**
 * `/` redirects to /orders (Requirement 2.3). Pure redirect — it still lands on the
 * auth-guarded /orders route, so unauthenticated users are subsequently sent to /login.
 */
export const indexRoute = createRoute({
  path: "/",
  getParentRoute: () => rootRoute,
  beforeLoad: () => {
    throw redirect({ to: "/orders" });
  },
});
