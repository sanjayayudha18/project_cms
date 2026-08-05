import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { type AuthUser, ROLE_NAV_PERMISSIONS, type Role, useAuthStore } from "./store";

// Force the real-backend code path so the fetch mocks below are exercised.
// (In the test env VITE_API_MODE is undefined, which would otherwise resolve to "stub".)
vi.mock("@/lib/api/config", () => ({
  apiConfig: { mode: "real", baseURL: "/api/v1", stubLatency: { min: 200, max: 800 } },
}));

// Mock global fetch
const mockFetch = vi.fn();
global.fetch = mockFetch;

// Mock window.location
const mockLocation = { href: "" };
Object.defineProperty(window, "location", {
  value: mockLocation,
  writable: true,
});

const testUser: AuthUser = {
  id: "user-1",
  fullName: "Test User",
  email: "test@cimb.com",
  roles: ["ATM_Support"],
  primaryRole: "ATM_Support",
};

describe("useAuthStore", () => {
  beforeEach(() => {
    useAuthStore.setState({
      user: null,
      accessToken: null,
      isAuthenticated: false,
      isLoading: true,
    });
    mockFetch.mockReset();
    mockLocation.href = "";
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
      expect(state.isLoading).toBe(true);
    });
  });

  describe("login", () => {
    it("stores user and token on successful login", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ user: testUser, accessToken: "jwt-token-123" }),
      });

      const result = await useAuthStore.getState().login({
        email: "test@cimb.com",
        password: "password123",
      });

      expect(result).toEqual({ success: true, user: testUser });

      const state = useAuthStore.getState();
      expect(state.user).toEqual(testUser);
      expect(state.accessToken).toBe("jwt-token-123");
      expect(state.isAuthenticated).toBe(true);
      expect(state.isLoading).toBe(false);
    });

    it("returns error on failed login", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        json: async () => ({ message: "Kredensial tidak valid" }),
      });

      const result = await useAuthStore.getState().login({
        email: "wrong@cimb.com",
        password: "wrong",
      });

      expect(result).toEqual({
        success: false,
        error: "Kredensial tidak valid",
      });

      const state = useAuthStore.getState();
      expect(state.user).toBeNull();
      expect(state.isAuthenticated).toBe(false);
    });

    it("sends credentials with include for httpOnly cookies", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ user: testUser, accessToken: "token" }),
      });

      await useAuthStore.getState().login({
        email: "test@cimb.com",
        password: "pass",
      });

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/v1/auth/login",
        expect.objectContaining({
          method: "POST",
          credentials: "include",
          headers: { "Content-Type": "application/json" },
        }),
      );
    });

    it("returns default error when response body is not JSON", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        json: async () => {
          throw new Error("not json");
        },
      });

      const result = await useAuthStore.getState().login({
        email: "test@cimb.com",
        password: "pass",
      });

      expect(result).toEqual({
        success: false,
        error: "Kredensial tidak valid",
      });
    });
  });

  describe("logout", () => {
    it("clears state and redirects to /login", () => {
      useAuthStore.setState({
        user: testUser,
        accessToken: "token",
        isAuthenticated: true,
        isLoading: false,
      });

      mockFetch.mockResolvedValueOnce({ ok: true });

      useAuthStore.getState().logout();

      const state = useAuthStore.getState();
      expect(state.user).toBeNull();
      expect(state.accessToken).toBeNull();
      expect(state.isAuthenticated).toBe(false);
      expect(mockLocation.href).toBe("/login");
    });

    it("fires server-side logout request with include credentials", () => {
      useAuthStore.setState({
        user: testUser,
        accessToken: "token",
        isAuthenticated: true,
        isLoading: false,
      });

      mockFetch.mockResolvedValueOnce({ ok: true });

      useAuthStore.getState().logout();

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/v1/auth/logout",
        expect.objectContaining({
          method: "POST",
          credentials: "include",
        }),
      );
    });
  });

  describe("refreshToken", () => {
    it("updates token and user on successful refresh", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          accessToken: "new-token",
          user: testUser,
        }),
      });

      const success = await useAuthStore.getState().refreshToken();

      expect(success).toBe(true);
      const state = useAuthStore.getState();
      expect(state.accessToken).toBe("new-token");
      expect(state.user).toEqual(testUser);
      expect(state.isAuthenticated).toBe(true);
    });

    it("calls logout on failed refresh (session expired)", async () => {
      mockFetch
        .mockResolvedValueOnce({ ok: false }) // refresh fails
        .mockResolvedValueOnce({ ok: true }); // logout fire-and-forget

      const success = await useAuthStore.getState().refreshToken();

      expect(success).toBe(false);
      expect(mockLocation.href).toBe("/login");
      expect(useAuthStore.getState().isAuthenticated).toBe(false);
    });
  });

  describe("initialize", () => {
    it("restores session from refresh cookie on success", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          accessToken: "restored-token",
          user: testUser,
        }),
      });

      await useAuthStore.getState().initialize();

      const state = useAuthStore.getState();
      expect(state.isLoading).toBe(false);
      expect(state.isAuthenticated).toBe(true);
      expect(state.accessToken).toBe("restored-token");
      expect(state.user).toEqual(testUser);
    });

    it("sets unauthenticated state when no valid session", async () => {
      mockFetch.mockResolvedValueOnce({ ok: false });

      await useAuthStore.getState().initialize();

      const state = useAuthStore.getState();
      expect(state.isLoading).toBe(false);
      expect(state.isAuthenticated).toBe(false);
      expect(state.user).toBeNull();
      expect(state.accessToken).toBeNull();
    });

    it("sets isLoading true during initialization", async () => {
      let loadingDuringFetch = false;
      mockFetch.mockImplementationOnce(() => {
        loadingDuringFetch = useAuthStore.getState().isLoading;
        return Promise.resolve({ ok: false });
      });

      await useAuthStore.getState().initialize();

      expect(loadingDuringFetch).toBe(true);
    });
  });

  describe("ROLE_NAV_PERMISSIONS", () => {
    it("defines permissions for all 10 roles", () => {
      const allRoles: Role[] = [
        "Admin",
        "ATM_Support",
        "Cash_Management",
        "Vendor",
        "WMO",
        "Finance",
        "Cash_Count_PIC",
        "Cash_Count_Lead",
        "Branch",
        "Approver",
      ];

      for (const role of allRoles) {
        expect(ROLE_NAV_PERMISSIONS[role]).toBeDefined();
        expect(ROLE_NAV_PERMISSIONS[role].length).toBeGreaterThan(0);
      }
    });

    it("Admin has wildcard access", () => {
      expect(ROLE_NAV_PERMISSIONS.Admin).toContain("*");
    });

    it("Vendor has specific route access without wildcard", () => {
      expect(ROLE_NAV_PERMISSIONS.Vendor).not.toContain("*");
      expect(ROLE_NAV_PERMISSIONS.Vendor).toContain("dsr-upload");
      expect(ROLE_NAV_PERMISSIONS.Vendor).toContain("invoice-upload");
    });
  });

  describe("XSS resistance", () => {
    it("stores access token only in memory, not in localStorage", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ user: testUser, accessToken: "sensitive-token" }),
      });

      await useAuthStore.getState().login({
        email: "test@cimb.com",
        password: "pass",
      });

      expect(useAuthStore.getState().accessToken).toBe("sensitive-token");
      expect(localStorage.getItem("accessToken")).toBeNull();
      expect(localStorage.getItem("token")).toBeNull();
    });
  });
});
