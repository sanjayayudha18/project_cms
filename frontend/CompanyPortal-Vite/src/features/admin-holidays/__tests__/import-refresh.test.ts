/**
 * Bugfix 2026-10-15: the CSV import goes through the real API client, so an expired access
 * token is refreshed and the upload retried once (single-flight 401 refresh, FE-08).
 * Only `fetch` and the store's refreshToken are mocked.
 */

import { useAuthStore } from "@/lib/auth/store";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { importHolidays } from "../api";

const mockFetch = vi.fn();
const initialState = useAuthStore.getState();

beforeEach(() => {
  mockFetch.mockReset();
  vi.stubGlobal("fetch", mockFetch);
  useAuthStore.setState({
    accessToken: "expired-token",
    refreshToken: vi.fn(async () => {
      useAuthStore.setState({ accessToken: "fresh-token" });
      return true;
    }),
  });
});

afterEach(() => {
  vi.unstubAllGlobals();
  useAuthStore.setState(initialState, true);
});

it("a 401 on import refreshes the token and retries the upload once", async () => {
  mockFetch
    .mockResolvedValueOnce(new Response(null, { status: 401 }))
    .mockResolvedValueOnce(new Response(JSON.stringify({ created: 1, skipped: [] })));
  const file = new File(["holiday_date,name\n2026-12-25,Natal\n"], "libur.csv");

  await expect(importHolidays(file)).resolves.toEqual({ created: 1, skipped: [] });

  expect(useAuthStore.getState().refreshToken).toHaveBeenCalledTimes(1);
  expect(mockFetch).toHaveBeenCalledTimes(2);
  const [url, init] = mockFetch.mock.calls[1] as [string, RequestInit];
  expect(url).toMatch(/\/admin\/holidays\/import$/);
  expect(init.method).toBe("POST");
  expect((init.headers as Record<string, string>).Authorization).toBe("Bearer fresh-token");
  expect((init.body as FormData).get("file")).toBeInstanceOf(File);
});
