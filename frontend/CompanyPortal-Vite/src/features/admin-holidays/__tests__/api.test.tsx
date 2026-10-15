/**
 * fe-coverage C2: admin-holidays api (queries, mutations, CSV import). The HTTP client
 * is mocked; the real TanStack hooks run.
 */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const client = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn() }));
vi.mock("@/lib/api/client", () => ({ api: client }));

import {
  importHolidays,
  useCreateHoliday,
  useHolidays,
  useImportHolidays,
  useRenameHoliday,
  useSetHolidayActive,
} from "../api";

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
const KEY = { queryKey: ["admin-holidays"] };

beforeEach(() => {
  client.get.mockReset();
  client.post.mockReset();
  client.put.mockReset();
});

describe("admin-holidays queries + mutations", () => {
  it("useHolidays asks for one year + status, up to 100 rows", async () => {
    client.get.mockResolvedValue({ data: { items: [{ id: 1 }], total: 1 } });
    const { result } = setup(() => useHolidays(2026, "active"));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(client.get).toHaveBeenCalledWith(
      "/admin/holidays?year=2026&status=active&page_size=100",
    );
    expect(result.current.data).toEqual({ items: [{ id: 1 }], total: 1 });
  });

  it.each([
    [
      "create",
      () => useCreateHoliday(),
      { holiday_date: "2026-12-25", name: "Natal" },
      ["post", "/admin/holidays", { holiday_date: "2026-12-25", name: "Natal" }],
    ],
    [
      "rename",
      () => useRenameHoliday(),
      { id: 3, name: "Hari Natal" },
      ["put", "/admin/holidays/3", { name: "Hari Natal" }],
    ],
    [
      "enable",
      () => useSetHolidayActive(),
      { id: 3, active: true },
      ["post", "/admin/holidays/3/enable"],
    ],
    [
      "disable",
      () => useSetHolidayActive(),
      { id: 3, active: false },
      ["post", "/admin/holidays/3/disable"],
    ],
  ])("%s calls the endpoint and refreshes the holiday list", async (_n, hook, vars, expected) => {
    client.post.mockResolvedValue({ data: {} });
    client.put.mockResolvedValue({ data: {} });
    const { result, invalidate } = setup(hook as () => Mutation);

    await act(async () => {
      await result.current.mutateAsync(vars);
    });

    const [method, ...args] = expected as [keyof typeof client, ...unknown[]];
    expect(client[method]).toHaveBeenCalledWith(...args);
    expect(invalidate).toHaveBeenCalledWith(KEY);
  });

  it("a duplicate date (409) is surfaced and the list is not refreshed", async () => {
    client.post.mockRejectedValue({ status: 409, message: "Tanggal sudah ada" });
    const { result, invalidate } = setup(() => useCreateHoliday());

    await act(async () => {
      await result.current
        .mutateAsync({ holiday_date: "2026-12-25", name: "Natal" })
        .catch(() => {});
    });

    await waitFor(() => expect(result.current.error?.message).toBe("Tanggal sudah ada"));
    expect(invalidate).not.toHaveBeenCalled();
  });
});

describe("importHolidays", () => {
  // Bugfix 2026-10-15: the import goes through api.post (401 refresh + retry, see
  // import-refresh.test.ts), so these tests pin the client contract, not raw fetch.
  const file = new File(["holiday_date,name\n2026-12-25,Natal\n"], "libur.csv", {
    type: "text/csv",
  });

  it("posts the file as multipart form data and returns the result", async () => {
    client.post.mockResolvedValue({ data: { created: 1, skipped: [] } });

    await expect(importHolidays(file)).resolves.toEqual({ created: 1, skipped: [] });

    const [path, body] = client.post.mock.calls[0] as [string, FormData];
    expect(path).toBe("/admin/holidays/import");
    expect(body.get("file")).toBeInstanceOf(File);
  });

  it("throws the server message on failure, or a default when there is none", async () => {
    client.post.mockRejectedValueOnce({ status: 422, message: "Format CSV salah" });
    await expect(importHolidays(file)).rejects.toThrow("Format CSV salah");

    client.post.mockRejectedValueOnce({ status: 500, message: "" });
    await expect(importHolidays(file)).rejects.toThrow("Impor gagal");
  });

  it("useImportHolidays refreshes the holiday list after a successful import", async () => {
    client.post.mockResolvedValue({ data: { created: 2, skipped: [] } });
    const { result, invalidate } = setup(() => useImportHolidays());

    await act(async () => {
      await result.current.mutateAsync(file);
    });

    expect(invalidate).toHaveBeenCalledWith(KEY);
  });
});
