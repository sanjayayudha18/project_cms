import { NotFound } from "@/components/NotFound";
import { AuthProvider } from "@/features/auth/AuthContext";
import { AuthLayout, ProtectedLayout } from "@/features/auth/ProtectedRoute";
import { useAuth } from "@/features/auth/useAuth";
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
  useRouterState,
} from "@tanstack/react-router";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import * as fc from "fast-check";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { z } from "zod";

/**
 * Property-based tests for the TanStack Router migration.
 *
 * Feature: vendor-portal-tanstack-router-migration, Property 1: Route-Guard Round-Trip
 * Feature: vendor-portal-tanstack-router-migration, Property 2: Sanitasi Redirect Param
 *
 * Validates: Requirements 3.1, 3.2, 3.3, 3.4, 7.1, 7.2, 7.3
 */

// ─── Mock fetch ───────────────────────────────────────────────────────────────

const mockFetch = vi.fn();

beforeEach(() => {
  vi.stubGlobal("fetch", mockFetch);
  // Default: refresh on mount returns 401 (unauthenticated); login succeeds as vendor.
  mockFetch.mockImplementation(async (url: string) => {
    if (typeof url === "string" && url.includes("/login")) {
      return new Response(
        JSON.stringify({
          access_token: "mock-token",
          user: {
            id: 1,
            username: "vendor.user",
            full_name: "Vendor User",
            email: "vendor@example.com",
            role: "VENDOR-USER",
            is_karyawan: false,
            vendor_id: "vendor-gardanet",
          },
        }),
        { status: 200 },
      );
    }
    return new Response(JSON.stringify({ error: "unauthorized" }), { status: 401 });
  });
});

afterEach(() => {
  vi.restoreAllMocks();
});

// ─── Shared schema (mirrors src/routes/login.tsx) ───────────────────────────────

const redirectSearchSchema = z.object({
  redirect: z
    .string()
    .optional()
    .transform((v) => (v?.startsWith("/") && !v.startsWith("//") ? v : undefined)),
});

// ─── Test pages ─────────────────────────────────────────────────────────────────

function CurrentLocation() {
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const search = useRouterState({ select: (s) => s.location.searchStr });
  return (
    <>
      <span data-testid="pathname">{pathname}</span>
      <span data-testid="search">{search}</span>
    </>
  );
}

function MockLoginPage() {
  const { login } = useAuth();
  // Read the committed redirect search param from the router location, then apply
  // the same sanitization as production (safe internal paths only). This is shown for
  // assertion; the actual post-login navigation is performed by the guest guard.
  const rawRedirect = useRouterState({
    select: (s) => (s.location.search as { redirect?: string }).redirect,
  });
  const redirectTo =
    rawRedirect?.startsWith("/") && !rawRedirect.startsWith("//") ? rawRedirect : "/orders";

  return (
    <div>
      <span data-testid="login-page">login</span>
      <span data-testid="redirect-target">{redirectTo}</span>
      <button
        type="button"
        data-testid="do-login"
        onClick={async () => {
          try {
            // Only authenticate; the guest guard (AuthLayout) performs the post-login
            // redirect based on the preserved `redirect` param. This mirrors production
            // and avoids a double-navigation race.
            await login("vendor.user", "password123");
          } catch {
            // ignore
          }
        }}
      >
        Login
      </button>
      <CurrentLocation />
    </div>
  );
}

function ProtectedProbe() {
  return (
    <>
      <span data-testid="protected-page">protected</span>
      <CurrentLocation />
    </>
  );
}

// ─── Router builder ───────────────────────────────────────────────────────────

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
    component: MockLoginPage,
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
    component: ProtectedProbe,
  });
  const evidenceRoute = createRoute({
    path: "/orders/$id/evidence",
    getParentRoute: () => shellRoute,
    component: ProtectedProbe,
  });
  const invoicesRoute = createRoute({
    path: "/invoices",
    getParentRoute: () => shellRoute,
    component: ProtectedProbe,
  });
  const scheduleRoute = createRoute({
    path: "/schedule",
    getParentRoute: () => shellRoute,
    component: ProtectedProbe,
  });
  const dsrRoute = createRoute({
    path: "/dsr",
    getParentRoute: () => shellRoute,
    component: ProtectedProbe,
  });
  const notificationsRoute = createRoute({
    path: "/notifications",
    getParentRoute: () => shellRoute,
    component: ProtectedProbe,
  });

  const indexRoute = createRoute({
    path: "/",
    getParentRoute: () => rootRoute,
    beforeLoad: () => {
      throw redirect({ to: "/orders" });
    },
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
  ]);

  const history = createMemoryHistory({ initialEntries: [initialPath] });
  return createRouter({ routeTree, history });
}

// ─── Generators ─────────────────────────────────────────────────────────────────

const protectedPathArb = fc.oneof(
  fc.constant("/orders"),
  fc.constant("/invoices"),
  fc.constant("/schedule"),
  fc.constant("/dsr"),
  fc.constant("/notifications"),
  fc
    .array(fc.constantFrom("a", "b", "c", "1", "2", "3", "-"), { minLength: 1, maxLength: 8 })
    .map((chars) => `/orders/${chars.join("")}/evidence`),
);

// Optional query string appended to a protected path. Keys are unique (a subset of
// distinct keys) so TanStack Router's search (de)serialization round-trips byte-for-byte
// — duplicate keys would be normalized into arrays and are not representative of real links.
const queryStringArb = fc.oneof(
  fc.constant(""),
  fc
    .uniqueArray(
      fc.record({
        k: fc.constantFrom("tab", "page", "q", "sort"),
        v: fc.constantFrom("1", "2", "open", "asc", "x"),
      }),
      { minLength: 1, maxLength: 3, selector: (p) => p.k },
    )
    .map((pairs) => `?${pairs.map((p) => `${p.k}=${p.v}`).join("&")}`),
);

async function settle() {
  await act(async () => {
    await new Promise((r) => setTimeout(r, 20));
  });
}

// Interactive round-trip properties mount a full router + real timers + userEvent per
// run, which is memory-heavy. The non-interactive redirect check stays at 100 runs;
// interactive checks use a smaller-but-still-broad sample to stay within heap limits.
const INTERACTIVE_RUNS = 40;

// ─── Property 1: Route-Guard Round-Trip ─────────────────────────────────────────

describe("Feature: vendor-portal-tanstack-router-migration, Property 1: Route-Guard Round-Trip", () => {
  it("unauthenticated access to a protected path redirects to /login preserving the path + query", async () => {
    await fc.assert(
      fc.asyncProperty(protectedPathArb, queryStringArb, async (path, qs) => {
        cleanup();
        queryClient.clear();
        const fullPath = path + qs;
        const router = buildRouter(fullPath);
        const { unmount } = render(<RouterProvider router={router} />);

        await settle();

        // Redirected to /login
        expect(screen.getByTestId("login-page")).toBeInTheDocument();
        expect(screen.getByTestId("pathname").textContent).toBe("/login");

        // redirect target preserves the original full path (pathname + search)
        expect(screen.getByTestId("redirect-target").textContent).toBe(fullPath);

        unmount();
        cleanup();
        queryClient.clear();
      }),
      { numRuns: 100 },
    );
  });

  it("after successful login, user is returned to the originally requested path", async () => {
    const user = userEvent.setup();
    await fc.assert(
      fc.asyncProperty(protectedPathArb, async (path) => {
        cleanup();
        queryClient.clear();
        const router = buildRouter(path);
        const { unmount } = render(<RouterProvider router={router} />);

        await settle();
        expect(screen.getByTestId("login-page")).toBeInTheDocument();
        // The guard must have preserved the requested path into the redirect target
        // before we log in, otherwise post-login would fall back to /orders.
        await waitFor(() => {
          expect(screen.getByTestId("redirect-target").textContent).toBe(path);
        });

        await user.click(screen.getByTestId("do-login"));
        await settle();

        // Back on the protected page at the original path
        expect(screen.getByTestId("protected-page")).toBeInTheDocument();
        expect(screen.getByTestId("pathname").textContent).toBe(path);

        unmount();
        cleanup();
        queryClient.clear();
      }),
      { numRuns: INTERACTIVE_RUNS },
    );
  });
});

// ─── Property 2: Sanitasi Redirect Param ────────────────────────────────────────

describe("Feature: vendor-portal-tanstack-router-migration, Property 2: Sanitasi Redirect Param", () => {
  const unsafeRedirectArb = fc.oneof(
    fc.webUrl(),
    fc.constant("//evil.com"),
    fc.constant("//evil.com/path"),
    fc.string().map((s) => `http://${s}`),
    fc.string().map((s) => `https://${s}`),
    // non-path junk not starting with a single "/"
    fc
      .string({ minLength: 1 })
      .filter((s) => !s.startsWith("/")),
  );

  it("unsafe redirect values result in landing on /orders after login", async () => {
    const user = userEvent.setup();
    await fc.assert(
      fc.asyncProperty(unsafeRedirectArb, async (unsafe) => {
        cleanup();
        queryClient.clear();
        const router = buildRouter(`/login?redirect=${encodeURIComponent(unsafe)}`);
        const { unmount } = render(<RouterProvider router={router} />);

        await settle();
        expect(screen.getByTestId("login-page")).toBeInTheDocument();
        // sanitized target falls back to /orders
        expect(screen.getByTestId("redirect-target").textContent).toBe("/orders");

        await user.click(screen.getByTestId("do-login"));
        await settle();

        expect(screen.getByTestId("protected-page")).toBeInTheDocument();
        expect(screen.getByTestId("pathname").textContent).toBe("/orders");

        unmount();
        cleanup();
        queryClient.clear();
      }),
      { numRuns: INTERACTIVE_RUNS },
    );
  });
});
