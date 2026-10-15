import { api } from "@/lib/api/client";
import { act, render, screen, waitFor } from "@testing-library/react";
import { StrictMode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AuthProvider } from "../AuthContext";
import { useAuth, useAuthRefresh } from "../useAuth";

// ─── Mock fetch ───────────────────────────────────────────────────────────────

const mockFetch = vi.fn();

beforeEach(() => {
  vi.stubGlobal("fetch", mockFetch);
  // Default: refresh on mount returns 401 (not authenticated)
  mockFetch.mockResolvedValue(
    new Response(JSON.stringify({ error: "unauthorized" }), { status: 401 }),
  );
});

afterEach(() => {
  vi.restoreAllMocks();
});

// ─── Test Consumer ────────────────────────────────────────────────────────────

function TestConsumer() {
  const { state, login, logout } = useAuth();
  return (
    <div>
      <span data-testid="authenticated">{String(state.isAuthenticated)}</span>
      <span data-testid="loading">{String(state.isAuthLoading)}</span>
      <span data-testid="user">{state.user?.fullName ?? "none"}</span>
      <span data-testid="error">{state.error ?? "none"}</span>
      <span data-testid="retry-after">
        {state.rateLimitRetryAfter !== null ? String(state.rateLimitRetryAfter) : "none"}
      </span>
      <button
        type="button"
        onClick={() => login("vendor.user", "password123").catch(() => {})}
        data-testid="login-valid"
      >
        Login
      </button>
      <button
        type="button"
        onClick={() => login("wrong", "wrong").catch(() => {})}
        data-testid="login-invalid"
      >
        Login Invalid
      </button>
      <button
        type="button"
        onClick={() => {
          void logout();
        }}
        data-testid="logout"
      >
        Logout
      </button>
    </div>
  );
}

// ─── Helper ───────────────────────────────────────────────────────────────────

function mockLoginSuccess() {
  mockFetch.mockImplementation(async (url: string) => {
    if (url.includes("/refresh")) {
      return new Response(JSON.stringify({ error: "unauthorized" }), { status: 401 });
    }
    if (url.includes("/login")) {
      return new Response(
        JSON.stringify({
          access_token: "mock-access-token-abc",
          user: {
            id: 1,
            username: "vendor.user",
            full_name: "Budi Santoso",
            email: "budi@vendor.com",
            role: "VENDOR-USER",
            is_karyawan: false,
            vendor_id: 42,
          },
        }),
        { status: 200 },
      );
    }
    return new Response(null, { status: 404 });
  });
}

function mockLoginUnauthorized() {
  mockFetch.mockImplementation(async (url: string) => {
    if (url.includes("/refresh")) {
      return new Response(JSON.stringify({ error: "unauthorized" }), { status: 401 });
    }
    if (url.includes("/login")) {
      return new Response(
        JSON.stringify({ error: "invalid_credentials", message: "Invalid credentials" }),
        { status: 401 },
      );
    }
    return new Response(null, { status: 404 });
  });
}

// ─── Tests ────────────────────────────────────────────────────────────────────

describe("AuthContext", () => {
  it("starts with loading state then resolves to unauthenticated", async () => {
    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>,
    );

    // Wait for initialization to finish
    await waitFor(() => {
      expect(screen.getByTestId("loading")).toHaveTextContent("false");
    });

    expect(screen.getByTestId("authenticated")).toHaveTextContent("false");
    expect(screen.getByTestId("user")).toHaveTextContent("none");
  });

  it("login with valid credentials sets authenticated state", async () => {
    mockLoginSuccess();

    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>,
    );

    // Wait for init
    await waitFor(() => {
      expect(screen.getByTestId("loading")).toHaveTextContent("false");
    });

    await act(async () => {
      screen.getByTestId("login-valid").click();
    });

    expect(screen.getByTestId("authenticated")).toHaveTextContent("true");
    expect(screen.getByTestId("user")).toHaveTextContent("Budi Santoso");
    expect(screen.getByTestId("error")).toHaveTextContent("none");
  });

  it("login with invalid credentials sets error", async () => {
    mockLoginUnauthorized();

    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("loading")).toHaveTextContent("false");
    });

    await act(async () => {
      screen.getByTestId("login-invalid").click();
    });

    expect(screen.getByTestId("authenticated")).toHaveTextContent("false");
    expect(screen.getByTestId("error")).toHaveTextContent("Email atau password salah");
  });

  it("login sends X-Portal-Type: vendor header", async () => {
    mockLoginSuccess();

    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("loading")).toHaveTextContent("false");
    });

    await act(async () => {
      screen.getByTestId("login-valid").click();
    });

    // Find the login call
    const loginCall = mockFetch.mock.calls.find(
      (call) => typeof call[0] === "string" && call[0].includes("/login"),
    );
    expect(loginCall).toBeDefined();
    expect(loginCall?.[1].headers["X-Portal-Type"]).toBe("vendor");
  });

  it("logout clears auth state", async () => {
    mockLoginSuccess();

    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("loading")).toHaveTextContent("false");
    });

    // Login first
    await act(async () => {
      screen.getByTestId("login-valid").click();
    });
    expect(screen.getByTestId("authenticated")).toHaveTextContent("true");

    // Logout
    await act(async () => {
      screen.getByTestId("logout").click();
    });

    expect(screen.getByTestId("authenticated")).toHaveTextContent("false");
    expect(screen.getByTestId("user")).toHaveTextContent("none");
  });

  it("handles rate limit (429) with Retry-After", async () => {
    mockFetch.mockImplementation(async (url: string) => {
      if (url.includes("/refresh")) {
        return new Response(JSON.stringify({ error: "unauthorized" }), { status: 401 });
      }
      if (url.includes("/login")) {
        return new Response(
          JSON.stringify({ error: "rate_limited", message: "Too many attempts" }),
          {
            status: 429,
            headers: { "Retry-After": "120" },
          },
        );
      }
      return new Response(null, { status: 404 });
    });

    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("loading")).toHaveTextContent("false");
    });

    await act(async () => {
      screen.getByTestId("login-valid").click();
    });

    expect(screen.getByTestId("error")).toHaveTextContent("Terlalu banyak percobaan login");
    expect(screen.getByTestId("retry-after")).toHaveTextContent("120");
  });

  it("handles portal mismatch (403)", async () => {
    mockFetch.mockImplementation(async (url: string) => {
      if (url.includes("/refresh")) {
        return new Response(JSON.stringify({ error: "unauthorized" }), { status: 401 });
      }
      if (url.includes("/login")) {
        return new Response(
          JSON.stringify({ error: "portal_mismatch", message: "Portal mismatch" }),
          { status: 403 },
        );
      }
      return new Response(null, { status: 404 });
    });

    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("loading")).toHaveTextContent("false");
    });

    await act(async () => {
      screen.getByTestId("login-valid").click();
    });

    expect(screen.getByTestId("error")).toHaveTextContent(
      "Akun tidak memiliki akses ke portal ini",
    );
  });

  it("initializes as authenticated when refresh succeeds", async () => {
    mockFetch.mockResolvedValue(
      new Response(
        JSON.stringify({
          access_token: "refreshed-token",
          user: {
            id: 1,
            username: "vendor.user",
            full_name: "Budi Santoso",
            email: "budi@vendor.com",
            role: "VENDOR-USER",
            is_karyawan: false,
            vendor_id: 42,
          },
        }),
        { status: 200 },
      ),
    );

    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("loading")).toHaveTextContent("false");
    });

    expect(screen.getByTestId("authenticated")).toHaveTextContent("true");
    expect(screen.getByTestId("user")).toHaveTextContent("Budi Santoso");
  });
});

describe("useAuth", () => {
  it("throws error when used outside AuthProvider", () => {
    function BadConsumer() {
      useAuth();
      return null;
    }

    // Suppress React error boundary console error
    const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    expect(() => render(<BadConsumer />)).toThrow("useAuth must be used within an AuthProvider");
    consoleSpy.mockRestore();
  });
});

// ─── fe-auth-unify U1 (FR-1): init /refresh single-flight ─────────────────────

function refreshCalls() {
  return mockFetch.mock.calls.filter(([url]) => String(url).includes("/auth/refresh")).length;
}

function mockRefreshAs(role: string) {
  mockFetch.mockImplementation(async (url: string) => {
    if (url.includes("/refresh")) {
      return new Response(
        JSON.stringify({
          access_token: "init-token",
          user: {
            id: 7,
            username: "vendor.init",
            full_name: "Init Vendor",
            email: "init@vendor.com",
            role,
            is_karyawan: false,
            vendor_id: 42,
          },
        }),
        { status: 200 },
      );
    }
    return new Response(null, { status: 404 });
  });
}

describe("AuthContext init under StrictMode (FR-1)", () => {
  beforeEach(() => {
    mockFetch.mockClear();
  });

  it("sends exactly one /refresh per load even when effects run twice", async () => {
    render(
      <StrictMode>
        <AuthProvider>
          <TestConsumer />
        </AuthProvider>
      </StrictMode>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("loading")).toHaveTextContent("false");
    });

    expect(refreshCalls()).toBe(1);
  });

  it("restores a VENDOR-USER session once under StrictMode", async () => {
    mockRefreshAs("VENDOR-USER");

    render(
      <StrictMode>
        <AuthProvider>
          <TestConsumer />
        </AuthProvider>
      </StrictMode>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("authenticated")).toHaveTextContent("true");
    });
    expect(screen.getByTestId("user")).toHaveTextContent("Init Vendor");
    expect(refreshCalls()).toBe(1);
  });

  it("still rejects a non-vendor session restored at init", async () => {
    mockRefreshAs("ATM-USER");

    render(
      <StrictMode>
        <AuthProvider>
          <TestConsumer />
        </AuthProvider>
      </StrictMode>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("loading")).toHaveTextContent("false");
    });
    expect(screen.getByTestId("authenticated")).toHaveTextContent("false");
  });

  it("sends a new /refresh on a later load (single-flight is not a permanent cache)", async () => {
    const first = render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>,
    );
    await waitFor(() => {
      expect(screen.getByTestId("loading")).toHaveTextContent("false");
    });
    first.unmount();

    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>,
    );
    await waitFor(() => {
      expect(screen.getByTestId("loading")).toHaveTextContent("false");
    });

    expect(refreshCalls()).toBe(2);
  });
});

// ─── fe-auth-unify U2 (FR-2): token usable without waiting for an effect ─────

function LoginThenFetchConsumer({ onDone }: { onDone: (r: unknown) => void }) {
  const { login, logout } = useAuth();
  return (
    <div>
      <button
        type="button"
        data-testid="login-then-fetch"
        onClick={async () => {
          await login("vendor.user", "password123");
          onDone(await api.get("/api/v1/vendor/probe").catch((e: unknown) => e));
        }}
      >
        go
      </button>
      <button
        type="button"
        data-testid="logout-then-fetch"
        onClick={async () => {
          await logout();
          onDone(await api.get("/api/v1/vendor/probe").catch((e: unknown) => e));
        }}
      >
        out
      </button>
    </div>
  );
}

function probeAuthHeaders() {
  return mockFetch.mock.calls
    .filter(([url]) => String(url).includes("/vendor/probe"))
    .map(([, init]) => (init as RequestInit).headers as Record<string, string>)
    .map((h) => h.Authorization ?? null);
}

describe("AuthContext token wiring (FR-2)", () => {
  beforeEach(() => {
    mockFetch.mockClear();
    mockFetch.mockImplementation(async (url: string, init?: RequestInit) => {
      if (url.includes("/refresh")) {
        return new Response(JSON.stringify({ error: "unauthorized" }), { status: 401 });
      }
      if (url.includes("/login")) {
        return new Response(
          JSON.stringify({
            access_token: "fresh-token",
            user: {
              id: 1,
              username: "vendor.user",
              full_name: "Budi Santoso",
              email: "budi@vendor.com",
              role: "VENDOR-USER",
              is_karyawan: false,
              vendor_id: 42,
            },
          }),
          { status: 200 },
        );
      }
      if (url.includes("/logout")) return new Response(null, { status: 204 });
      const auth = (init?.headers as Record<string, string> | undefined)?.Authorization;
      return auth === "Bearer fresh-token"
        ? new Response(JSON.stringify({ ok: true }), { status: 200 })
        : new Response(JSON.stringify({ error: "unauthorized" }), { status: 401 });
    });
  });

  it("first request right after login carries the new Bearer token, no refresh", async () => {
    const onDone = vi.fn();
    render(
      <AuthProvider>
        <LoginThenFetchConsumer onDone={onDone} />
      </AuthProvider>,
    );
    await waitFor(() => expect(mockFetch).toHaveBeenCalled());
    const refreshBefore = mockFetch.mock.calls.filter(([u]) =>
      String(u).includes("/refresh"),
    ).length;

    await act(async () => {
      screen.getByTestId("login-then-fetch").click();
    });
    await waitFor(() => expect(onDone).toHaveBeenCalled());

    expect(probeAuthHeaders()[0]).toBe("Bearer fresh-token");
    expect(mockFetch.mock.calls.filter(([u]) => String(u).includes("/refresh")).length).toBe(
      refreshBefore,
    );
  });

  it("first request right after logout carries no token", async () => {
    const onDone = vi.fn();
    render(
      <AuthProvider>
        <LoginThenFetchConsumer onDone={onDone} />
      </AuthProvider>,
    );
    await act(async () => {
      screen.getByTestId("login-then-fetch").click();
    });
    await waitFor(() => expect(onDone).toHaveBeenCalledTimes(1));

    await act(async () => {
      screen.getByTestId("logout-then-fetch").click();
    });
    await waitFor(() => expect(onDone).toHaveBeenCalledTimes(2));

    expect(probeAuthHeaders().at(-1)).toBeNull();
  });
});

// ─── fe-coverage C1: login error mapping, refresh paths, useAuthRefresh ──────

function respondLoginWith(response: () => Response | Promise<Response>) {
  mockFetch.mockImplementation(async (url: string) => {
    if (url.includes("/refresh")) {
      return new Response(JSON.stringify({ error: "unauthorized" }), { status: 401 });
    }
    if (url.includes("/login")) return response();
    return new Response(null, { status: 204 });
  });
}

async function renderReady(ui = <TestConsumer />) {
  render(<AuthProvider>{ui}</AuthProvider>);
  await waitFor(() => {
    expect(screen.getByTestId("loading")).toHaveTextContent("false");
  });
}

const jsonRes = (body: unknown, status: number, headers?: Record<string, string>) =>
  new Response(JSON.stringify(body), { status, headers });

describe("AuthContext login error mapping (C1)", () => {
  beforeEach(() => {
    mockFetch.mockClear();
  });

  it.each([
    ["403 account_inactive", () => jsonRes({ error: "account_inactive" }, 403), "Akun tidak aktif"],
    [
      "403 portal_mismatch",
      () => jsonRes({ error: "portal_mismatch" }, 403),
      "Akun tidak memiliki akses ke portal ini",
    ],
    ["403 other with message", () => jsonRes({ error: "x", message: "Diblokir" }, 403), "Diblokir"],
    ["403 without body", () => new Response("not json", { status: 403 }), "Akses ditolak"],
    ["422 with message", () => jsonRes({ message: "Email tidak valid" }, 422), "Email tidak valid"],
    ["422 without body", () => new Response("", { status: 422 }), "Validasi gagal"],
    ["503", () => new Response("", { status: 503 }), "Layanan sedang tidak tersedia"],
    [
      "500 with message",
      () => jsonRes({ message: "Kesalahan internal" }, 500),
      "Kesalahan internal",
    ],
    [
      "500 without body",
      () => new Response("", { status: 500 }),
      "Terjadi kesalahan. Silakan coba lagi.",
    ],
    [
      "network failure",
      () => Promise.reject(new TypeError("Failed to fetch")),
      "Gagal terhubung ke server. Silakan coba lagi.",
    ],
  ])("%s → shows the mapped error and stays logged out", async (_name, response, message) => {
    respondLoginWith(response);
    await renderReady();

    await act(async () => {
      screen.getByTestId("login-valid").click();
    });

    await waitFor(() => {
      expect(screen.getByTestId("error")).toHaveTextContent(message);
    });
    expect(screen.getByTestId("authenticated")).toHaveTextContent("false");
  });

  it("429 exposes Retry-After seconds", async () => {
    respondLoginWith(() => jsonRes({ error: "rate_limited" }, 429, { "Retry-After": "42" }));
    await renderReady();

    await act(async () => {
      screen.getByTestId("login-valid").click();
    });

    await waitFor(() => {
      expect(screen.getByTestId("error")).toHaveTextContent("Terlalu banyak percobaan login");
    });
    expect(screen.getByTestId("retry-after")).toHaveTextContent("42");
  });

  it("429 without a usable Retry-After leaves retry-after empty", async () => {
    respondLoginWith(() => jsonRes({ error: "rate_limited" }, 429, { "Retry-After": "soon" }));
    await renderReady();

    await act(async () => {
      screen.getByTestId("login-valid").click();
    });

    await waitFor(() => {
      expect(screen.getByTestId("error")).toHaveTextContent("Terlalu banyak percobaan login");
    });
    expect(screen.getByTestId("retry-after")).toHaveTextContent("none");
  });

  it("logout clears state even when POST /logout fails", async () => {
    mockLoginSuccess();
    await renderReady();
    await act(async () => {
      screen.getByTestId("login-valid").click();
    });
    await waitFor(() => {
      expect(screen.getByTestId("authenticated")).toHaveTextContent("true");
    });
    mockFetch.mockImplementation(async (url: string) => {
      if (url.includes("/logout")) throw new TypeError("Failed to fetch");
      return new Response(null, { status: 204 });
    });

    await act(async () => {
      screen.getByTestId("logout").click();
    });

    expect(screen.getByTestId("authenticated")).toHaveTextContent("false");
    expect(screen.getByTestId("user")).toHaveTextContent("none");
  });
});

function RefreshConsumer({ onResult }: { onResult: (ok: boolean) => void }) {
  const { state } = useAuth();
  const { handleUnauthorized } = useAuthRefresh();
  return (
    <div>
      <span data-testid="loading">{String(state.isAuthLoading)}</span>
      <span data-testid="authenticated">{String(state.isAuthenticated)}</span>
      <span data-testid="user">{state.user?.fullName ?? "none"}</span>
      <span data-testid="error">{state.error ?? "none"}</span>
      <button
        type="button"
        data-testid="refresh"
        onClick={async () => onResult(await handleUnauthorized())}
      >
        refresh
      </button>
    </div>
  );
}

function respondRefreshWith(response: () => Response | Promise<Response>) {
  let first = true;
  mockFetch.mockImplementation(async (url: string) => {
    if (url.includes("/refresh")) {
      if (first) {
        first = false; // mount-time init: not logged in
        return new Response(JSON.stringify({ error: "unauthorized" }), { status: 401 });
      }
      return response();
    }
    return new Response(null, { status: 204 });
  });
}

const refreshUser = (role: string) => ({
  access_token: "refreshed",
  user: {
    id: 3,
    username: "v",
    full_name: "Refreshed Vendor",
    email: "v@vendor.com",
    role,
    is_karyawan: false,
    vendor_id: 9,
  },
});

describe("AuthContext refreshToken + useAuthRefresh (C1)", () => {
  beforeEach(() => {
    mockFetch.mockClear();
  });

  it("successful refresh restores the vendor session", async () => {
    respondRefreshWith(() => jsonRes(refreshUser("VENDOR-USER"), 200));
    const onResult = vi.fn();
    await renderReady(<RefreshConsumer onResult={onResult} />);

    await act(async () => {
      screen.getByTestId("refresh").click();
    });

    await waitFor(() => expect(onResult).toHaveBeenCalledWith(true));
    expect(screen.getByTestId("user")).toHaveTextContent("Refreshed Vendor");
  });

  it.each([
    ["401", () => jsonRes({ error: "expired" }, 401)],
    ["non-vendor role", () => jsonRes(refreshUser("ATM-USER"), 200)],
    ["network failure", () => Promise.reject(new TypeError("Failed to fetch"))],
  ])("refresh %s → false, logged out, POST /logout sent", async (_name, response) => {
    respondRefreshWith(response);
    const onResult = vi.fn();
    await renderReady(<RefreshConsumer onResult={onResult} />);

    await act(async () => {
      screen.getByTestId("refresh").click();
    });

    await waitFor(() => expect(onResult).toHaveBeenCalledWith(false));
    expect(screen.getByTestId("authenticated")).toHaveTextContent("false");
    expect(mockFetch.mock.calls.some(([u]) => String(u).includes("/logout"))).toBe(true);
  });
});
