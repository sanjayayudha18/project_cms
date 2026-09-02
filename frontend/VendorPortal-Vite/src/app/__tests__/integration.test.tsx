import { AppShell } from "@/app/AppShell";
import { NotFound } from "@/components/NotFound";
import ordersData from "@/data/orders.json";
import { AuthProvider } from "@/features/auth/AuthContext";
import { LoginPage } from "@/features/auth/LoginPage";
import { AuthLayout, ProtectedLayout } from "@/features/auth/ProtectedRoute";
import { DsrPage } from "@/features/dsr/DsrPage";
import { InvoicesPage } from "@/features/invoices/InvoicesPage";
import { NotificationsPage } from "@/features/notifications/NotificationsPage";
import { OrdersPage } from "@/features/orders/OrdersPage";
import { SchedulePage } from "@/features/schedule/SchedulePage";
import { queryClient } from "@/lib/queryClient";
import type { CITOrder } from "@/lib/types";
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
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, vi } from "vitest";
import { z } from "zod";

// ─── Mock fetch ───────────────────────────────────────────────────────────────

const mockFetch = vi.fn();

beforeEach(() => {
  vi.stubGlobal("fetch", mockFetch);
  // Default: simulate successful login for integration tests
  mockFetch.mockImplementation(async (url: string) => {
    if (typeof url === "string" && url.includes("/refresh")) {
      return new Response(JSON.stringify({ error: "unauthorized" }), { status: 401 });
    }
    if (typeof url === "string" && url.includes("/login")) {
      return new Response(
        JSON.stringify({
          access_token: "mock-access-token",
          user: {
            id: 1,
            username: "gardanet.admin",
            full_name: "Budi Santoso",
            email: "budi@gardanet.com",
            role: "VENDOR-USER",
            is_karyawan: false,
            // Must match the vendorId used in orders.json seed data.
            // useOrders() filters with String(vendorId), so this string flows through unchanged.
            vendor_id: "vendor-gardanet",
          },
        }),
        { status: 200 },
      );
    }
    if (typeof url === "string" && url.includes("/logout")) {
      return new Response(null, { status: 200 });
    }
    return new Response(null, { status: 404 });
  });
});

afterEach(() => {
  vi.restoreAllMocks();
  queryClient.clear();
});

// ─── Redirect search schema (mirrors src/routes/login.tsx) ──────────────────────

const redirectSearchSchema = z.object({
  redirect: z
    .string()
    .optional()
    .transform((v) => (v?.startsWith("/") && !v.startsWith("//") ? v : undefined)),
});

// ─── Helper: render app with the real TanStack Router tree ──────────────────────

function renderApp(initialPath = "/") {
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
    component: LoginPage,
  });

  const protectedRoute = createRoute({
    id: "_protected",
    getParentRoute: () => rootRoute,
    component: ProtectedLayout,
  });
  const shellRoute = createRoute({
    id: "_shell",
    getParentRoute: () => protectedRoute,
    component: () => (
      <AppShell>
        <Outlet />
      </AppShell>
    ),
  });
  const ordersRoute = createRoute({
    path: "/orders",
    getParentRoute: () => shellRoute,
    component: OrdersPage,
  });
  const invoicesRoute = createRoute({
    path: "/invoices",
    getParentRoute: () => shellRoute,
    component: InvoicesPage,
  });
  const scheduleRoute = createRoute({
    path: "/schedule",
    getParentRoute: () => shellRoute,
    component: SchedulePage,
  });
  const dsrRoute = createRoute({
    path: "/dsr",
    getParentRoute: () => shellRoute,
    component: DsrPage,
  });
  const notificationsRoute = createRoute({
    path: "/notifications",
    getParentRoute: () => shellRoute,
    component: NotificationsPage,
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
        invoicesRoute,
        scheduleRoute,
        dsrRoute,
        notificationsRoute,
      ]),
    ]),
    indexRoute,
  ]);

  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: [initialPath] }),
  });

  const result = render(<RouterProvider router={router} />);
  return { ...result, router };
}

// --- Helper: perform login ---

async function performLogin(
  user: ReturnType<typeof userEvent.setup>,
  username: string,
  password: string,
) {
  const usernameInput = screen.getByLabelText("Nama pengguna");
  const passwordInput = screen.getByLabelText("Kata sandi");
  const submitButton = screen.getByRole("button", { name: /masuk/i });

  await user.type(usernameInput, username);
  await user.type(passwordInput, password);
  await user.click(submitButton);
}

// ===== Test 1: Login → redirect → data display =====

describe("Login → redirect → data display flow", () => {
  it("redirects unauthenticated user to login, then shows orders after login", async () => {
    const user = userEvent.setup();
    renderApp("/orders");

    // Should redirect to login since unauthenticated (after isAuthLoading resolves)
    await waitFor(() => {
      expect(screen.getByLabelText("Nama pengguna")).toBeInTheDocument();
    });

    // Fill in credentials and submit
    await performLogin(user, "gardanet.admin", "password123");

    // Should redirect to orders page and display CIT Orders heading
    await waitFor(() => {
      expect(screen.getByRole("heading", { name: /cit orders/i })).toBeInTheDocument();
    });

    // Verify orders data is displayed (Gardanet ATM IDs visible)
    await waitFor(() => {
      expect(screen.getByText("ATM-JKT-001")).toBeInTheDocument();
    });
  });
});

// ===== Test 2: Vendor scoping =====

describe("Vendor scoping", () => {
  it("logged in as Gardanet shows only Gardanet data, no SSI or G4S data", async () => {
    const user = userEvent.setup();
    renderApp("/orders");

    // Login as gardanet.admin
    await waitFor(() => {
      expect(screen.getByLabelText("Nama pengguna")).toBeInTheDocument();
    });
    await performLogin(user, "gardanet.admin", "password123");

    // Wait for orders page to render
    await waitFor(() => {
      expect(screen.getByRole("heading", { name: /cit orders/i })).toBeInTheDocument();
    });

    // Wait for data to load — check a known Gardanet ATM ID
    await waitFor(() => {
      expect(screen.getByText("ATM-JKT-001")).toBeInTheDocument();
    });

    // Verify Gardanet ATM IDs are present (first 3 Gardanet orders)
    const gardanetOrders = (ordersData as CITOrder[]).filter(
      (o) => o.vendorId === "vendor-gardanet",
    );
    for (const order of gardanetOrders.slice(0, 3)) {
      expect(screen.getByText(order.atmId)).toBeInTheDocument();
    }

    // Verify SSI ATM IDs are NOT visible (ATM-SBY-001 belongs to SSI)
    expect(screen.queryByText("ATM-SBY-001")).not.toBeInTheDocument();
    // Verify G4S ATM IDs are NOT visible (ATM-SMG-001 belongs to G4S)
    expect(screen.queryByText("ATM-SMG-001")).not.toBeInTheDocument();
  });
});

// ===== Test 3: Navigation between all screens =====

describe("Navigation between screens", () => {
  it("can navigate to each screen via sidebar links", async () => {
    const user = userEvent.setup();
    renderApp("/orders");

    // Login first
    await waitFor(() => {
      expect(screen.getByLabelText("Nama pengguna")).toBeInTheDocument();
    });
    await performLogin(user, "gardanet.admin", "password123");

    // Wait for initial page (orders)
    await waitFor(() => {
      expect(screen.getByRole("heading", { name: /cit orders/i })).toBeInTheDocument();
    });

    // Navigate to Invoices
    const invoicesLink = screen.getByRole("link", { name: /invoices/i });
    await user.click(invoicesLink);
    await waitFor(() => {
      expect(screen.getByRole("heading", { name: /invoices/i })).toBeInTheDocument();
    });

    // Navigate to Replenishment Schedule
    const scheduleLink = screen.getByRole("link", { name: /replenishment schedule/i });
    await user.click(scheduleLink);
    await waitFor(() => {
      expect(screen.getByRole("heading", { name: /replenishment schedule/i })).toBeInTheDocument();
    });

    // Navigate to DSR Monitor
    const dsrLink = screen.getByRole("link", { name: /dsr/i });
    await user.click(dsrLink);
    await waitFor(() => {
      expect(screen.getByRole("heading", { name: /dsr monitor/i, level: 1 })).toBeInTheDocument();
    });

    // Navigate to Notifications
    const notificationsLink = screen.getByRole("link", { name: /notifications/i });
    await user.click(notificationsLink);
    await waitFor(() => {
      expect(screen.getByRole("heading", { name: /notifications/i })).toBeInTheDocument();
    });

    // Navigate back to CIT Orders
    const ordersLink = screen.getByRole("link", { name: /cit orders/i });
    await user.click(ordersLink);
    await waitFor(() => {
      expect(screen.getByRole("heading", { name: /cit orders/i })).toBeInTheDocument();
    });
  });
});

// ===== Test 4: Error boundary fallback rendering =====

describe("Error boundary fallback", () => {
  function ThrowingComponent(): never {
    throw new Error("Test error");
  }

  function renderWithShell(PageComponent: () => React.ReactNode) {
    const rootRoute = createRootRoute({
      component: () => (
        <QueryClientProvider client={queryClient}>
          <AuthProvider>
            <Outlet />
          </AuthProvider>
        </QueryClientProvider>
      ),
    });
    // Pre-authenticate by stubbing refresh to succeed for this subtree.
    const shellRoute = createRoute({
      id: "_shell",
      getParentRoute: () => rootRoute,
      component: () => (
        <AppShell>
          <Outlet />
        </AppShell>
      ),
    });
    const pageRoute = createRoute({
      path: "/",
      getParentRoute: () => shellRoute,
      component: PageComponent,
    });
    const routeTree = rootRoute.addChildren([shellRoute.addChildren([pageRoute])]);
    const router = createRouter({
      routeTree,
      history: createMemoryHistory({ initialEntries: ["/"] }),
    });
    return render(<RouterProvider router={router} />);
  }

  it('catches errors and shows "Terjadi kesalahan" fallback', async () => {
    const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    // Authenticated session so AppShell renders.
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

    renderWithShell(ThrowingComponent);

    await waitFor(() => {
      expect(screen.getByText("Terjadi kesalahan")).toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: /muat ulang/i })).toBeInTheDocument();

    consoleSpy.mockRestore();
  });

  it('"Muat ulang" button resets the error boundary', async () => {
    const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    const user = userEvent.setup();
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

    let shouldThrow = true;
    function ConditionalThrower() {
      if (shouldThrow) throw new Error("Test error");
      return <p>Content recovered</p>;
    }

    renderWithShell(ConditionalThrower);

    await waitFor(() => {
      expect(screen.getByText("Terjadi kesalahan")).toBeInTheDocument();
    });

    shouldThrow = false;
    const reloadButton = screen.getByRole("button", { name: /muat ulang/i });
    await user.click(reloadButton);

    await waitFor(() => {
      expect(screen.getByText("Content recovered")).toBeInTheDocument();
    });
    expect(screen.queryByText("Terjadi kesalahan")).not.toBeInTheDocument();

    consoleSpy.mockRestore();
  });
});
