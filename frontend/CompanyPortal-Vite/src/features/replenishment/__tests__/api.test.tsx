/**
 * fe-coverage C4: realisasi replenish vs order — date-range rules (max 31 days) and the
 * classification/summary queries. downloadReport is not covered here: it calls `fetch`
 * directly instead of the API client (no 401 refresh), reported in fe-coverage/tests.md C4.
 */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const client = vi.hoisted(() => ({ get: vi.fn() }));
vi.mock("@/lib/api/client", () => ({ api: client }));

import {
  type ClassificationParams,
  rangeDays,
  rangeError,
  useClassifications,
  useReplenishmentSummary,
} from "../api";

function setup<T>(hook: () => T) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  return renderHook(hook, { wrapper });
}

const base: ClassificationParams = {
  dateFrom: "2026-10-01",
  dateTo: "2026-10-15",
  vendorId: null,
  regionCode: null,
  classification: null,
  onlyDiff: false,
  page: 1,
};

beforeEach(() => {
  client.get.mockReset();
});

describe("date range rules", () => {
  it("counts days inclusively", () => {
    expect(rangeDays("2026-10-01", "2026-10-01")).toBe(1);
    expect(rangeDays("2026-10-01", "2026-10-31")).toBe(31);
  });

  it.each([
    ["", "2026-10-15", "Isi tanggal awal dan akhir"],
    ["2026-10-01", "", "Isi tanggal awal dan akhir"],
    ["2026-10-15", "2026-10-01", "Tanggal akhir sebelum tanggal awal"],
    ["2026-10-01", "2026-11-01", "Rentang maksimal 31 hari"],
    ["2026-10-01", "2026-10-31", null],
  ])("rangeError(%s, %s) = %s", (from, to, expected) => {
    expect(rangeError(from, to)).toBe(expected);
  });
});

describe("classification + summary queries", () => {
  it("sends only the dates and page when no filter is set", async () => {
    client.get.mockResolvedValue({ data: { items: [], total: 0 } });
    const { result } = setup(() => useClassifications(base));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(client.get).toHaveBeenCalledWith(
      "/replenishment/classifications?date_from=2026-10-01&date_to=2026-10-15&page=1&page_size=20",
    );
  });

  it("sends every filter that is set", async () => {
    client.get.mockResolvedValue({ data: { items: [], total: 0 } });
    const { result } = setup(() =>
      useClassifications({
        ...base,
        vendorId: 0,
        regionCode: "JKT",
        classification: "late",
        onlyDiff: true,
        page: 3,
      }),
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(client.get).toHaveBeenCalledWith(
      "/replenishment/classifications?date_from=2026-10-01&date_to=2026-10-15&vendor_id=0&region_code=JKT&classification=late&only_diff=true&page=3&page_size=20",
    );
  });

  it("does not query an invalid range (over 31 days)", () => {
    const { result } = setup(() =>
      useClassifications({ ...base, dateFrom: "2026-09-01", dateTo: "2026-10-15" }),
    );

    expect(result.current.fetchStatus).toBe("idle");
    expect(client.get).not.toHaveBeenCalled();
  });

  it("loads the summary for the range", async () => {
    client.get.mockResolvedValue({ data: { by_class: [], instructions: [] } });
    const { result } = setup(() => useReplenishmentSummary("2026-10-01", "2026-10-15"));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(client.get).toHaveBeenCalledWith(
      "/replenishment/summary?date_from=2026-10-01&date_to=2026-10-15",
    );
    expect(result.current.data).toEqual({ by_class: [], instructions: [] });
  });
});
