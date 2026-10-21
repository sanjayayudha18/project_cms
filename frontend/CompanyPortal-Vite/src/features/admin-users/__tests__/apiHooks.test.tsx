/**
 * fe-coverage C2: admin-users api + hooks. Only the HTTP client is mocked, so the real
 * fetchers (paths, query string) and the real TanStack hooks (enabled, invalidation) run.
 */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const client = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn() }));
vi.mock("@/lib/api/client", () => ({ api: client }));

import { createUser, disableUser, enableUser, getUser, listUsers, updateUser } from "../api";
import {
  adminUserKeys,
  useCreateUser,
  useDisableUser,
  useEnableUser,
  useUpdateUser,
  useUser,
  useUsersList,
  useVendorOptions,
} from "../hooks";
import type { CreateUserPayload, UpdateUserPayload } from "../types";

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

type Mutation = { mutateAsync: (v: unknown) => Promise<unknown> };

const createPayload = {
  username: "budi",
  full_name: "Budi",
  email: "budi@cimb.local",
  role: "ATM-USER",
  auth_source: "ldap",
  vendor_id: null,
} as unknown as CreateUserPayload;

const updatePayload = {
  full_name: "Budi S",
  email: "budi@cimb.local",
  role: "ATM-SPV",
  vendor_id: null,
} as unknown as UpdateUserPayload;

beforeEach(() => {
  client.get.mockReset();
  client.post.mockReset();
  client.put.mockReset();
});

describe("admin-users api", () => {
  it("listUsers always sends page + page_size and only the filters that are set", async () => {
    client.get.mockResolvedValue({ data: { users: [], total: 0 } });

    await listUsers({ page: 1, page_size: 25 });
    await listUsers({
      page: 2,
      page_size: 10,
      status: "disabled",
      vendor_id: 0,
      role: "ADMIN",
      q: "bu di",
    } as never);

    expect(client.get.mock.calls.map((c) => c[0])).toEqual([
      "/admin/users?page=1&page_size=25",
      "/admin/users?page=2&page_size=10&status=disabled&vendor_id=0&role=ADMIN&q=bu+di",
    ]);
  });

  it("get / create / update / disable / enable hit the user endpoints", async () => {
    client.get.mockResolvedValue({ data: { id: 3 } });
    client.post.mockResolvedValue({ data: { id: 3, message: "ok" } });
    client.put.mockResolvedValue({ data: { id: 3 } });

    await getUser(3);
    await createUser(createPayload);
    await updateUser(3, updatePayload);
    await disableUser(3);
    await enableUser(3);

    expect(client.get).toHaveBeenCalledWith("/admin/users/3");
    expect(client.put).toHaveBeenCalledWith("/admin/users/3", updatePayload);
    expect(client.post.mock.calls).toEqual([
      ["/admin/users", createPayload],
      ["/admin/users/3/disable"],
      ["/admin/users/3/enable"],
    ]);
  });
});

describe("admin-users hooks", () => {
  it("useUsersList loads the requested page", async () => {
    client.get.mockResolvedValue({ data: { users: [{ id: 1 }], total: 1 } });
    const { result } = setup(() => useUsersList({ page: 1, page_size: 25 }));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual({ users: [{ id: 1 }], total: 1 });
  });

  it("useVendorOptions reads /admin/users/vendor-options (APPACCESS-safe lookup)", async () => {
    const options = { vendors: [{ id: 1, code: "ABC", name: "Abacus" }], branches: [] };
    client.get.mockResolvedValue({ data: options });
    const { result } = setup(() => useVendorOptions());

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(client.get).toHaveBeenCalledWith("/admin/users/vendor-options");
    expect(result.current.data).toEqual(options);
  });

  it("useUser is idle without an id and loads with one", async () => {
    const idle = setup(() => useUser(null));
    expect(idle.result.current.fetchStatus).toBe("idle");

    client.get.mockResolvedValue({ data: { id: 8 } });
    const loaded = setup(() => useUser(8));
    await waitFor(() => expect(loaded.result.current.isSuccess).toBe(true));
    expect(client.get).toHaveBeenCalledWith("/admin/users/8");
  });

  it.each([
    ["useCreateUser", () => useCreateUser(), createPayload],
    ["useUpdateUser", () => useUpdateUser(), { id: 3, payload: updatePayload }],
    ["useDisableUser", () => useDisableUser(), 3],
    ["useEnableUser", () => useEnableUser(), 3],
  ])("%s refreshes the users list after success", async (_name, hook, variables) => {
    client.post.mockResolvedValue({ data: { id: 3, message: "ok" } });
    client.put.mockResolvedValue({ data: { id: 3 } });
    const { result, invalidate } = setup(hook as () => Mutation);

    await act(async () => {
      await result.current.mutateAsync(variables);
    });

    expect(invalidate).toHaveBeenCalledWith({ queryKey: adminUserKeys.all });
  });

  it("a failed mutation does not refresh the list and exposes the API error", async () => {
    client.post.mockRejectedValue({ status: 409, message: "Username sudah dipakai" });
    const { result, invalidate } = setup(() => useCreateUser());

    await act(async () => {
      await result.current.mutateAsync(createPayload).catch(() => {});
    });

    await waitFor(() => expect(result.current.error?.message).toBe("Username sudah dipakai"));
    expect(invalidate).not.toHaveBeenCalled();
  });
});
