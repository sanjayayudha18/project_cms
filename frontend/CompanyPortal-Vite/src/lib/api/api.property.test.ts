import { useAuthStore } from "@/lib/auth/store";
import * as fc from "fast-check";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { api } from "./client";
import { computeDelay } from "./stubs/index";

/**
 * Property 17: Stub API Latency Range
 * Validates: Requirements 8.2
 *
 * For any request intercepted by the Stub API, the simulated delay SHALL be
 * a value in the inclusive range [200ms, 800ms].
 */
describe("Stub API — Property 17: Stub API Latency Range", () => {
  const randomValue = fc.double({
    min: 0,
    max: 1,
    noNaN: true,
    noDefaultInfinity: true,
    maxExcluded: true,
  });

  it("computeDelay always produces a value >= 200 for any random in [0, 1)", () => {
    fc.assert(
      fc.property(randomValue, (r) => {
        const delay = computeDelay(r, 200, 800);
        expect(delay).toBeGreaterThanOrEqual(200);
      }),
    );
  });

  it("computeDelay always produces a value <= 800 for any random in [0, 1)", () => {
    fc.assert(
      fc.property(randomValue, (r) => {
        const delay = computeDelay(r, 200, 800);
        expect(delay).toBeLessThanOrEqual(800);
      }),
    );
  });

  it("computeDelay always produces an integer", () => {
    fc.assert(
      fc.property(randomValue, (r) => {
        const delay = computeDelay(r, 200, 800);
        expect(Number.isInteger(delay)).toBe(true);
      }),
    );
  });

  it("computeDelay covers the full range boundaries", () => {
    // random = 0 should give min
    expect(computeDelay(0, 200, 800)).toBe(200);
    // random just below 1 should give max
    const almostOne = 1 - Number.EPSILON;
    expect(computeDelay(almostOne, 200, 800)).toBeLessThanOrEqual(800);
    expect(computeDelay(almostOne, 200, 800)).toBeGreaterThanOrEqual(200);
  });
});

/**
 * Property 18: Auth Header Injection
 * Validates: Requirements 8.4
 *
 * For any outgoing request to a protected API endpoint when a non-null access
 * token exists in the auth store, the request interceptor SHALL add an
 * `Authorization: Bearer {token}` header. The token value in the header SHALL
 * exactly match the stored access token.
 */
describe("API Client — Property 18: Auth Header Injection", () => {
  const mockFetch = vi.fn<typeof globalThis.fetch>();

  beforeEach(() => {
    vi.stubGlobal("fetch", mockFetch);
    useAuthStore.setState({
      user: null,
      accessToken: null,
      isAuthenticated: false,
      isAuthLoading: false,
      error: null,
      rateLimitRetryAfter: null,
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
    mockFetch.mockReset();
  });

  const arbitraryToken = fc.string({ minLength: 1 }).filter((s) => s.trim().length > 0);

  function successResponse(): Response {
    return new Response(JSON.stringify({ ok: true }), {
      status: 200,
      statusText: "OK",
      headers: { "Content-Type": "application/json" },
    });
  }

  it("Authorization header is always 'Bearer {token}' matching the stored token exactly", async () => {
    await fc.assert(
      fc.asyncProperty(arbitraryToken, async (token) => {
        mockFetch.mockReset();
        mockFetch.mockResolvedValue(successResponse());
        useAuthStore.setState({ accessToken: token });

        await api.get("/test");

        const [, options] = mockFetch.mock.calls[0];
        const headers = options?.headers as Record<string, string>;
        expect(headers.Authorization).toBe(`Bearer ${token}`);
      }),
    );
  });

  it("token value in header exactly matches the stored access token (no mutation)", async () => {
    await fc.assert(
      fc.asyncProperty(fc.uuid(), async (token) => {
        mockFetch.mockReset();
        mockFetch.mockResolvedValue(successResponse());
        useAuthStore.setState({ accessToken: token });

        await api.get("/protected-endpoint");

        const [, options] = mockFetch.mock.calls[0];
        const headers = options?.headers as Record<string, string>;
        const extractedToken = headers.Authorization?.replace("Bearer ", "");
        expect(extractedToken).toBe(token);
      }),
    );
  });

  it("no Authorization header when access token is null", async () => {
    mockFetch.mockResolvedValue(successResponse());
    useAuthStore.setState({ accessToken: null });

    await api.get("/test");

    const [, options] = mockFetch.mock.calls[0];
    const headers = options?.headers as Record<string, string>;
    expect(headers.Authorization).toBeUndefined();
  });
});
