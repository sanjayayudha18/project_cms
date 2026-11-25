import { usePermissionStore } from "@/lib/auth/permissionStore";
import type { DbRole } from "@/lib/auth/store";
import { useAuthStore } from "@/lib/auth/store";
import { getVisibleNavForRole as getVisibleNav } from "@/test/visibleNavForRole";
import {
  RouterProvider,
  createMemoryHistory,
  createRouter,
  isRedirect,
} from "@tanstack/react-router";
import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { routeTree } from "../routeTree";
import { ForbiddenError } from "./_protected";
import { exportPreviewRoute } from "./exports";
import { adminExportsRoute } from "./settings/admin/exports";

type Guard = () => Promise<void>;
const options = (r: unknown) => (r as { options: Record<string, unknown> }).options;
const previewGuard = options(exportPreviewRoute).beforeLoad as Guard;

function as(role: DbRole, keys: string[] = ["exports"]) {
  // No-op load: signing in triggers the store's auto-load, which would overwrite these keys.
  usePermissionStore.setState({ keys: new Set(keys), status: "ready", load: async () => {} });
  useAuthStore.setState({
    user: {
      id: 1,
      username: "u",
      fullName: "U",
      email: "u@x",
      role,
      isKaryawan: true,
      vendorId: null,
    },
    accessToken: "t",
    isAuthenticated: true,
    isAuthLoading: false,
  });
}

describe("/exports/$dataset route", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("is mounted at /exports/$dataset", () =>
    expect(options(exportPreviewRoute).path).toBe("/exports/$dataset"));

  it.each<DbRole>(["ADMIN", "ADMIN_PARAM"])("allows %s", async (role) => {
    as(role);
    await expect(previewGuard()).resolves.toBeUndefined();
  });

  it.each<DbRole>(["ATM-USER", "ATM-SPV", "BRANCH-ATM-SPV", "ACM-USER"])(
    "forbids %s",
    async (role) => {
      as(role);
      await expect(previewGuard()).rejects.toBeInstanceOf(ForbiddenError);
    },
  );

  it("forbids ADMIN without the exports key", async () => {
    as("ADMIN", []);
    await expect(previewGuard()).rejects.toBeInstanceOf(ForbiddenError);
  });

  it("keeps only filter keys from the search", () => {
    const validate = options(exportPreviewRoute).validateSearch as (
      s: Record<string, unknown>,
    ) => Record<string, unknown>;
    expect(validate({ status: "all", vendorId: 3, junk: "x" })).toEqual({
      status: "all",
      vendorId: 3,
    });
  });

  it("renders UnknownDatasetState with a link back to /exports for an unknown slug", async () => {
    // The app shell reads matchMedia (jsdom lacks it).
    vi.stubGlobal("matchMedia", () => ({
      matches: false,
      addEventListener: () => {},
      removeEventListener: () => {},
      addListener: () => {},
      removeListener: () => {},
    }));
    as("ADMIN");
    // Root runs initialize() on mount, which would drop the session; keep the signed-in user.
    useAuthStore.setState({ initialize: async () => {} });
    const router = createRouter({
      routeTree,
      history: createMemoryHistory({ initialEntries: ["/exports/nope"] }),
    });
    render(<RouterProvider router={router} />);
    // Mounts the whole app tree; the default 1s wait trips under full-suite coverage load.
    expect(
      await screen.findByText("Data ekspor tidak dikenal.", {}, { timeout: 10_000 }),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Kembali ke Export Data" })).toHaveAttribute(
      "href",
      "/exports",
    );
  });
});

describe("/settings/admin/exports redirect", () => {
  it("redirects to /exports", () => {
    let thrown: unknown;
    try {
      (options(adminExportsRoute).beforeLoad as () => void)();
    } catch (e) {
      thrown = e;
    }
    expect(isRedirect(thrown)).toBe(true);
    expect((thrown as { options: { to: string } }).options.to).toBe("/exports");
  });

  it("has no component", () => expect(options(adminExportsRoute).component).toBeUndefined());
});

describe("Export Data is not in the sidebar", () => {
  const hasItem = (role: DbRole) => getVisibleNav(role).all.some((i) => i.id === "exports");

  it.each<DbRole>([
    "ADMIN",
    "ADMIN_PARAM",
    "ATM-USER",
    "ATM-SPV",
    "BRANCH-ATM-USER",
    "BRANCH-ATM-SPV",
    "ACM-USER",
    "BRANCH-USER",
    "VENDOR-USER",
  ])("is not in the sidebar for %s", (role) => expect(hasItem(role)).toBe(false));
});
