import { AppShell } from "@/components/layout/AppShell";
import { Forbidden } from "@/components/pages/Forbidden";
import { NotificationBell } from "@/features/notifications";
import { usePermissionStore } from "@/lib/auth/permissionStore";
import { useAuthStore } from "@/lib/auth/store";
import type { DbRole } from "@/lib/auth/store";
import { ErrorComponent, Navigate, Outlet, createRoute, redirect } from "@tanstack/react-router";
import type { ErrorComponentProps } from "@tanstack/react-router";
import { rootRoute } from "./__root";

/**
 * Protected layout — wraps all authenticated routes with AppShell + route guard.
 *
 * beforeLoad checks (in order):
 * 1. isAuthLoading → allow through (component handles loading state)
 * 2. Not authenticated → redirect to /login?redirect={currentPath}
 * 3. Access check delegated to child routes via requirePermission()
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
  errorComponent: ProtectedError,
});

/** Thrown by requirePermission; protectedRoute's errorComponent renders the 403 page. */
export class ForbiddenError extends Error {
  constructor() {
    super("Forbidden");
    this.name = "ForbiddenError";
  }
}

function ProtectedError(props: ErrorComponentProps) {
  if (props.error instanceof ForbiddenError) {
    return (
      <AppShell notifications={<NotificationBell />}>
        <Forbidden />
      </AppShell>
    );
  }
  return <ErrorComponent {...props} />;
}

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
    <AppShell notifications={<NotificationBell />}>
      <Outlet />
    </AppShell>
  );
}

// ─── Permission guard (spec permission-catalog-coverage) ──────────────────────

/**
 * beforeLoad guard: the user needs at least one of `key` in their role's permission set.
 * No ADMIN/ADMIN_PARAM bypass. `opts.roles` narrows further (e.g. exports dataset).
 * Waits for session restore and the permission load; no key → throws ForbiddenError.
 */
export function requirePermission(key: string | readonly string[], opts?: { roles?: DbRole[] }) {
  const keys = typeof key === "string" ? [key] : key;
  return async () => {
    const { user, isAuthLoading } = useAuthStore.getState();
    // Session still restoring; RootComponent re-runs guards once it resolves.
    if (isAuthLoading) return;
    if (!user) throw redirect({ to: "/login" });

    const perms = usePermissionStore.getState();
    if (perms.status === "idle" || perms.status === "loading") await perms.load();

    if (opts?.roles && !opts.roles.includes(user.role)) throw new ForbiddenError();
    if (!usePermissionStore.getState().hasAny(keys)) throw new ForbiddenError();
  };
}
