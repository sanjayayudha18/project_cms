/**
 * Mutation-hook invalidation tests (design.md "useCreateRole() (invalidates
 * roleKeys.roles())", "useUpdateRolePermissions() (invalidates
 * roleKeys.roles())"). Real TanStack Query, real QueryClient; only `../api`
 * is mocked so no network runs. Mirrors rbac-settings/__tests__/hooks.test.tsx.
 */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mockApi = vi.hoisted(() => ({
  getRoles: vi.fn(),
  getCatalog: vi.fn(),
  createRole: vi.fn(),
  updateRolePermissions: vi.fn(),
}));

vi.mock("../api", () => mockApi);

import {
  roleKeys,
  useCatalogForPortal,
  useCreateRole,
  useRolesForPortal,
  useUpdateRolePermissions,
} from "../hooks/useRoleQueries";

function queryWrapper() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("portal-filtered reads", () => {
  it("useRolesForPortal keeps only roles of that portal", async () => {
    mockApi.getRoles.mockResolvedValue({
      roles: [
        { id: 1, role: "ADMIN", portal: "main" },
        { id: 2, role: "VENDOR-USER", portal: "vendor" },
        { id: 3, role: "EXT-CLIENT", portal: "service" },
      ],
    });
    const { result } = renderHook(() => useRolesForPortal("vendor"), { wrapper: queryWrapper() });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.map((r) => r.role)).toEqual(["VENDOR-USER"]);
  });

  it("useCatalogForPortal keeps only catalog entries of that portal", async () => {
    mockApi.getCatalog.mockResolvedValue({
      catalog: [
        { id: 1, key: "dashboard", portal: "main" },
        { id: 2, key: "vendor-orders", portal: "vendor" },
      ],
    });
    const { result } = renderHook(() => useCatalogForPortal("main"), { wrapper: queryWrapper() });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.map((e) => e.key)).toEqual(["dashboard"]);
  });
});

function renderMutation<THookResult extends { mutate: (v: never) => unknown }>(
  hook: () => THookResult,
) {
  const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
  const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  return { ...renderHook(hook, { wrapper }), invalidateSpy };
}

beforeEach(() => {
  for (const fn of Object.values(mockApi)) {
    fn.mockReset();
  }
});

describe("role-management mutation hooks invalidate roleKeys.roles() on success", () => {
  it("useCreateRole invalidates the roles key", async () => {
    mockApi.createRole.mockResolvedValue({ id: 11, role: "AUDITOR", description: null });
    const { result, invalidateSpy } = renderMutation(useCreateRole);

    await act(async () => {
      result.current.mutate({ role: "AUDITOR", description: "" } as never);
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: roleKeys.roles() });
    expect(mockApi.createRole.mock.calls[0][0]).toEqual({ role: "AUDITOR", description: "" });
  });

  it("useUpdateRolePermissions invalidates the roles key and calls the api with (id, ids)", async () => {
    mockApi.updateRolePermissions.mockResolvedValue({ role_id: 7, permissions: [1, 2] });
    const { result, invalidateSpy } = renderMutation(useUpdateRolePermissions);

    await act(async () => {
      result.current.mutate({ id: 7, menuFeatureIds: [1, 2] } as never);
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: roleKeys.roles() });
    expect(mockApi.updateRolePermissions).toHaveBeenCalledWith(7, [1, 2]);
  });

  it("a failed mutation does not invalidate the roles key", async () => {
    mockApi.createRole.mockRejectedValue(new Error("conflict"));
    const { result, invalidateSpy } = renderMutation(useCreateRole);

    await act(async () => {
      result.current.mutate({ role: "DUP", description: "" } as never);
    });
    await waitFor(() => expect(result.current.isError).toBe(true));

    expect(invalidateSpy).not.toHaveBeenCalled();
  });
});
