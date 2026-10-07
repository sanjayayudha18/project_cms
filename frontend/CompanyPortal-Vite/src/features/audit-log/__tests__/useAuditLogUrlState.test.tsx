import { renderHook } from "@testing-library/react";
import * as fc from "fast-check";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { EMPTY_AUDIT_LOG_FILTERS } from "../types";
import { omitDefaults, parseParams, useAuditLogUrlState } from "../useAuditLogUrlState";

const navigateMock = vi.fn();
let searchState: Record<string, unknown> = {};

vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return { ...actual, useNavigate: () => navigateMock, useSearch: () => searchState };
});

beforeEach(() => {
  navigateMock.mockReset();
  searchState = { page: 4, action: "approve" };
});

describe("useAuditLogUrlState", () => {
  it("a filter change resets the page to 1 (omitted from the URL) and keeps other filters", () => {
    const { result } = renderHook(() => useAuditLogUrlState());

    result.current.setFilters({ entity_type: "vendor_request" });

    expect(navigateMock).toHaveBeenCalledWith({
      to: ".",
      search: { action: "approve", entity_type: "vendor_request" },
    });
  });

  it("setPage keeps the filters and only changes the page", () => {
    const { result } = renderHook(() => useAuditLogUrlState());

    result.current.setPage(5);

    expect(navigateMock).toHaveBeenCalledWith({ to: ".", search: { page: 5, action: "approve" } });
  });

  it("resetFilters clears every filter and the page", () => {
    const { result } = renderHook(() => useAuditLogUrlState());

    result.current.resetFilters();

    expect(navigateMock).toHaveBeenCalledWith({ to: ".", search: {} });
  });

  it("stringifies numeric ids the router JSON-parsed", () => {
    searchState = { actor_id: 3, entity_id: 12 };
    const { result } = renderHook(() => useAuditLogUrlState());

    expect(result.current.params.filters.actor_id).toBe("3");
    expect(result.current.params.filters.entity_id).toBe("12");
  });
});

/** Property 12: URL round-trip reconstructs identical filters. */
describe("URL round-trip", () => {
  it("parseParams(omitDefaults(x)) === x", () => {
    const digits = fc.integer({ min: 0, max: 1_000_000 }).map(String);
    const date = fc
      .date({ min: new Date("2020-01-01"), max: new Date("2030-01-01"), noInvalidDate: true })
      .map((d) => d.toISOString().slice(0, 10));
    const word = fc.stringMatching(/^[a-z_]{1,20}$/);

    fc.assert(
      fc.property(
        fc.integer({ min: 1, max: 500 }),
        fc.integer({ min: 1, max: 100 }),
        fc.option(digits, { nil: null }),
        fc.option(word, { nil: null }),
        fc.option(word, { nil: null }),
        fc.option(digits, { nil: null }),
        fc.option(date, { nil: null }),
        fc.option(date, { nil: null }),
        (page, page_size, actor_id, action, entity_type, entity_id, date_from, date_to) => {
          const params = {
            page,
            page_size,
            filters: { actor_id, action, entity_type, entity_id, date_from, date_to },
          };
          expect(parseParams(omitDefaults(params))).toEqual(params);
        },
      ),
    );
  });

  it("empty filters + defaults produce an empty URL", () => {
    expect(omitDefaults({ page: 1, page_size: 25, filters: EMPTY_AUDIT_LOG_FILTERS })).toEqual({});
  });
});
