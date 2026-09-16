/**
 * Mutation-hook invalidation tests (Task 16.2: "successful submit
 * invalidates the query key", Requirements 4.4, 5.5, 6.3). The hooks are
 * real (real TanStack Query, real QueryClient); only `../api` is mocked
 * so no network runs. Mirrors the renderHook pattern in
 * useCashFlowData.test.ts.
 */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mockApi = vi.hoisted(() => ({
  getUserHierarchy: vi.fn(),
  getDelegations: vi.fn(),
  getLeaves: vi.fn(),
  getPolicies: vi.fn(),
  setUserHierarchy: vi.fn(),
  createDelegation: vi.fn(),
  revokeDelegation: vi.fn(),
  createLeave: vi.fn(),
  createPolicy: vi.fn(),
  updatePolicy: vi.fn(),
}));

vi.mock("../api", () => mockApi);

import {
  rbacKeys,
  useCreateDelegation,
  useCreateLeave,
  useCreatePolicy,
  useRevokeDelegation,
  useSetHierarchy,
  useUpdatePolicy,
} from "../hooks/useRbacQueries";

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

async function expectInvalidatesKey<THookResult extends { mutate: (v: never) => unknown }>(
  hook: () => THookResult,
  apiFn: (typeof mockApi)[keyof typeof mockApi],
  resolvedValue: unknown,
  variables: never,
  expectedKey: unknown,
) {
  apiFn.mockResolvedValue(resolvedValue);
  const { result, invalidateSpy } = renderMutation(hook);

  await act(async () => {
    result.current.mutate(variables);
  });
  await waitFor(() => expect(result.current.isSuccess).toBe(true));

  expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: expectedKey });
}

beforeEach(() => {
  for (const fn of Object.values(mockApi)) {
    fn.mockReset();
  }
});

describe("rbac mutation hooks invalidate their own key on success", () => {
  it("useSetHierarchy invalidates the users key", async () => {
    await expectInvalidatesKey(
      useSetHierarchy,
      mockApi.setUserHierarchy,
      { id: 5, supervisor_id: 2, approval_level: 3 },
      { userId: 5, values: { supervisor_id: 2, approval_level: 3 } } as never,
      rbacKeys.users(),
    );
  });

  it("useCreateDelegation invalidates the delegations key", async () => {
    await expectInvalidatesKey(
      useCreateDelegation,
      mockApi.createDelegation,
      {
        id: 1,
        from_user_id: 1,
        to_user_id: 2,
        start_at: "2026-09-20T02:00:00Z",
        end_at: "2026-09-22T10:00:00Z",
        reason: null,
      },
      {
        from_user_id: 1,
        to_user_id: 2,
        start_at: "2026-09-20T02:00:00Z",
        end_at: "2026-09-22T10:00:00Z",
      } as never,
      rbacKeys.delegations(),
    );
  });

  it("useRevokeDelegation invalidates the delegations key", async () => {
    await expectInvalidatesKey(
      useRevokeDelegation,
      mockApi.revokeDelegation,
      {
        id: 7,
        from_user_id: 1,
        to_user_id: 2,
        start_at: "2026-09-20T02:00:00Z",
        end_at: "2026-09-22T10:00:00Z",
        reason: null,
      },
      7 as never,
      rbacKeys.delegations(),
    );
  });

  it("useCreateLeave invalidates the leaves key", async () => {
    await expectInvalidatesKey(
      useCreateLeave,
      mockApi.createLeave,
      { id: 1, user_id: 3, start_at: "2026-09-20T02:00:00Z", end_at: "2026-09-22T10:00:00Z" },
      {
        user_id: 3,
        start_at: "2026-09-20T02:00:00Z",
        end_at: "2026-09-22T10:00:00Z",
      } as never,
      rbacKeys.leaves(),
    );
  });

  it("useCreatePolicy invalidates the policies key", async () => {
    await expectInvalidatesKey(
      useCreatePolicy,
      mockApi.createPolicy,
      {
        id: 1,
        document_type: "invoice",
        min_amount: "1000000.00",
        max_amount: "5000000.00",
        required_level: 2,
      },
      {
        document_type: "invoice",
        min_amount: "1000000.00",
        max_amount: "5000000.00",
        required_level: 2,
      } as never,
      rbacKeys.policies(),
    );
  });

  it("useUpdatePolicy invalidates the policies key", async () => {
    await expectInvalidatesKey(
      useUpdatePolicy,
      mockApi.updatePolicy,
      {
        id: 4,
        document_type: "invoice",
        min_amount: "1000000.00",
        max_amount: "6000000.00",
        required_level: 2,
      },
      {
        id: 4,
        values: {
          document_type: "invoice",
          min_amount: "1000000.00",
          max_amount: "6000000.00",
          required_level: 2,
        },
      } as never,
      rbacKeys.policies(),
    );
  });
});
