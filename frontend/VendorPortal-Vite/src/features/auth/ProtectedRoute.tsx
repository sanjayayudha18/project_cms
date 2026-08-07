import { Navigate, Outlet, useLocation } from 'react-router';
import { Loader2 } from 'lucide-react';
import { useAuth } from '@/features/auth/useAuth';

/**
 * Route guard for authenticated-only routes.
 * Shows a loading spinner while auth state initializes (isAuthLoading).
 * Redirects unauthenticated users to /login?redirect={path}, preserving
 * the originally requested URL so they can be redirected back after login.
 */
export function ProtectedRoute() {
  const { state } = useAuth();
  const location = useLocation();

  // Wait for auth initialization before making decisions
  if (state.isAuthLoading) {
    return (
      <div
        className="flex min-h-svh items-center justify-center"
        style={{ backgroundColor: 'var(--n-50, oklch(0.975 0.004 29))' }}
      >
        <Loader2
          className="h-8 w-8 animate-spin"
          style={{ color: 'var(--red-500, oklch(0.552 0.205 29))' }}
        />
      </div>
    );
  }

  if (!state.isAuthenticated) {
    const currentPath = location.pathname + location.search;
    return (
      <Navigate
        to={`/login?redirect=${encodeURIComponent(currentPath)}`}
        replace
      />
    );
  }

  return <Outlet />;
}

/**
 * Route guard for guest-only routes (e.g. /login).
 * Redirects authenticated users to /dashboard since they don't need
 * the login screen.
 */
export function GuestRoute() {
  const { state } = useAuth();

  // Wait for auth initialization
  if (state.isAuthLoading) {
    return (
      <div
        className="flex min-h-svh items-center justify-center"
        style={{ backgroundColor: 'var(--n-50, oklch(0.975 0.004 29))' }}
      >
        <Loader2
          className="h-8 w-8 animate-spin"
          style={{ color: 'var(--red-500, oklch(0.552 0.205 29))' }}
        />
      </div>
    );
  }

  if (state.isAuthenticated) {
    return <Navigate to="/dashboard" replace />;
  }

  return <Outlet />;
}
