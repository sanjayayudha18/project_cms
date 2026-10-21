import type { AuthUser, DbRole } from "@/lib/types";
import { VENDOR_ROLES, isVendorRole, partyRolesOf, partyScopeLabel } from "@/lib/vendorRoles";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useContext } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// vendor-party-scope task 11: portal guard for the 3 vendor roles, scope line, Realisasi gating.

// Controllable auth state for the shell / page tests. The guard tests below bypass this mock
// by reading the real AuthContext directly.
let mockUser: AuthUser | null = null;
vi.mock("@/features/auth/useAuth", () => ({
  useAuth: () => ({ state: { user: mockUser }, logout: vi.fn() }),
  useAuthRefresh: () => vi.fn(),
}));
vi.mock("@/features/notifications/useNotifications", () => ({
  useUnreadCount: () => ({ data: 0 }),
}));
const useRealizationsMock = vi.fn();
vi.mock("@/features/orders/useOrders", () => ({
  useRealizations: (...args: unknown[]) => useRealizationsMock(...args),
}));

import { AppShell } from "@/app/AppShell";
import { AuthContext, AuthProvider } from "@/features/auth/AuthContext";
import { RealizationsGate } from "@/features/realizations/RealizationsPage";

function makeUser(role: DbRole, over: Partial<AuthUser> = {}): AuthUser {
  return {
    id: 1,
    username: "v",
    fullName: "Budi",
    email: "b@v.com",
    role,
    isKaryawan: false,
    vendorId: 4,
    vendorName: "Gardanet",
    vendorBranchId: null,
    vendorBranchName: null,
    ...over,
  };
}

function renderShell(path = "/orders") {
  const root = createRootRoute({
    component: () => (
      <QueryClientProvider client={new QueryClient()}>
        <AppShell>
          <Outlet />
        </AppShell>
      </QueryClientProvider>
    ),
  });
  const page = (p: string) =>
    createRoute({ path: p, getParentRoute: () => root, component: () => <span>page {p}</span> });
  const realizations = createRoute({
    path: "/realizations",
    getParentRoute: () => root,
    component: RealizationsGate,
  });
  const tree = root.addChildren([
    page("/orders"),
    page("/dsr"),
    page("/notifications"),
    realizations,
  ]);
  const router = createRouter({
    routeTree: tree,
    history: createMemoryHistory({ initialEntries: [path] }),
  });
  return render(<RouterProvider router={router} />);
}

// ─── vendorRoles helpers ──────────────────────────────────────────────────────

describe("vendorRoles", () => {
  it("accepts exactly the three vendor roles", () => {
    for (const r of VENDOR_ROLES) expect(isVendorRole(r)).toBe(true);
    expect(VENDOR_ROLES).toHaveLength(3);
    for (const r of ["ATM-USER", "ADMIN", "", "vendor-user"]) expect(isVendorRole(r)).toBe(false);
  });

  it("maps each role to its parties and scope label (Req 8.4)", () => {
    expect(partyRolesOf("VENDOR-USER")).toEqual(["replenish", "vault"]);
    expect(partyRolesOf("VENDOR-ATM-USER")).toEqual(["replenish"]);
    expect(partyRolesOf("VENDOR-VAULT-USER")).toEqual(["vault"]);
    expect(partyScopeLabel("VENDOR-USER")).toBe("Pengisian ATM + Penyedia uang");
    expect(partyScopeLabel("VENDOR-ATM-USER")).toBe("Pengisian ATM");
    expect(partyScopeLabel("VENDOR-VAULT-USER")).toBe("Penyedia uang");
  });
});

// ─── AuthContext guard (init + refresh) ───────────────────────────────────────

const mockFetch = vi.fn();

function sessionResponse(role: string) {
  return new Response(
    JSON.stringify({
      access_token: "tok",
      user: {
        id: 7,
        username: "u",
        full_name: "Init Vendor",
        email: "i@v.com",
        role,
        is_karyawan: false,
        vendor_id: 42,
      },
    }),
    { status: 200 },
  );
}

function mockSession(role: string, me?: Record<string, unknown> | "fail") {
  mockFetch.mockImplementation(async (url: string) => {
    if (url.includes("/refresh")) return sessionResponse(role);
    if (url.includes("/me")) {
      if (me === "fail") throw new Error("network");
      return new Response(JSON.stringify(me ?? {}), { status: 200 });
    }
    return new Response(null, { status: 404 });
  });
}

function refreshCalls() {
  return mockFetch.mock.calls.filter(([u]) => String(u).includes("/refresh")).length;
}

function Probe() {
  const ctx = useContext(AuthContext);
  if (!ctx) return null;
  const { state, refreshToken } = ctx;
  const u = state.user;
  return (
    <div>
      <span data-testid="loading">{String(state.isAuthLoading)}</span>
      <span data-testid="authed">{String(state.isAuthenticated)}</span>
      <span data-testid="vendor">{u?.vendorName ?? "none"}</span>
      <span data-testid="branch">{u?.vendorBranchName ?? "none"}</span>
      <button type="button" onClick={() => void refreshToken()}>
        refresh
      </button>
    </div>
  );
}

function renderProbe() {
  return render(
    <AuthProvider>
      <Probe />
    </AuthProvider>,
  );
}

describe("AuthContext vendor-role guard", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", mockFetch);
    mockFetch.mockReset();
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  for (const role of VENDOR_ROLES) {
    it(`init accepts ${role} and loads the /me vendor context`, async () => {
      mockSession(role, {
        vendor_name: "Gardanet",
        vendor_branch_id: 9,
        vendor_branch_name: "Jakarta",
      });
      renderProbe();
      await waitFor(() => expect(screen.getByTestId("loading")).toHaveTextContent("false"));
      expect(screen.getByTestId("authed")).toHaveTextContent("true");
      expect(screen.getByTestId("vendor")).toHaveTextContent("Gardanet");
      expect(screen.getByTestId("branch")).toHaveTextContent("Jakarta");
    });
  }

  it("init rejects an internal role", async () => {
    mockSession("ATM-USER");
    renderProbe();
    await waitFor(() => expect(screen.getByTestId("loading")).toHaveTextContent("false"));
    expect(screen.getByTestId("authed")).toHaveTextContent("false");
  });

  it("keeps the session with null vendor context when /me fails", async () => {
    mockSession("VENDOR-ATM-USER", "fail");
    renderProbe();
    await waitFor(() => expect(screen.getByTestId("authed")).toHaveTextContent("true"));
    expect(screen.getByTestId("vendor")).toHaveTextContent("none");
  });

  for (const role of VENDOR_ROLES) {
    it(`refresh keeps ${role} signed in`, async () => {
      mockSession(role, { vendor_name: "Gardanet" });
      renderProbe();
      await waitFor(() => expect(screen.getByTestId("authed")).toHaveTextContent("true"));
      const before = refreshCalls();
      await userEvent.click(screen.getByRole("button", { name: "refresh" }));
      await waitFor(() => expect(refreshCalls()).toBe(before + 1));
      expect(screen.getByTestId("authed")).toHaveTextContent("true");
    });
  }

  it("refresh drops the session when the role turned internal", async () => {
    mockSession("VENDOR-USER", { vendor_name: "Gardanet" });
    renderProbe();
    await waitFor(() => expect(screen.getByTestId("authed")).toHaveTextContent("true"));
    mockSession("ADMIN");
    await userEvent.click(screen.getByRole("button", { name: "refresh" }));
    await waitFor(() => expect(screen.getByTestId("authed")).toHaveTextContent("false"));
  });
});

// ─── Shell: scope line + nav + Realisasi page ─────────────────────────────────

describe("AppShell scope line and Realisasi gating", () => {
  beforeEach(() => {
    useRealizationsMock.mockReset();
    useRealizationsMock.mockReturnValue({
      data: { items: [], total: 0 },
      isLoading: false,
      isError: false,
    });
  });

  it("shows vendor-wide scope and the Realisasi menu for VENDOR-USER", async () => {
    mockUser = makeUser("VENDOR-USER");
    renderShell();
    expect(await screen.findByTestId("scope-line")).toHaveTextContent(
      "Gardanet · Semua cabang · Pengisian ATM + Penyedia uang",
    );
    expect(screen.getByRole("link", { name: /Realisasi/ })).toBeInTheDocument();
  });

  it("shows the cabang for a branch-scoped ATM user", async () => {
    mockUser = makeUser("VENDOR-ATM-USER", { vendorBranchId: 9, vendorBranchName: "Jakarta" });
    renderShell();
    expect(await screen.findByTestId("scope-line")).toHaveTextContent(
      "Gardanet · Jakarta · Pengisian ATM",
    );
    expect(screen.getByRole("link", { name: /Realisasi/ })).toBeInTheDocument();
  });

  it("hides the Realisasi menu for VENDOR-VAULT-USER", async () => {
    mockUser = makeUser("VENDOR-VAULT-USER");
    renderShell();
    expect(await screen.findByTestId("scope-line")).toHaveTextContent("Penyedia uang");
    expect(screen.queryByRole("link", { name: /Realisasi/ })).not.toBeInTheDocument();
  });

  it("shows the unavailable notice (no list query) when a vault user opens /realizations", async () => {
    mockUser = makeUser("VENDOR-VAULT-USER");
    renderShell("/realizations");
    expect(await screen.findByText("Tidak tersedia untuk peran Anda")).toBeInTheDocument();
    expect(useRealizationsMock).not.toHaveBeenCalled();
  });

  it("renders the Realisasi page for an ATM user", async () => {
    mockUser = makeUser("VENDOR-ATM-USER");
    renderShell("/realizations");
    expect(await screen.findByRole("heading", { name: "Realisasi" })).toBeInTheDocument();
    expect(screen.queryByText("Tidak tersedia untuk peran Anda")).not.toBeInTheDocument();
  });
});
