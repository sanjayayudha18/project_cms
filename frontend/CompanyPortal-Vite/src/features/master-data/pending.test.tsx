import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { usePendingCreates, usePendingEntityIds } from "./pending";

const getMock = vi.fn();
vi.mock("@/lib/api/client", () => ({ api: { get: (...args: unknown[]) => getMock(...args) } }));

function wrapper({ children }: { children: React.ReactNode }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

describe("pending changes", () => {
  it("splits one response into row ids (update/disable) and staged creates", async () => {
    getMock.mockResolvedValue({
      data: {
        changes: [
          { id: 1, entity_id: null, op: "create", payload: { vault_code: "NEW" } },
          { id: 2, entity_id: 40, op: "update", payload: { category: "ATM" } },
          { id: 3, entity_id: 41, op: "disable", payload: null },
        ],
      },
    });

    const { result } = renderHook(
      () => ({
        ids: usePendingEntityIds("vendor_vault"),
        creates: usePendingCreates("vendor_vault"),
      }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.creates).toHaveLength(1));
    expect(result.current.creates[0]).toEqual({ id: 1, payload: { vault_code: "NEW" } });
    expect([...result.current.ids].sort()).toEqual([40, 41]);
    expect(getMock).toHaveBeenCalledTimes(1);
    expect(getMock.mock.calls[0][0]).toContain("entity_type=vendor_vault&status=pending");
  });
});
