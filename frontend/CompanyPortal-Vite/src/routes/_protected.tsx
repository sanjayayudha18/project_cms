import { AppShell } from "@/components/layout/AppShell";
import { useAuthStore } from "@/lib/auth/store";
import type { DbRole } from "@/lib/auth/store";
import { Navigate, Outlet, createRoute, redirect } from "@tanstack/react-router";
import { rootRoute } from "./__root";

/**
 * Protected layout — wraps all authenticated routes with AppShell + route guard.
 *
 * beforeLoad checks (in order):
 * 1. isAuthLoading → allow through (component handles loading state)
 * 2. Not authenticated → redirect to /login?redirect={currentPath}
 * 3. Role check delegated to child routes via requireRoles()
 */
export const protectedRoute = createRoute({
  id: "_protected",
  getParentRoute: () => rootRoute,
  beforeLoad: ({ location }) => {
    const { isAuthenticated, isAuthLoading } = useAuthStore.getState();

    // Step 1: If still loading auth state, allow through (component handles loading)
    if (isAuthLoading) return;

    // Step 2: If unauthenticated, redirect to login with the original path preserved
    if (!isAuthenticated) {
      throw redirect({
        to: "/login",
        search: { redirect: location.pathname },
      });
    }
  },
  component: ProtectedLayout,
});

function ProtectedLayout() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  const isAuthLoading = useAuthStore((s) => s.isAuthLoading);
  const currentPath = window.location.pathname;

  // Step 1: If auth is still loading, show nothing (prevents flash)
  if (isAuthLoading) return null;

  // Step 2: If auth resolved as unauthenticated, redirect with path preserved
  if (!isAuthenticated) {
    return <Navigate to="/login" search={{ redirect: currentPath }} />;
  }

  // Auth is resolved and user is authenticated → render AppShell within 1 cycle
  return (
    <AppShell>
      <Outlet />
    </AppShell>
  );
}

// ─── RBAC Helper ──────────────────────────────────────────────────────────────

/**
 * Creates a beforeLoad guard that checks user role against required roles.
 * Use in child routes that need role-specific access control.
 *
 * Behavior:
 * - If user is not authenticated → redirect to /login
 * - If user role is ADMIN or ADMIN_PARAM → bypass role check (access granted)
 * - If user role is in allowedRoles → access granted
 * - Otherwise → return { forbidden: true } to signal component to show 403 page
 */
export function requireRoles(allowedRoles: DbRole[]) {
  return () => {
    const { user } = useAuthStore.getState();

    if (!user) {
      throw redirect({ to: "/login" });
    }

    // ADMIN and ADMIN_PARAM bypass all role checks
    if (user.role === "ADMIN" || user.role === "ADMIN_PARAM") return;

    const hasRequiredRole = allowedRoles.includes(user.role);

    if (!hasRequiredRole) {
      // Signal to the route component to show 403 Forbidden page
      return { forbidden: true };
    }
  };
}
