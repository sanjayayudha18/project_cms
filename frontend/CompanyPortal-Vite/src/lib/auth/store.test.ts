import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { type AuthUser, type DbRole, useAuthStore } from "./store";

// Mock global fetch
const mockFetch = vi.fn();
global.fetch = mockFetch;

// Mock window.location
const mockLocation = { href: "", pathname: "/dashboard", search: "" };
Object.defineProperty(window, "location", {
  value: mockLocation,
  writable: true,
});

const testUser: AuthUser = {
  id: 1,
  username: "john.admin",
  fullName: "John Admin",
  email: "john.admin@cimb.local",
  role: "ADMIN",
  isKaryawan: true,
  vendorId: null,
};

const vendorUser: AuthUser = {
  id: 2,
  username: "vendor.user",
  fullName: "Vendor User",
  email: "vendor@partner.co",
  role: "VENDOR-USER",
  isKaryawan: false,
  vendorId: 10,
};

describe("useAuthStore", () => {
  beforeEach(() => {
    useAuthStore.setState({
      user: null,
      accessToken: null,
      isAuthenticated: false,
      isAuthLoading: true,
      error: null,
      rateLimitRetryAfter: null,
    });
    mockFetch.mockReset();
    mockLocation.href = "";
    mockLocation.pathname = "/dashboard";
    mockLocation.search = "";
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe("initial state", () => {
    it("starts with null user, no token, not authenticated, loading true", () => {
      const state = useAuthStore.getState();
      expect(state.user).toBeNull();
      expect(state.accessToken).toBeNull();
      expect(state.isAuthenticated).toBe(false);
      expect(state.isAuthLoading).toBe(true);
      expect(state.error).toBeNull();
      expect(state.rateLimitRetryAfter).toBeNull();
    });
  });

  describe("login", () => {
    it("stores user and token on successful login", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          access_token: "jwt-token-123",
          user: {
            id: 1,
            username: "john.admin",
            full_name: "John Admin",
            email: "john.admin@cimb.local",
            role: "ADMIN",
            is_karyawan: true,
            vendor_id: null,
          },
        }),
      });

      await useAuthStore.getState().login("john.admin", "Password123!");

      const state = useAuthStore.getState();
      expect(state.user).toEqual(testUser);
      expect(state.accessToken).toBe("jwt-token-123");
      expect(state.isAuthenticated).toBe(true);
      expect(state.isAuthLoading).toBe(false);
      expect(state.error).toBeNull();
    });

    it("maps snake_case backend response to camelCase AuthUser", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          access_token: "token",
          user: {
            id: 2,
            username: "vendor.user",
            full_name: "Vendor User",
            email: "vendor@partner.co",
            role: "VENDOR-USER",
            is_karyawan: false,
            vendor_id: 10,
          },
        }),
      });

      await useAuthStore.getState().login("vendor.user", "pass123");

      const state = useAuthStore.getState();
      expect(state.user).toEqual(vendorUser);
      expect(state.user?.vendorId).toBe(10);
      expect(state.user?.isKaryawan).toBe(false);
    });

    it("sets error on 401 (invalid credentials)", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        json: async () => ({ error: "auth_failed", message: "Username atau password salah" }),
      });

      await useAuthStore.getState().login("wrong", "wrong");

      const state = useAuthStore.getState();
      expect(state.error).toBe("Username atau password salah");
      expect(state.user).toBeNull();
      expect(state.isAuthenticated).toBe(false);
    });

    it("sets error on 403 account_inactive", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 403,
        json: async () => ({ error: "account_inactive", message: "Akun tidak aktif" }),
      });

      await useAuthStore.getState().login("inactive", "pass");

      expect(useAuthStore.getState().error).toBe("Akun tidak aktif");
    });

    it("sets error on 403 portal_mismatch", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 403,
        json: async () => ({
          error: "portal_mismatch",
          message: "Akun tidak memiliki akses ke portal ini",
        }),
      });

      await useAuthStore.getState().login("vendor", "pass");

      expect(useAuthStore.getState().error).toBe("Akun tidak memiliki akses ke portal ini");
    });

    it("sets error on 422 validation with field details", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 422,
        json: async () => ({
          error: "validation_error",
          message: "Validasi gagal",
          details: [
            { field: "username", message: "wajib diisi" },
            { field: "password", message: "wajib diisi" },
          ],
        }),
      });

      await useAuthStore.getState().login("", "");

      const state = useAuthStore.getState();
      expect(state.error).toContain("username");
      expect(state.error).toContain("password");
    });

    it("handles 429 rate limit with Retry-After header", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 429,
        headers: new Headers({ "Retry-After": "540" }),
        json: async () => ({ error: "rate_limited", message: "Terlalu banyak percobaan login" }),
      });

      await useAuthStore.getState().login("user", "pass");

      const state = useAuthStore.getState();
      expect(state.error).toBe("Terlalu banyak percobaan login");
      expect(state.rateLimitRetryAfter).toBe(540);
    });

    it("handles 429 without Retry-After header gracefully", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 429,
        headers: new Headers(),
        json: async () => ({ error: "rate_limited", message: "Terlalu banyak percobaan login" }),
      });

      await useAuthStore.getState().login("user", "pass");

      const state = useAuthStore.getState();
      expect(state.error).toBe("Terlalu banyak percobaan login");
      expect(state.rateLimitRetryAfter).toBeNull();
    });

    it("handles 503 service unavailable", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 503,
        json: async () => ({
          error: "service_unavailable",
          message: "Layanan sedang tidak tersedia",
        }),
      });

      await useAuthStore.getState().login("user", "pass");

      expect(useAuthStore.getState().error).toBe("Layanan sedang tidak tersedia");
    });

    it("handles network error", async () => {
      mockFetch.mockRejectedValueOnce(new Error("Network error"));

      await useAuthStore.getState().login("user", "pass");

      expect(useAuthStore.getState().error).toBe("Gagal terhubung ke server. Silakan coba lagi.");
    });

    it("sends X-Portal-Type: company header", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          access_token: "token",
          user: {
            id: 1,
            username: "john.admin",
            full_name: "John Admin",
            email: "john@cimb.local",
            role: "ADMIN",
            is_karyawan: true,
            vendor_id: null,
          },
        }),
      });

      await useAuthStore.getState().login("john.admin", "pass");

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/v1/auth/login",
        expect.objectContaining({
          method: "POST",
          credentials: "include",
          headers: expect.objectContaining({
            "Content-Type": "application/json",
            "X-Portal-Type": "company",
          }),
        }),
      );
    });

    it("sends username and password in body", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          access_token: "token",
          user: {
            id: 1,
            username: "john.admin",
            full_name: "John Admin",
            email: "john@cimb.local",
            role: "ADMIN",
            is_karyawan: true,
            vendor_id: null,
          },
        }),
      });

      await useAuthStore.getState().login("john.admin", "Password123!");

      const [, options] = mockFetch.mock.calls[0] as [string, RequestInit];
      const body = JSON.parse(options.body as string);
      expect(body).toEqual({ username: "john.admin", password: "Password123!" });
    });
  });

  describe("logout", () => {
    it("clears state and redirects to /login", async () => {
      useAuthStore.setState({
        user: testUser,
        accessToken: "token",
        isAuthenticated: true,
        isAuthLoading: false,
        error: null,
        rateLimitRetryAfter: null,
      });

      mockFetch.mockResolvedValueOnce({ ok: true });

      await useAuthStore.getState().logout();

      const state = useAuthStore.getState();
      expect(state.user).toBeNull();
      expect(state.accessToken).toBeNull();
      expect(state.isAuthenticated).toBe(false);
      expect(state.error).toBeNull();
      expect(state.rateLimitRetryAfter).toBeNull();
      expect(mockLocation.href).toBe("/login");
    });

    it("fires server-side logout request with include credentials", async () => {
      useAuthStore.setState({
        user: testUser,
        accessToken: "token",
        isAuthenticated: true,
        isAuthLoading: false,
        error: null,
        rateLimitRetryAfter: null,
      });

      mockFetch.mockResolvedValueOnce({ ok: true });

      await useAuthStore.getState().logout();

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/v1/auth/logout",
        expect.objectContaining({
          method: "POST",
          credentials: "include",
        }),
      );
    });

    it("clears state even when server logout fails", async () => {
      useAuthStore.setState({
        user: testUser,
        accessToken: "token",
        isAuthenticated: true,
        isAuthLoading: false,
        error: null,
        rateLimitRetryAfter: null,
      });

      mockFetch.mockRejectedValueOnce(new Error("Network error"));

      await useAuthStore.getState().logout();

      const state = useAuthStore.getState();
      expect(state.user).toBeNull();
      expect(state.isAuthenticated).toBe(false);
    });
  });

  describe("refreshToken", () => {
    it("updates token and user on successful refresh", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          access_token: "new-token",
          user: {
            id: 1,
            username: "john.admin",
            full_name: "John Admin",
            email: "john.admin@cimb.local",
            role: "ADMIN",
            is_karyawan: true,
            vendor_id: null,
          },
        }),
      });

      const success = await useAuthStore.getState().refreshToken();

      expect(success).toBe(true);
      const state = useAuthStore.getState();
      expect(state.accessToken).toBe("new-token");
      expect(state.user).toEqual(testUser);
      expect(state.isAuthenticated).toBe(true);
    });

    it("clears state and redirects on failed refresh", async () => {
      mockFetch.mockResolvedValueOnce({ ok: false, status: 401 });

      const success = await useAuthStore.getState().refreshToken();

      expect(success).toBe(false);
      expect(mockLocation.href).toContain("/login");
      expect(useAuthStore.getState().isAuthenticated).toBe(false);
    });

    it("redirects with current path as redirect param", async () => {
      mockLocation.pathname = "/dashboard";
      mockLocation.search = "?tab=overview";

      mockFetch.mockResolvedValueOnce({ ok: false, status: 401 });

      await useAuthStore.getState().refreshToken();

      expect(mockLocation.href).toBe("/login?redirect=%2Fdashboard%3Ftab%3Doverview");
    });

    it("single-flight: concurrent refreshes share the same promise", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          access_token: "shared-token",
          user: {
            id: 1,
            username: "john.admin",
            full_name: "John Admin",
            email: "john.admin@cimb.local",
            role: "ADMIN",
            is_karyawan: true,
            vendor_id: null,
          },
        }),
      });

      // Trigger multiple concurrent refreshes
      const [r1, r2, r3] = await Promise.all([
        useAuthStore.getState().refreshToken(),
        useAuthStore.getState().refreshToken(),
        useAuthStore.getState().refreshToken(),
      ]);

      // Only one fetch call should have been made
      expect(mockFetch).toHaveBeenCalledTimes(1);
      expect(r1).toBe(true);
      expect(r2).toBe(true);
      expect(r3).toBe(true);
    });
  });

  describe("initialize", () => {
    it("restores session from refresh cookie on success", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          access_token: "restored-token",
          user: {
            id: 1,
            username: "john.admin",
            full_name: "John Admin",
            email: "john.admin@cimb.local",
            role: "ADMIN",
            is_karyawan: true,
            vendor_id: null,
          },
        }),
      });

      await useAuthStore.getState().initialize();

      const state = useAuthStore.getState();
      expect(state.isAuthLoading).toBe(false);
      expect(state.isAuthenticated).toBe(true);
      expect(state.accessToken).toBe("restored-token");
      expect(state.user).toEqual(testUser);
    });

    it("sets unauthenticated state when no valid session", async () => {
      mockFetch.mockResolvedValueOnce({ ok: false, status: 401 });

      await useAuthStore.getState().initialize();

      const state = useAuthStore.getState();
      expect(state.isAuthLoading).toBe(false);
      expect(state.isAuthenticated).toBe(false);
      expect(state.user).toBeNull();
      expect(state.accessToken).toBeNull();
    });

    it("sets isAuthLoading true during initialization", async () => {
      let loadingDuringFetch = false;
      mockFetch.mockImplementationOnce(() => {
        loadingDuringFetch = useAuthStore.getState().isAuthLoading;
        return Promise.resolve({ ok: false, status: 401 });
      });

      await useAuthStore.getState().initialize();

      expect(loadingDuringFetch).toBe(true);
    });

    it("handles network error gracefully during init", async () => {
      mockFetch.mockRejectedValueOnce(new Error("Network error"));

      await useAuthStore.getState().initialize();

      const state = useAuthStore.getState();
      expect(state.isAuthLoading).toBe(false);
      expect(state.isAuthenticated).toBe(false);
    });
  });

  describe("clearError", () => {
    it("clears error and rateLimitRetryAfter", () => {
      useAuthStore.setState({
        error: "Some error",
        rateLimitRetryAfter: 300,
      });

      useAuthStore.getState().clearError();

      const state = useAuthStore.getState();
      expect(state.error).toBeNull();
      expect(state.rateLimitRetryAfter).toBeNull();
    });
  });

  describe("DbRole type coverage", () => {
    const ALL_DB_ROLES: DbRole[] = [
      "ADMIN",
      "ADMIN_PARAM",
      "ATM-USER",
      "ATM-SPV",
      "BRANCH-USER",
      "BRANCH-SPV",
      "BRANCH-ATM-USER",
      "BRANCH-ATM-SPV",
      "VENDOR-USER",
    ];

    it("accepts all 9 DB roles in user object", () => {
      for (const role of ALL_DB_ROLES) {
        const user: AuthUser = {
          id: 1,
          username: "test",
          fullName: "Test",
          email: "test@test.com",
          role,
          isKaryawan: role !== "VENDOR-USER",
          vendorId: role === "VENDOR-USER" ? 1 : null,
        };
        expect(user.role).toBe(role);
      }
    });

    it("user has single role (not array)", () => {
      useAuthStore.setState({
        user: testUser,
        isAuthenticated: true,
      });
      const user = useAuthStore.getState().user;
      expect(typeof user?.role).toBe("string");
      expect(Array.isArray(user?.role)).toBe(false);
    });
  });

  describe("XSS resistance", () => {
    it("stores access token only in memory, not in localStorage", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          access_token: "sensitive-token",
          user: {
            id: 1,
            username: "john.admin",
            full_name: "John Admin",
            email: "john@cimb.local",
            role: "ADMIN",
            is_karyawan: true,
            vendor_id: null,
          },
        }),
      });

      await useAuthStore.getState().login("john.admin", "pass");

      expect(useAuthStore.getState().accessToken).toBe("sensitive-token");
      expect(localStorage.getItem("accessToken")).toBeNull();
      expect(localStorage.getItem("token")).toBeNull();
    });
  });
});
