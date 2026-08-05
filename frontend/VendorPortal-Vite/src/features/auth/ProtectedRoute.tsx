import { Navigate, Outlet, useLocation } from 'react-router';
import { useAuth } from '@/features/auth/useAuth';

/**
 * Route guard for authenticated-only routes.
 * Redirects unauthenticated users to /login, preserving the originally
 * requested URL so they can be redirected back after login.
 */
export function ProtectedRoute() {
  const { state } = useAuth();
  const location = useLocation();

  if (!state.isAuthenticated) {
    return (
      <Navigate
        to="/login"
        state={{ from: location.pathname }}
        replace
      />
    );
  }

  return <Outlet />;
}

/**
 * Route guard for guest-only routes (e.g. /login).
 * Redirects authenticated users to /orders since they don't need
 * the login screen.
 */
export function GuestRoute() {
  const { state } = useAuth();

  if (state.isAuthenticated) {
    return <Navigate to="/orders" replace />;
  }

  return <Outlet />;
}
