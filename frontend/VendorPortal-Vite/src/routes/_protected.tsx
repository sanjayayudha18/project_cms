import { AppShell } from "@/app/AppShell";
import { ProtectedLayout } from "@/features/auth/ProtectedRoute";
import { Outlet, createRoute } from "@tanstack/react-router";
import { rootRoute } from "./__root";

/**
 * Pathless protected layout (padanan ProtectedRoute lama). Applies the auth guard
 * via ProtectedLayout, which redirects unauthenticated users to /login preserving
 * the requested path (Requirement 3.1) and enforces the 10s auth-init timeout
 * (Requirement 3.7).
 */
export const protectedRoute = createRoute({
  id: "_protected",
  getParentRoute: () => rootRoute,
  component: ProtectedLayout,
});

/**
 * Pathless shell layout under the protected route. Wraps the authenticated pages
 * in AppShell (Requirement 2.7).
 */
export const shellRoute = createRoute({
  id: "_shell",
  getParentRoute: () => protectedRoute,
  component: ShellLayout,
});

function ShellLayout() {
  return (
    <AppShell>
      <Outlet />
    </AppShell>
  );
}
