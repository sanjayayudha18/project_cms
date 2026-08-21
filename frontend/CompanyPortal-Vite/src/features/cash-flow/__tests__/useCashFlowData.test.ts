import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { createElement } from "react";
import { describe, expect, it } from "vitest";

import { CASH_FLOW_QUERY_KEY } from "../constants";
import { useCashFlowData } from "../useCashFlowData";

function createWrapper(queryClient?: QueryClient) {
  const client =
    queryClient ??
    new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return createElement(QueryClientProvider, { client }, children);
  };
}

describe("useCashFlowData", () => {
  it('uses the correct query key ["cash-flow", "summary"]', () => {
    expect(CASH_FLOW_QUERY_KEY).toEqual(["cash-flow", "summary"]);
  });

  it("returns isLoading: true initially", () => {
    const { result } = renderHook(() => useCashFlowData(), {
      wrapper: createWrapper(),
    });

    expect(result.current.isLoading).toBe(true);
    expect(result.current.data).toBeUndefined();
  });

  it("returns data conforming to CashFlowSummary shape after loading", async () => {
    const { result } = renderHook(() => useCashFlowData(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    const data = result.current.data;
    expect(data).toBeDefined();

    // stats has length 4
    expect(data!.stats).toHaveLength(4);

    // vendorChart.data has length 7 (7 days)
    expect(data!.vendorChart.data).toHaveLength(7);

    // vendorChart.vendors has length 4
    expect(data!.vendorChart.vendors).toHaveLength(4);

    // atmLevels has length 6
    expect(data!.atmLevels).toHaveLength(6);
  });

  it('data values match prototype (first stat: "Rp 48,2 M")', async () => {
    const { result } = renderHook(() => useCashFlowData(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    const data = result.current.data!;
    expect(data.stats[0]!.value).toBe("Rp 48,2 M");
    expect(data.stats[0]!.label).toBe("Total Kas Beredar");
  });

  it("exposes error state fields for error handling", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });

    const { result } = renderHook(() => useCashFlowData(), {
      wrapper: createWrapper(queryClient),
    });

    // Hook returns the error-related interface fields
    expect(result.current).toHaveProperty("isError");
    expect(result.current).toHaveProperty("error");
    expect(result.current).toHaveProperty("refetch");
    expect(typeof result.current.refetch).toBe("function");

    // Initially no error while loading/fetching
    expect(result.current.isError).toBe(false);
    expect(result.current.error).toBeNull();

    // Wait for data to load successfully, confirming no error occurred
    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.isError).toBe(false);
    expect(result.current.data).toBeDefined();
  });
});
