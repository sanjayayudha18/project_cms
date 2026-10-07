/**
 * Task 8 test checklist: "filter change resets to page 1 + refetches with
 * correct params". Mirrors the mocking approach used by
 * vendor-request/__tests__/VendorRequestList.test.tsx for @tanstack/react-router.
 */

import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useAdminUsersUrlState } from "../useAdminUsersUrlState";

const navigateMock = vi.fn();
let searchState: Record<string, unknown> = {};

vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    useNavigate: () => navigateMock,
    useSearch: () => searchState,
  };
});

beforeEach(() => {
  navigateMock.mockReset();
  searchState = { page: 3, role: "ADMIN" };
});

describe("useAdminUsersUrlState", () => {
  it("resets page to 1 (omitted, since 1 is the default) when a filter changes", () => {
    const { result } = renderHook(() => useAdminUsersUrlState());

    result.current.setParams({ role: "APPACCESS", page: 1 });

    expect(navigateMock).toHaveBeenCalledWith({
      to: ".",
      search: { role: "APPACCESS" },
    });
  });

  it("carries the vendor_id filter through to the navigated search params", () => {
    const { result } = renderHook(() => useAdminUsersUrlState());

    result.current.setParams({ vendor_id: 5, page: 1 });

    expect(navigateMock).toHaveBeenCalledWith({
      to: ".",
      search: { role: "ADMIN", vendor_id: 5 },
    });
  });

  it("reflects the current page/role/vendor_id in params for the list query", () => {
    searchState = { page: 2, page_size: 25, status: "active", role: "APPACCESS", vendor_id: 5 };
    const { result } = renderHook(() => useAdminUsersUrlState());

    expect(result.current.params).toEqual({
      page: 2,
      page_size: 25,
      status: "active",
      vendor_id: 5,
      role: "APPACCESS",
      q: "",
    });
  });
});
