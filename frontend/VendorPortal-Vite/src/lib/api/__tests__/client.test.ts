import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// fe-coverage C1: VendorPortal api client — Bearer injection, 401 single-flight refresh,
// FormData, error parsing. The module keeps its auth hooks in module state, so each test
// imports a fresh copy.

type ClientModule = typeof import("../client");

const mockFetch = vi.fn();

async function loadClient(): Promise<ClientModule> {
  vi.resetModules();
  return import("../client");
}

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status });
}

function headersOf(call: number): Record<string, string> {
  return (mockFetch.mock.calls[call]?.[1] as RequestInit).headers as Record<string, string>;
}

beforeEach(() => {
  mockFetch.mockReset();
  vi.stubGlobal("fetch", mockFetch);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("VendorPortal api client", () => {
  it("sends JSON with Bearer token and credentials", async () => {
    const { api, configureApiAuth } = await loadClient();
    configureApiAuth({ getToken: () => "tok", refresh: vi.fn(), onFailure: vi.fn() });
    mockFetch.mockResolvedValueOnce(json({ ok: 1 }));

    const res = await api.post<{ ok: number }>("/api/v1/x", { a: 1 });

    expect(res).toEqual({ data: { ok: 1 }, status: 200 });
    const [url, init] = mockFetch.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/v1/x");
    expect(init.method).toBe("POST");
    expect(init.body).toBe('{"a":1}');
    expect(init.credentials).toBe("include");
    expect(headersOf(0)).toEqual({
      "Content-Type": "application/json",
      Authorization: "Bearer tok",
    });
  });

  it("omits Authorization when skipAuth is set or there is no token", async () => {
    const { api, configureApiAuth } = await loadClient();
    configureApiAuth({ getToken: () => "tok", refresh: vi.fn(), onFailure: vi.fn() });
    mockFetch.mockImplementation(async () => json({}));

    await api.get("/api/v1/public", { skipAuth: true });
    configureApiAuth({ getToken: () => null, refresh: vi.fn(), onFailure: vi.fn() });
    await api.get("/api/v1/anon");

    expect(headersOf(0).Authorization).toBeUndefined();
    expect(headersOf(1).Authorization).toBeUndefined();
  });

  it("sends FormData as-is without a Content-Type header", async () => {
    const { api, configureApiAuth } = await loadClient();
    configureApiAuth({ getToken: () => "tok", refresh: vi.fn(), onFailure: vi.fn() });
    mockFetch.mockResolvedValueOnce(json({ id: 1 }, 201));
    const form = new FormData();
    form.append("file", new Blob(["x"]), "a.xlsx");

    await api.post("/api/v1/dsr", form);

    const init = mockFetch.mock.calls[0]?.[1] as RequestInit;
    expect(init.body).toBe(form);
    expect(headersOf(0)).toEqual({ Authorization: "Bearer tok" });
  });

  it("put/patch/delete use the right method; GET has no body", async () => {
    const { api } = await loadClient();
    mockFetch.mockImplementation(async () => json({}));

    await api.put("/p", { v: 1 });
    await api.patch("/q", { v: 2 });
    await api.delete("/r");
    await api.get("/s");

    expect(mockFetch.mock.calls.map((c) => (c[1] as RequestInit).method)).toEqual([
      "PUT",
      "PATCH",
      "DELETE",
      "GET",
    ]);
    expect((mockFetch.mock.calls[3]?.[1] as RequestInit).body).toBeUndefined();
  });

  it("returns undefined data on 204", async () => {
    const { api } = await loadClient();
    mockFetch.mockResolvedValueOnce(new Response(null, { status: 204 }));

    await expect(api.delete("/r")).resolves.toEqual({ data: undefined, status: 204 });
  });

  it("on 401 refreshes once, then retries with the new token", async () => {
    const { api, configureApiAuth } = await loadClient();
    let token = "old";
    const refresh = vi.fn(async () => {
      token = "new";
      return true;
    });
    configureApiAuth({ getToken: () => token, refresh, onFailure: vi.fn() });
    mockFetch
      .mockResolvedValueOnce(json({ error: "unauthorized" }, 401))
      .mockResolvedValueOnce(json({ ok: true }));

    const res = await api.get("/api/v1/x");

    expect(res.data).toEqual({ ok: true });
    expect(refresh).toHaveBeenCalledTimes(1);
    expect(headersOf(1).Authorization).toBe("Bearer new");
  });

  it("concurrent 401s share one refresh", async () => {
    const { api, configureApiAuth } = await loadClient();
    let release: (v: boolean) => void = () => {};
    const refresh = vi.fn(
      () =>
        new Promise<boolean>((resolve) => {
          release = resolve;
        }),
    );
    configureApiAuth({ getToken: () => "t", refresh, onFailure: vi.fn() });
    mockFetch.mockImplementation(async () => json({ error: "unauthorized" }, 401));

    const p1 = api.get("/a");
    const p2 = api.get("/b");
    await vi.waitFor(() => expect(mockFetch).toHaveBeenCalledTimes(2));
    mockFetch.mockImplementation(async () => json({ ok: true }));
    release(true);

    await expect(p1).resolves.toMatchObject({ status: 200 });
    await expect(p2).resolves.toMatchObject({ status: 200 });
    expect(refresh).toHaveBeenCalledTimes(1);
  });

  it("refresh failure calls onFailure and throws the original 401", async () => {
    const { api, configureApiAuth } = await loadClient();
    const onFailure = vi.fn();
    configureApiAuth({ getToken: () => "t", refresh: async () => false, onFailure });
    mockFetch.mockResolvedValueOnce(new Response("", { status: 401, statusText: "Unauthorized" }));

    await expect(api.get("/x")).rejects.toEqual({ status: 401, message: "Unauthorized" });
    expect(onFailure).toHaveBeenCalledTimes(1);
    expect(mockFetch).toHaveBeenCalledTimes(1);
  });

  it("401 before auth is wired: no refresh, throws", async () => {
    const { api } = await loadClient();
    mockFetch.mockResolvedValueOnce(new Response("", { status: 401 }));

    await expect(api.get("/x")).rejects.toMatchObject({ status: 401 });
    expect(mockFetch).toHaveBeenCalledTimes(1);
  });

  it("does not refresh on 401 for skipAuth requests", async () => {
    const { api, configureApiAuth } = await loadClient();
    const refresh = vi.fn();
    configureApiAuth({ getToken: () => "t", refresh, onFailure: vi.fn() });
    mockFetch.mockResolvedValueOnce(json({ message: "Sesi berakhir" }, 401));

    await expect(api.post("/login", {}, { skipAuth: true })).rejects.toEqual({
      status: 401,
      message: "Sesi berakhir",
      details: undefined,
    });
    expect(refresh).not.toHaveBeenCalled();
  });

  it("parses message + details from an error body", async () => {
    const { api } = await loadClient();
    mockFetch.mockResolvedValueOnce(
      json({ message: "Validasi gagal", details: [{ field: "amount" }] }, 422),
    );

    await expect(api.post("/x", {})).rejects.toEqual({
      status: 422,
      message: "Validasi gagal",
      details: [{ field: "amount" }],
    });
  });

  it("falls back to statusText when the error body is not JSON", async () => {
    const { api } = await loadClient();
    mockFetch.mockResolvedValueOnce(
      new Response("<html>bad gateway</html>", { status: 502, statusText: "Bad Gateway" }),
    );

    await expect(api.get("/x")).rejects.toEqual({ status: 502, message: "Bad Gateway" });
  });
});
