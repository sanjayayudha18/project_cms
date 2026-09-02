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
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import * as fc from "fast-check";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { z } from "zod";

/**
 * Property-based tests for route/path preservation and internal masking.
 *
 * Feature: vendor-portal-tanstack-router-migration, Property 3: Preservasi Pemetaan Path Route
 * Feature: vendor-portal-tanstack-router-migration, Property 4: Internal-Route Masking Tak Terbedakan
 *
 * Validates: Requirements 1.2, 2.2, 2.3, 2.4, 2.5, 2.6
 */

const mockFetch = vi.fn();

beforeEach(() => {
  vi.stubGlobal("fetch", mockFetch);
  // Authenticated session so protected pages resolve to their real component.
  mockFetch.mockImplementation(async (url: string) => {
    if (typeof url === "string" && url.includes("/refresh")) {
      return new Response(
        JSON.stringify({
          access_token: "t",
          user: {
            id: 1,
            username: "v",
            full_name: "V",
            email: "v@x.com",
            role: "VENDOR-USER",
            is_karyawan: false,
            vendor_id: "vendor-gardanet",
          },
        }),
        { status: 200 },
      );
    }
    return new Response(null, { status: 404 });
  });
});

afterEach(() => {
  vi.restoreAllMocks();
  cleanup();
  queryClient.clear();
});

const redirectSearchSchema = z.object({
  redirect: z
    .string()
    .optional()
    .transform((v) => (v?.startsWith("/") && !v.startsWith("//") ? v : undefined)),
});

// Page probes with a stable testid so we can assert which page rendered.
function pageProbe(name: string) {
  return () => <span data-testid={`page-${name}`}>{name}</span>;
}

function buildRouter(initialPath: string) {
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
    component: pageProbe("login"),
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
    component: pageProbe("orders"),
  });
  const evidenceRoute = createRoute({
    path: "/orders/$id/evidence",
    getParentRoute: () => shellRoute,
    component: pageProbe("evidence"),
  });
  const invoicesRoute = createRoute({
    path: "/invoices",
    getParentRoute: () => shellRoute,
    component: pageProbe("invoices"),
  });
  const scheduleRoute = createRoute({
    path: "/schedule",
    getParentRoute: () => shellRoute,
    component: pageProbe("schedule"),
  });
  const dsrRoute = createRoute({
    path: "/dsr",
    getParentRoute: () => shellRoute,
    component: pageProbe("dsr"),
  });
  const notificationsRoute = createRoute({
    path: "/notifications",
    getParentRoute: () => shellRoute,
    component: pageProbe("notifications"),
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

  return createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: [initialPath] }),
  });
}

// ─── Property 3: Preservasi Pemetaan Path Route ─────────────────────────────────

describe("Feature: vendor-portal-tanstack-router-migration, Property 3: Preservasi Pemetaan Path Route", () => {
  const protectedPageArb = fc.constantFrom(
    { path: "/orders", testid: "page-orders" },
    { path: "/invoices", testid: "page-invoices" },
    { path: "/schedule", testid: "page-schedule" },
    { path: "/dsr", testid: "page-dsr" },
    { path: "/notifications", testid: "page-notifications" },
    { path: "/orders/abc/evidence", testid: "page-evidence" },
  );

  it("each protected path resolves to the same page inside the shell", async () => {
    await fc.assert(
      fc.asyncProperty(protectedPageArb, async ({ path, testid }) => {
        cleanup();
        queryClient.clear();
        render(<RouterProvider router={buildRouter(path)} />);
        await waitFor(() => {
          expect(screen.getByTestId(testid)).toBeInTheDocument();
        });
      }),
      { numRuns: 100 },
    );
  });

  it("/ and /dashboard both redirect to /orders", async () => {
    await fc.assert(
      fc.asyncProperty(fc.constantFrom("/", "/dashboard"), async (path) => {
        cleanup();
        queryClient.clear();
        render(<RouterProvider router={buildRouter(path)} />);
        await waitFor(() => {
          expect(screen.getByTestId("page-orders")).toBeInTheDocument();
        });
      }),
      { numRuns: 100 },
    );
  });
});

// ─── Property 4: Internal-Route Masking Tak Terbedakan ──────────────────────────

describe("Feature: vendor-portal-tanstack-router-migration, Property 4: Internal-Route Masking Tak Terbedakan", () => {
  // Internal-only paths and arbitrary unknown paths must both render NotFound,
  // observably indistinguishable.
  const maskedOrUnknownArb = fc.oneof(
    fc.constantFrom("/admin", "/forecasting", "/reconciliation"),
    fc
      .array(fc.constantFrom("a", "b", "c", "x", "y", "z", "1", "2", "9", "-", "_"), {
        minLength: 1,
        maxLength: 12,
      })
      .map((chars) => `/${chars.join("")}`)
      // Exclude accidental collisions with real top-level paths.
      .filter(
        (p) =>
          ![
            "/orders",
            "/invoices",
            "/schedule",
            "/dsr",
            "/notifications",
            "/login",
            "/dashboard",
            "/admin",
            "/forecasting",
            "/reconciliation",
          ].includes(p),
      ),
  );

  it("internal-only and unknown paths both render the NotFound page", async () => {
    await fc.assert(
      fc.asyncProperty(maskedOrUnknownArb, async (path) => {
        cleanup();
        queryClient.clear();
        render(<RouterProvider router={buildRouter(path)} />);
        await waitFor(() => {
          expect(screen.getByText("Halaman tidak ditemukan")).toBeInTheDocument();
        });
        // The "Kembali ke beranda" link is part of the NotFound page — confirms
        // identical output for masked and unknown routes.
        expect(screen.getByRole("link", { name: /kembali ke beranda/i })).toBeInTheDocument();
      }),
      { numRuns: 100 },
    );
  });
});
