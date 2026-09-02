import { useAuth } from "@/features/auth/useAuth";
import { Outlet, useLocation, useNavigate } from "@tanstack/react-router";
import { Loader2 } from "lucide-react";
import { useEffect, useRef, useState } from "react";

/** Milliseconds to wait for auth initialization before giving up (Requirement 3.7). */
const AUTH_INIT_TIMEOUT_MS = 10_000;

/** Full-page spinner shown while auth state initializes. Markup preserved from pre-migration. */
function AuthLoadingSpinner() {
  return (
    <div
      className="flex min-h-svh items-center justify-center"
      style={{ backgroundColor: "var(--n-50, oklch(0.975 0.004 29))" }}
    >
      <Loader2
        className="h-8 w-8 animate-spin"
        style={{ color: "var(--red-500, oklch(0.552 0.205 29))" }}
      />
    </div>
  );
}

/**
 * Auth guard layout for authenticated-only routes (padanan ProtectedRoute lama).
 *
 * - While `isAuthLoading` → spinner (Requirement 3.6, 4.4), with a 10s timeout
 *   that falls back to /login if auth never resolves (Requirement 3.7).
 * - `!isAuthenticated` → redirect to /login?redirect={pathname+search}, preserving
 *   the originally requested URL (Requirement 3.1, 4.5).
 * - Otherwise → render the nested route via <Outlet/>.
 *
 * Redirects run inside an effect (never during render) to avoid TanStack Router
 * navigation loops. AuthContext is NOT modified — the timeout lives purely here.
 */
export function ProtectedLayout() {
  const { state } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [timedOut, setTimedOut] = useState(false);
  const hasRedirected = useRef(false);

  const stillLoading = state.isAuthLoading && !timedOut;
  const shouldRedirect = !stillLoading && !state.isAuthenticated;

  // 10s auth-init timeout (Requirement 3.7).
  useEffect(() => {
    if (!state.isAuthLoading) {
      setTimedOut(false);
      return;
    }
    const timer = setTimeout(() => setTimedOut(true), AUTH_INIT_TIMEOUT_MS);
    return () => clearTimeout(timer);
  }, [state.isAuthLoading]);

  // Redirect unauthenticated users to /login, preserving the requested path.
  // Captured once via a ref so the redirect fires a single time per mount and the
  // navigation itself (which changes location) does not retrigger the effect.
  useEffect(() => {
    if (shouldRedirect && !hasRedirected.current) {
      hasRedirected.current = true;
      const currentPath = location.pathname + location.searchStr;
      void navigate({ to: "/login", search: { redirect: currentPath }, replace: true });
    }
  }, [shouldRedirect, location.pathname, location.searchStr, navigate]);

  if (stillLoading) {
    return <AuthLoadingSpinner />;
  }

  if (!state.isAuthenticated) {
    // Redirect is in-flight (scheduled by the effect); render nothing to avoid a flash.
    return null;
  }

  return <Outlet />;
}

/**
 * Guest guard layout for guest-only routes such as /login (padanan GuestRoute lama).
 *
 * - While `isAuthLoading` → spinner (Requirement 3.6, 4.4).
 * - `isAuthenticated` → redirect authenticated users away from the guest area
 *   (Requirement 3.5, 4.6). When a valid `redirect` param is present it wins
 *   (Requirement 3.2, 3.3); otherwise the default destination /orders is used.
 *   Honoring the param here (in addition to LoginPage) makes the guard and the login
 *   flow agree on the same target, avoiding a redirect race after a fresh login.
 *   Pre-migration went to /dashboard which itself redirects to /orders; going straight
 *   to /orders is observably identical.
 * - Otherwise → render the nested route via <Outlet/>.
 */
export function AuthLayout() {
  const { state } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const hasRedirected = useRef(false);

  const shouldRedirect = !state.isAuthLoading && state.isAuthenticated;

  useEffect(() => {
    if (shouldRedirect && !hasRedirected.current) {
      hasRedirected.current = true;
      // Read `redirect` from the raw query string. The _auth layout route has no
      // validateSearch, so its typed `location.search` does not carry `redirect`
      // (search is scoped per-route to the child loginRoute). searchStr is the raw
      // query and is reliable at this level.
      const params = new URLSearchParams(location.searchStr);
      const rawRedirect = params.get("redirect");
      const isSafe = rawRedirect?.startsWith("/") && !rawRedirect.startsWith("//");
      // Navigate via href so an arbitrary preserved path (possibly with dynamic
      // segments, e.g. /orders/123/evidence) resolves; fall back to /orders default.
      const target = isSafe && rawRedirect ? rawRedirect : "/orders";
      void navigate({ href: target, replace: true });
    }
  }, [shouldRedirect, location.searchStr, navigate]);

  if (state.isAuthLoading) {
    return <AuthLoadingSpinner />;
  }

  if (state.isAuthenticated) {
    return null;
  }

  return <Outlet />;
}
