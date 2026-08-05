import { useAuthStore } from "@/lib/auth/store";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { api, apiClient } from "./client";

// Force the real-backend code path so the fetch mocks below are exercised
// instead of the stub interceptor (test env resolves mode to "stub" by default).
vi.mock("./config", () => ({
  apiConfig: { mode: "real", baseURL: "/api/v1", stubLatency: { min: 200, max: 800 } },
}));

// ─── Mocks ────────────────────────────────────────────────────────────────────

const mockFetch = vi.fn<typeof globalThis.fetch>();

beforeEach(() => {
  vi.stubGlobal("fetch", mockFetch);
  useAuthStore.setState({
    user: null,
    accessToken: null,
    isAuthenticated: false,
    isLoading: false,
  });
});

afterEach(() => {
  vi.restoreAllMocks();
  mockFetch.mockReset();
});

// ─── Helpers ──────────────────────────────────────────────────────────────────

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    statusText: status === 200 ? "OK" : "Error",
    headers: { "Content-Type": "application/json" },
  });
}

function unauthorizedResponse(): Response {
  return new Response(JSON.stringify({ message: "Unauthorized" }), {
    status: 401,
    statusText: "Unauthorized",
    headers: { "Content-Type": "application/json" },
  });
}

// ─── Tests ────────────────────────────────────────────────────────────────────

describe("API Client - Auth Header Injection", () => {
  it("injects Authorization header when access token is present", async () => {
    useAuthStore.setState({ accessToken: "test-jwt-token" });
    mockFetch.mockResolvedValueOnce(jsonResponse({ ok: true }));

    await api.get("/test");

    const [, options] = mockFetch.mock.calls[0];
    const headers = options?.headers as Record<string, string>;
    expect(headers.Authorization).toBe("Bearer test-jwt-token");
  });

  it("does not inject Authorization header when no token", async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ ok: true }));

    await api.get("/test");

    const [, options] = mockFetch.mock.calls[0];
    const headers = options?.headers as Record<string, string>;
    expect(headers.Authorization).toBeUndefined();
  });

  it("skips auth header when skipAuth is true", async () => {
    useAuthStore.setState({ accessToken: "test-jwt-token" });
    mockFetch.mockResolvedValueOnce(jsonResponse({ ok: true }));

    await apiClient({ method: "GET", path: "/auth/login", skipAuth: true });

    const [, options] = mockFetch.mock.calls[0];
    const headers = options?.headers as Record<string, string>;
    expect(headers.Authorization).toBeUndefined();
  });

  it("includes credentials: include for cookie-based refresh", async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ ok: true }));

    await api.get("/test");

    const [, options] = mockFetch.mock.calls[0];
    expect(options?.credentials).toBe("include");
  });
});

describe("API Client - Request Methods", () => {
  it("sends GET request to correct URL", async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ items: [] }));

    const result = await api.get<{ items: string[] }>("/users");

    const [url] = mockFetch.mock.calls[0];
    expect(url).toBe("/api/v1/users");
    expect(result.data).toEqual({ items: [] });
    expect(result.status).toBe(200);
  });

  it("sends POST request with JSON body", async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ id: "1" }, 201));

    await api.post("/users", { name: "Test" });

    const [, options] = mockFetch.mock.calls[0];
    expect(options?.method).toBe("POST");
    expect(options?.body).toBe(JSON.stringify({ name: "Test" }));
  });

  it("handles 204 No Content", async () => {
    mockFetch.mockResolvedValueOnce(new Response(null, { status: 204, statusText: "No Content" }));

    const result = await api.delete("/users/1");

    expect(result.status).toBe(204);
    expect(result.data).toBeUndefined();
  });

  it("throws ApiError on non-2xx response", async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ message: "Not Found" }, 404));

    await expect(api.get("/missing")).rejects.toMatchObject({
      status: 404,
      message: "Not Found",
    });
  });
});

describe("API Client - 401 Retry Logic", () => {
  it("retries request after successful token refresh", async () => {
    useAuthStore.setState({ accessToken: "expired-token" });

    // First call returns 401
    mockFetch.mockResolvedValueOnce(unauthorizedResponse());
    // Refresh token call succeeds
    mockFetch.mockResolvedValueOnce(
      jsonResponse({
        accessToken: "new-token",
        user: {
          id: "1",
          fullName: "User",
          email: "u@test.com",
          roles: ["Admin"],
          primaryRole: "Admin",
        },
      }),
    );
    // Retry call succeeds
    mockFetch.mockResolvedValueOnce(jsonResponse({ data: "success" }));

    const result = await api.get<{ data: string }>("/protected");

    // Should have been called 3 times: original + refresh + retry
    expect(mockFetch).toHaveBeenCalledTimes(3);
    expect(result.data).toEqual({ data: "success" });
  });

  it("clears session and throws when refresh also fails", async () => {
    useAuthStore.setState({ accessToken: "expired-token" });

    // Mock window.location for logout redirect
    const originalLocation = window.location;
    Object.defineProperty(window, "location", {
      value: { ...originalLocation, href: "/" },
      writable: true,
      configurable: true,
    });

    // First call returns 401
    mockFetch.mockResolvedValueOnce(unauthorizedResponse());
    // Refresh token call also returns 401 (failure)
    mockFetch.mockResolvedValueOnce(unauthorizedResponse());
    // logout() fires a background POST to /auth/logout
    mockFetch.mockResolvedValueOnce(jsonResponse({}));

    await expect(api.get("/protected")).rejects.toMatchObject({
      status: 401,
    });

    // Auth store should be cleared after failed refresh
    const state = useAuthStore.getState();
    expect(state.accessToken).toBeNull();
    expect(state.isAuthenticated).toBe(false);

    // Restore
    Object.defineProperty(window, "location", {
      value: originalLocation,
      writable: true,
      configurable: true,
    });
  });

  it("does not retry when skipAuth is true", async () => {
    useAuthStore.setState({ accessToken: "some-token" });
    mockFetch.mockResolvedValueOnce(unauthorizedResponse());

    await expect(
      apiClient({ method: "GET", path: "/auth/refresh", skipAuth: true }),
    ).rejects.toMatchObject({ status: 401 });

    // Only 1 call, no retry
    expect(mockFetch).toHaveBeenCalledTimes(1);
  });
});
