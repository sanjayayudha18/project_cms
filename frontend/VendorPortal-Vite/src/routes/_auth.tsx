import { AuthLayout } from "@/features/auth/ProtectedRoute";
import { createRoute } from "@tanstack/react-router";
import { rootRoute } from "./__root";

/**
 * Pathless guest layout (padanan GuestRoute lama). Applies the guest guard via
 * AuthLayout, which redirects authenticated users to /orders (Requirement 3.5).
 */
export const authRoute = createRoute({
  id: "_auth",
  getParentRoute: () => rootRoute,
  component: AuthLayout,
});
