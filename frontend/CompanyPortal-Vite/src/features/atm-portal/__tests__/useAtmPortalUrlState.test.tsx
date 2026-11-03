/**
 * fe-coverage C5: ATM Portal URL state hook (mode switch, setParams, debounced search).
 * parse/omit round-trips are covered by useAtmPortalUrlState.property.test.ts.
 */

import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const nav = vi.hoisted(() => ({ navigate: vi.fn(), search: {} as Record<string, unknown> }));
vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return { ...actual, useNavigate: () => nav.navigate, useSearch: () => nav.search };
});

import { ATM_PORTAL_MODE_CASHPOS } from "../constants";
import { useAtmPortalUrlState } from "../useAtmPortalUrlState";

beforeEach(() => {
  nav.navigate.mockReset();
  nav.search = {};
});

afterEach(() => {
  vi.useRealTimers();
});

const lastSearch = () =>
  (nav.navigate.mock.calls.at(-1)?.[0] as { search: Record<string, unknown> }).search;

describe("useAtmPortalUrlState", () => {
  it("switching to the same mode does nothing", () => {
    const { result } = renderHook(() => useAtmPortalUrlState());

    result.current.setMode(result.current.params.mode);

    expect(nav.navigate).not.toHaveBeenCalled();
  });

  it("switching to cashpos resets to page 1 with that mode's default sort (kept out of the URL)", () => {
    nav.search = { page: 4, sort_by: "custom", sort_order: "desc" };
    const { result } = renderHook(() => useAtmPortalUrlState());

    result.current.setMode(ATM_PORTAL_MODE_CASHPOS);

    const search = lastSearch();
    expect(search.mode).toBe(ATM_PORTAL_MODE_CASHPOS);
    expect(search).not.toHaveProperty("page");
    expect(search).not.toHaveProperty("sort_by");
    expect(search).not.toHaveProperty("sort_order");
  });

  it("switching back from cashpos uses the replenish defaults", () => {
    nav.search = { mode: ATM_PORTAL_MODE_CASHPOS, page: 2 };
    const { result } = renderHook(() => useAtmPortalUrlState());

    result.current.setMode("replenish" as never);

    expect(lastSearch()).not.toHaveProperty("mode");
    expect(lastSearch()).not.toHaveProperty("sort_by");
  });

  it("setParams merges into the URL", () => {
    const { result } = renderHook(() => useAtmPortalUrlState());

    result.current.setParams({ page: 3, region: "JKT" } as never);

    expect(lastSearch()).toMatchObject({ page: 3, region: "JKT" });
  });

  it("typing a search is pushed to the URL only after the debounce", () => {
    vi.useFakeTimers();
    const { result } = renderHook(() => useAtmPortalUrlState());

    act(() => result.current.setSearchInput("T00"));
    expect(nav.navigate).not.toHaveBeenCalled();
    act(() => {
      vi.advanceTimersByTime(1000);
    });

    expect(lastSearch()).toMatchObject({ search: "T00" });
  });
});
