import { AppShell } from "@/components/layout/AppShell";
import { useAuthStore } from "@/lib/auth/store";
import type { Role } from "@/lib/auth/store";
import { Navigate, Outlet, createRoute, redirect, useRouter } from "@tanstack/react-router";
import { rootRoute } from "./__root";

/**
 * Protected layout — wraps all authenticated routes with AppShell + route guard.
 *
 * beforeLoad checks:
 * 1. Is user authenticated? If not → redirect to /login
 * 2. Does user have the required role for this route? If not → render Unauthorized page
 */
export const protectedRoute = createRoute({
  id: "_protected",
  getParentRoute: () => rootRoute,
  beforeLoad: () => {
    const { isAuthenticated, isLoading } = useAuthStore.getState();

    // If still loading auth state, allow through (component will handle loading)
    if (isLoading) return;

    if (!isAuthenticated) {
      throw redirect({ to: "/login" });
    }
  },
  component: ProtectedLayout,
});

function ProtectedLayout() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  const isLoading = useAuthStore((s) => s.isLoading);

  // Double-check: if auth resolved as unauthenticated, redirect
  if (!isLoading && !isAuthenticated) {
    return <Navigate to="/login" />;
  }

  return (
    <AppShell>
      <Outlet />
    </AppShell>
  );
}

// ─── Unauthorized Page ────────────────────────────────────────────────────────

export function UnauthorizedPage() {
  const router = useRouter();

  return (
    <div className="flex min-h-[60vh] flex-col items-center justify-center gap-[var(--space-4)] text-center">
      <div
        className="flex h-16 w-16 items-center justify-center rounded-full"
        style={{ backgroundColor: "var(--danger-bg)" }}
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          width="32"
          height="32"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
          style={{ color: "var(--danger-fg)" }}
          aria-hidden="true"
        >
          <circle cx="12" cy="12" r="10" />
          <line x1="4.93" y1="4.93" x2="19.07" y2="19.07" />
        </svg>
      </div>

      <h1 className="text-xl font-semibold" style={{ color: "var(--n-900)" }}>
        Akses Tidak Diizinkan
      </h1>

      <p className="max-w-[400px] text-sm" style={{ color: "var(--n-600)" }}>
        Anda tidak memiliki izin untuk mengakses halaman ini. Silakan hubungi administrator jika
        Anda memerlukan akses.
      </p>

      <button
        type="button"
        onClick={() => router.navigate({ to: "/" })}
        className="mt-[var(--space-2)] inline-flex items-center gap-[var(--space-2)] rounded-[var(--radius-md)] px-[var(--space-4)] py-[var(--space-2)] text-sm font-medium transition-colors duration-200"
        style={{
          color: "var(--red-600)",
          backgroundColor: "var(--red-50)",
        }}
      >
        Kembali ke Beranda
      </button>
    </div>
  );
}

// ─── RBAC Helper ──────────────────────────────────────────────────────────────

/**
 * Creates a beforeLoad guard that checks user roles against required roles.
 * Use in child routes that need role-specific access control.
 */
export function requireRoles(requiredRoles: Role[]) {
  return () => {
    const { user } = useAuthStore.getState();

    if (!user) {
      throw redirect({ to: "/login" });
    }

    // Admin bypasses all role checks
    if (user.roles.includes("Admin")) return;

    const hasRequiredRole = requiredRoles.some((role) => user.roles.includes(role));

    if (!hasRequiredRole) {
      // We'll signal to the component to show unauthorized
      return { unauthorized: true };
    }
  };
}
