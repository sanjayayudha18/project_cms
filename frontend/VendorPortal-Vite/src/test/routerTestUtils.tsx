import { NotFound } from "@/components/NotFound";
import { AuthProvider } from "@/features/auth/AuthContext";
import { AuthLayout, ProtectedLayout } from "@/features/auth/ProtectedRoute";
import { queryClient } from "@/lib/queryClient";
import { QueryClientProvider } from "@tanstack/react-query";
import {
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  redirect,
} from "@tanstack/react-router";
import { render } from "@testing-library/react";
import type { ReactNode } from "react";
import { z } from "zod";

/**
 * Router test utilities (Task 8.1).
 *
 * Builds a TanStack Router instance backed by createMemoryHistory so property/unit
 * tests can drive routing from an initial path. The route tree mirrors the real one
 * (src/routes/*) but lets callers substitute lightweight page components so tests are
 * stable and fast. AuthProvider is always active via the root component, matching the
 * production provider placement.
 */

const redirectSearchSchema = z.object({
  redirect: z
    .string()
    .optional()
    .transform((v) => (v?.startsWith("/") && !v.startsWith("//") ? v : undefined)),
});

export interface TestPages {
  readonly login: () => ReactNode;
  readonly orders: () => ReactNode;
  readonly evidence: () => ReactNode;
  readonly invoices: () => ReactNode;
  readonly schedule: () => ReactNode;
  readonly dsr: () => ReactNode;
  readonly notifications: () => ReactNode;
}

const defaultPages: TestPages = {
  login: () => <span data-testid="page-login">login</span>,
  orders: () => <span data-testid="page-orders">orders</span>,
  evidence: () => <span data-testid="page-evidence">evidence</span>,
  invoices: () => <span data-testid="page-invoices">invoices</span>,
  schedule: () => <span data-testid="page-schedule">schedule</span>,
  dsr: () => <span data-testid="page-dsr">dsr</span>,
  notifications: () => <span data-testid="page-notifications">notifications</span>,
};

/**
 * Builds the test route tree + router for a given initial path.
 * The structure is identical to production: rootRoute (providers) > _auth > login,
 * _protected > shell > pages, plus index/dashboard redirects and internal masking.
 */
export function buildTestRouter(initialPath: string, pagesOverride?: Partial<TestPages>) {
  const pages = { ...defaultPages, ...pagesOverride };

  const rootRoute = createRootRoute({
    component: () => (
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <Outlet />
        </AuthProvider>
      </QueryClientProvider>
    ),
    notFoundComponent: NotFound,
  });

  const authRoute = createRoute({
    id: "_auth",
    getParentRoute: () => rootRoute,
    component: AuthLayout,
  });

  const loginRoute = createRoute({
    path: "/login",
    getParentRoute: () => authRoute,
    validateSearch: redirectSearchSchema,
    component: pages.login,
  });

  const protectedRoute = createRoute({
    id: "_protected",
    getParentRoute: () => rootRoute,
    component: ProtectedLayout,
  });

  const shellRoute = createRoute({
    id: "_shell",
    getParentRoute: () => protectedRoute,
    component: () => <Outlet />,
  });

  const ordersRoute = createRoute({
    path: "/orders",
    getParentRoute: () => shellRoute,
    component: pages.orders,
  });
  const evidenceRoute = createRoute({
    path: "/orders/$id/evidence",
    getParentRoute: () => shellRoute,
    component: pages.evidence,
  });
  const invoicesRoute = createRoute({
    path: "/invoices",
    getParentRoute: () => shellRoute,
    component: pages.invoices,
  });
  const scheduleRoute = createRoute({
    path: "/schedule",
    getParentRoute: () => shellRoute,
    component: pages.schedule,
  });
  const dsrRoute = createRoute({
    path: "/dsr",
    getParentRoute: () => shellRoute,
    component: pages.dsr,
  });
  const notificationsRoute = createRoute({
    path: "/notifications",
    getParentRoute: () => shellRoute,
    component: pages.notifications,
  });

  const indexRoute = createRoute({
    path: "/",
    getParentRoute: () => rootRoute,
    beforeLoad: () => {
      throw redirect({ to: "/orders" });
    },
  });
  const dashboardRoute = createRoute({
    path: "/dashboard",
    getParentRoute: () => rootRoute,
    beforeLoad: () => {
      throw redirect({ to: "/orders" });
    },
  });
  const adminRoute = createRoute({
    path: "/admin",
    getParentRoute: () => rootRoute,
    component: NotFound,
  });
  const forecastingRoute = createRoute({
    path: "/forecasting",
    getParentRoute: () => rootRoute,
    component: NotFound,
  });
  const reconciliationRoute = createRoute({
    path: "/reconciliation",
    getParentRoute: () => rootRoute,
    component: NotFound,
  });

  const routeTree = rootRoute.addChildren([
    authRoute.addChildren([loginRoute]),
    protectedRoute.addChildren([
      shellRoute.addChildren([
        ordersRoute,
        evidenceRoute,
        invoicesRoute,
        scheduleRoute,
        dsrRoute,
        notificationsRoute,
      ]),
    ]),
    indexRoute,
    dashboardRoute,
    adminRoute,
    forecastingRoute,
    reconciliationRoute,
  ]);

  const history = createMemoryHistory({ initialEntries: [initialPath] });
  const router = createRouter({ routeTree, history });
  return router;
}

/** Renders a test router built for `initialPath`. */
export function renderWithRouter(initialPath: string, pagesOverride?: Partial<TestPages>) {
  const router = buildTestRouter(initialPath, pagesOverride);
  const result = render(<RouterProvider router={router} />);
  return { ...result, router };
}
