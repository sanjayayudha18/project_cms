import { NotFound } from "@/components/NotFound";
import { AuthProvider } from "@/features/auth/AuthContext";
import { queryClient } from "@/lib/queryClient";
import { QueryClientProvider } from "@tanstack/react-query";
import { Outlet, createRootRoute } from "@tanstack/react-router";

/**
 * Root route.
 *
 * Places the existing QueryClientProvider (outermost data-fetching provider,
 * Requirement 5.1) and the existing AuthProvider (Requirement 4.1) around the
 * router Outlet, mirroring the CompanyPortal convention. The AuthProvider must
 * sit above every route component because the Vendor Portal guards read auth via
 * useAuth() inside layout components (not beforeLoad).
 *
 * notFoundComponent renders the shared NotFound page for any unmatched path
 * (Requirement 2.6, 6.5).
 */
export const rootRoute = createRootRoute({
  component: RootComponent,
  notFoundComponent: NotFound,
});

function RootComponent() {
  return (
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <Outlet />
      </AuthProvider>
    </QueryClientProvider>
  );
}
