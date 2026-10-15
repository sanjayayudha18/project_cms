/**
 * fe-coverage C3: admin-vendors api + hooks + DSR-location-map hooks, table-driven.
 * Only the HTTP client is mocked, so the real fetchers (paths, query strings) and the
 * real TanStack hooks (invalidation of the vendor tree + master-data pending badges) run.
 * Every create/update/disable/enable here is a maker-checker *submission* (202): the
 * hooks must refresh the pending badges, not assume the row changed.
 */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const client = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn() }));
vi.mock("@/lib/api/client", () => ({ api: client }));

import {
  useDsrLocationMapMutation,
  useDsrLocationMaps,
  useUnmappedDsrLocations,
} from "../dsrLocationMaps";
import * as h from "../hooks";

type Method = keyof typeof client;
type Query = { isSuccess: boolean; data: unknown; fetchStatus: string };
type Mutation = {
  mutateAsync: (v: unknown) => Promise<unknown>;
  error: { message: string } | null;
};

function setup<T>(hook: () => T) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const invalidate = vi.spyOn(queryClient, "invalidateQueries");
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  return { ...renderHook(hook, { wrapper }), invalidate };
}

const invalidatedKeys = (spy: { mock: { calls: unknown[][] } }) =>
  spy.mock.calls.map((c) => (c[0] as { queryKey: unknown }).queryKey);

const ACCEPTED = { change_request_id: 77, status: "pending" };
const V = 42; // vendor id
const P = { name: "x" }; // payload placeholder; the client only forwards it

beforeEach(() => {
  client.get.mockReset();
  client.post.mockReset();
  client.put.mockReset();
});

describe("admin-vendors queries", () => {
  it.each<[string, () => Query, string, unknown, unknown]>([
    [
      "vendors list (defaults only)",
      () => h.useVendorsList({ page: 1, page_size: 25 }) as unknown as Query,
      "/admin/vendors?page=1&page_size=25",
      { vendors: [], total: 0 },
      { vendors: [], total: 0 },
    ],
    [
      "vendors list (filters)",
      () =>
        h.useVendorsList({
          page: 2,
          page_size: 10,
          status: "inactive",
          q: "gar da",
        } as never) as unknown as Query,
      "/admin/vendors?page=2&page_size=10&status=inactive&q=gar+da",
      { vendors: [{ id: 1 }], total: 1 },
      { vendors: [{ id: 1 }], total: 1 },
    ],
    [
      "vendor detail",
      () => h.useVendor(V) as unknown as Query,
      `/admin/vendors/${V}`,
      { id: V },
      { id: V },
    ],
    [
      "children of one kind (unwrapped)",
      () => h.useVendorChildren(V, "vaults" as never) as unknown as Query,
      `/admin/vendors/${V}/vaults?page=1&page_size=100&status=all`,
      { vaults: [{ id: 5 }] },
      [{ id: 5 }],
    ],
    [
      "children missing in response → []",
      () => h.useVendorChildren(V, "pics" as never) as unknown as Query,
      `/admin/vendors/${V}/pics?page=1&page_size=100&status=all`,
      {},
      [],
    ],
    [
      "branch detail",
      () => h.useVendorBranch(V, 3) as unknown as Query,
      `/admin/vendors/${V}/branches/3`,
      { id: 3 },
      { id: 3 },
    ],
    [
      "branches list",
      () =>
        h.useVendorBranches(V, {
          page: 1,
          page_size: 20,
          status: "all",
          q: "jkt",
        } as never) as unknown as Query,
      `/admin/vendors/${V}/branches?page=1&page_size=20&status=all&q=jkt`,
      { branches: [] },
      { branches: [] },
    ],
    [
      "vaults list by branch",
      () =>
        h.useVendorVaults(V, { page: 1, page_size: 20, branch_id: 3 } as never) as unknown as Query,
      `/admin/vendors/${V}/vaults?page=1&page_size=20&branch_id=3`,
      { vaults: [] },
      { vaults: [] },
    ],
    [
      "PICs list, vendor-wide only",
      () =>
        h.useVendorPics(V, {
          page: 1,
          page_size: 20,
          branch_id: 3,
          vendor_wide_only: true,
        } as never) as unknown as Query,
      `/admin/vendors/${V}/pics?page=1&page_size=20&branch_id=3&vendor_wide_only=true`,
      { pics: [] },
      { pics: [] },
    ],
    [
      "packages list by branch",
      () =>
        h.useVendorPackages(V, {
          page: 1,
          page_size: 20,
          branch_id: 3,
        } as never) as unknown as Query,
      `/admin/vendors/${V}/packages?page=1&page_size=20&branch_id=3`,
      { packages: [] },
      { packages: [] },
    ],
    [
      "package prices list (all filters)",
      () =>
        h.useVendorPackagePrices(V, {
          page: 1,
          page_size: 20,
          status: "active",
          package: "PAKET 3",
          machine_group: "CRM",
          price_class: "A",
        } as never) as unknown as Query,
      `/admin/vendors/${V}/package-prices?page=1&page_size=20&status=active&package=PAKET+3&machine_group=CRM&price_class=A`,
      { prices: [] },
      { prices: [] },
    ],
    [
      "branch ATMs",
      () => h.useBranchATMs(V, 3, { page: 2, page_size: 50 } as never) as unknown as Query,
      `/admin/vendors/${V}/branches/3/atms?page=2&page_size=50`,
      { atms: [] },
      { atms: [] },
    ],
  ])("%s", async (_name, hook, path, body, expected) => {
    client.get.mockResolvedValue({ data: body });
    const { result } = setup(hook);

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(client.get).toHaveBeenCalledWith(path);
    expect(result.current.data).toEqual(expected);
  });

  it("useVendor stays idle without an id", () => {
    const { result } = setup(() => h.useVendor(null));

    expect(result.current.fetchStatus).toBe("idle");
    expect(client.get).not.toHaveBeenCalled();
  });
});

const m = (hook: () => unknown) => hook as () => Mutation;

describe("admin-vendors maker-checker submissions", () => {
  it.each<[string, () => Mutation, unknown, [Method, ...unknown[]]]>([
    ["create vendor", m(h.useCreateVendor), P, ["post", "/admin/vendors", P]],
    [
      "update vendor",
      m(h.useUpdateVendor),
      { id: V, payload: P },
      ["put", `/admin/vendors/${V}`, P],
    ],
    ["disable vendor", m(h.useDisableVendor), V, ["post", `/admin/vendors/${V}/disable`]],
    ["enable vendor", m(h.useEnableVendor), V, ["post", `/admin/vendors/${V}/enable`]],
    [
      "create branch",
      m(() => h.useCreateVendorBranch(V)),
      P,
      ["post", `/admin/vendors/${V}/branches`, P],
    ],
    [
      "update branch",
      m(() => h.useUpdateVendorBranch(V)),
      { branchId: 3, payload: P },
      ["put", `/admin/vendors/${V}/branches/3`, P],
    ],
    [
      "disable branch",
      m(() => h.useDisableVendorBranch(V)),
      3,
      ["post", `/admin/vendors/${V}/branches/3/disable`],
    ],
    [
      "enable branch",
      m(() => h.useEnableVendorBranch(V)),
      3,
      ["post", `/admin/vendors/${V}/branches/3/enable`],
    ],
    [
      "create vault",
      m(() => h.useCreateVendorVault(V)),
      P,
      ["post", `/admin/vendors/${V}/vaults`, P],
    ],
    [
      "update vault",
      m(() => h.useUpdateVendorVault(V)),
      { vaultId: 5, payload: P },
      ["put", `/admin/vendors/${V}/vaults/5`, P],
    ],
    [
      "disable vault",
      m(() => h.useDisableVendorVault(V)),
      5,
      ["post", `/admin/vendors/${V}/vaults/5/disable`],
    ],
    [
      "enable vault",
      m(() => h.useEnableVendorVault(V)),
      5,
      ["post", `/admin/vendors/${V}/vaults/5/enable`],
    ],
    ["create PIC", m(() => h.useCreateVendorPic(V)), P, ["post", `/admin/vendors/${V}/pics`, P]],
    [
      "update PIC",
      m(() => h.useUpdateVendorPic(V)),
      { picId: 6, payload: P },
      ["put", `/admin/vendors/${V}/pics/6`, P],
    ],
    [
      "disable PIC",
      m(() => h.useDisableVendorPic(V)),
      6,
      ["post", `/admin/vendors/${V}/pics/6/disable`],
    ],
    [
      "enable PIC",
      m(() => h.useEnableVendorPic(V)),
      6,
      ["post", `/admin/vendors/${V}/pics/6/enable`],
    ],
    [
      "create package",
      m(() => h.useCreateVendorPackage(V)),
      P,
      ["post", `/admin/vendors/${V}/packages`, P],
    ],
    [
      "update package",
      m(() => h.useUpdateVendorPackage(V)),
      { packageId: 7, payload: P },
      ["put", `/admin/vendors/${V}/packages/7`, P],
    ],
    [
      "disable package",
      m(() => h.useDisableVendorPackage(V)),
      7,
      ["post", `/admin/vendors/${V}/packages/7/disable`],
    ],
    [
      "create package price",
      m(() => h.useCreateVendorPackagePrice(V)),
      P,
      ["post", `/admin/vendors/${V}/package-prices`, P],
    ],
    [
      "update package price",
      m(() => h.useUpdateVendorPackagePrice(V)),
      { priceId: 8, payload: P },
      ["put", `/admin/vendors/${V}/package-prices/8`, P],
    ],
    [
      "disable package price",
      m(() => h.useDisableVendorPackagePrice(V)),
      8,
      ["post", `/admin/vendors/${V}/package-prices/8/disable`],
    ],
  ])("%s → endpoint, then vendor tree + pending badges refresh", async (_n, hook, vars, call) => {
    client.post.mockResolvedValue({ data: ACCEPTED });
    client.put.mockResolvedValue({ data: ACCEPTED });
    const { result, invalidate } = setup(hook);

    let returned: unknown;
    await act(async () => {
      returned = await result.current.mutateAsync(vars);
    });

    const [method, ...args] = call;
    expect(client[method]).toHaveBeenCalledWith(...args);
    expect(returned).toEqual(ACCEPTED);
    expect(invalidatedKeys(invalidate)).toEqual([["admin-vendors"], ["master-data", "pending"]]);
  });

  it("a rejected submission (e.g. 409 pending change exists) refreshes nothing", async () => {
    client.post.mockRejectedValue({ status: 409, message: "Masih ada perubahan pending" });
    const { result, invalidate } = setup(() => h.useDisableVendor());

    await act(async () => {
      await result.current.mutateAsync(V).catch(() => {});
    });

    await waitFor(() => expect(result.current.error?.message).toBe("Masih ada perubahan pending"));
    expect(invalidate).not.toHaveBeenCalled();
  });
});

describe("DSR location → vault maps", () => {
  it("lists all maps and the unmapped DSR locations", async () => {
    client.get.mockImplementation(async (path: string) =>
      path.endsWith("/unmapped")
        ? { data: { locations: [{ dsr_location: "GUDANG A", last_report_date: null }] } }
        : { data: { maps: [{ id: 1, dsr_location: "VAULT JKT" }] } },
    );

    const maps = setup(() => useDsrLocationMaps(V));
    const unmapped = setup(() => useUnmappedDsrLocations(V));

    await waitFor(() => expect(maps.result.current.isSuccess).toBe(true));
    await waitFor(() => expect(unmapped.result.current.isSuccess).toBe(true));
    expect(client.get).toHaveBeenCalledWith(`/admin/vendors/${V}/dsr-location-maps?status=all`);
    expect(client.get).toHaveBeenCalledWith(`/admin/vendors/${V}/dsr-location-maps/unmapped`);
    expect(maps.result.current.data).toEqual([{ id: 1, dsr_location: "VAULT JKT" }]);
    expect(unmapped.result.current.data).toEqual([
      { dsr_location: "GUDANG A", last_report_date: null },
    ]);
  });

  it.each<[string, unknown, [Method, ...unknown[]]]>([
    [
      "create",
      { op: "create", dsr_location: "GUDANG A", vendor_vault_id: 5 },
      [
        "post",
        `/admin/vendors/${V}/dsr-location-maps`,
        { dsr_location: "GUDANG A", vendor_vault_id: 5 },
      ],
    ],
    [
      "update",
      { op: "update", id: 1, vendor_vault_id: 9 },
      ["put", `/admin/vendors/${V}/dsr-location-maps/1`, { vendor_vault_id: 9 }],
    ],
    [
      "disable",
      { op: "disable", id: 1 },
      ["post", `/admin/vendors/${V}/dsr-location-maps/1/disable`],
    ],
    ["enable", { op: "enable", id: 1 }, ["post", `/admin/vendors/${V}/dsr-location-maps/1/enable`]],
  ])(
    "%s submits a change request and refreshes the maps + pending badges",
    async (_n, mut, call) => {
      client.post.mockResolvedValue({ data: ACCEPTED });
      client.put.mockResolvedValue({ data: ACCEPTED });
      const { result, invalidate } = setup(() => useDsrLocationMapMutation(V));

      let returned: unknown;
      await act(async () => {
        returned = await result.current.mutateAsync(mut as never);
      });

      const [method, ...args] = call;
      expect(client[method]).toHaveBeenCalledWith(...args);
      expect(returned).toEqual(ACCEPTED);
      expect(invalidatedKeys(invalidate)).toEqual([
        ["admin-vendors", "children", V, "dsr-location-maps"],
        ["master-data", "pending"],
      ]);
    },
  );
});
