import { create } from "zustand";

// ─── Types ────────────────────────────────────────────────────────────────────

export type DbRole =
  | "ADMIN"
  | "ADMIN_PARAM"
  | "ATM-USER"
  | "ATM-SPV"
  | "BRANCH-USER"
  | "BRANCH-SPV"
  | "BRANCH-ATM-USER"
  | "BRANCH-ATM-SPV"
  | "VENDOR-USER";

export interface AuthUser {
  id: number;
  username: string;
  fullName: string;
  email: string;
  role: DbRole;
  isKaryawan: boolean;
  vendorId: number | null;
}

export interface AuthState {
  user: AuthUser | null;
  accessToken: string | null;
  isAuthenticated: boolean;
  isAuthLoading: boolean;
  error: string | null;
  rateLimitRetryAfter: number | null;
}

export interface AuthActions {
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  refreshToken: () => Promise<boolean>;
  initialize: () => Promise<void>;
  clearError: () => void;
}

export type AuthStore = AuthState & AuthActions;

// ─── API Endpoints ────────────────────────────────────────────────────────────

const AUTH_API_BASE = "/api/v1/auth";

// ─── Single-flight refresh ────────────────────────────────────────────────────

let refreshPromise: Promise<boolean> | null = null;

// ─── Backend response types ───────────────────────────────────────────────────

interface LoginSuccessResponse {
  access_token: string;
  user: {
    id: number;
    username: string;
    full_name: string;
    email: string;
    role: DbRole;
    is_karyawan: boolean;
    vendor_id: number | null;
  };
}

interface ApiErrorResponse {
  error: string;
  message: string;
  details?: Array<{ field: string; message: string }>;
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

function mapUserResponse(raw: LoginSuccessResponse["user"]): AuthUser {
  return {
    id: raw.id,
    username: raw.username,
    fullName: raw.full_name,
    email: raw.email,
    role: raw.role,
    isKaryawan: raw.is_karyawan,
    vendorId: raw.vendor_id,
  };
}

// ─── Store ────────────────────────────────────────────────────────────────────

export const useAuthStore = create<AuthStore>((set, _get) => ({
  user: null,
  accessToken: null,
  isAuthenticated: false,
  isAuthLoading: true,
  error: null,
  rateLimitRetryAfter: null,

  login: async (username: string, password: string) => {
    set({ error: null, rateLimitRetryAfter: null });

    try {
      const response = await fetch(`${AUTH_API_BASE}/login`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Portal-Type": "company",
        },
        body: JSON.stringify({ username, password }),
        credentials: "include",
      });

      if (response.ok) {
        const data = (await response.json()) as LoginSuccessResponse;
        set({
          user: mapUserResponse(data.user),
          accessToken: data.access_token,
          isAuthenticated: true,
          isAuthLoading: false,
          error: null,
          rateLimitRetryAfter: null,
        });
        return;
      }

      // Handle error responses
      switch (response.status) {
        case 401: {
          set({ error: "Username atau password salah" });
          return;
        }
        case 403: {
          const body = (await response.json().catch(() => null)) as ApiErrorResponse | null;
          if (body?.error === "account_inactive") {
            set({ error: "Akun tidak aktif" });
          } else if (body?.error === "portal_mismatch") {
            set({ error: "Akun tidak memiliki akses ke portal ini" });
          } else {
            set({ error: body?.message ?? "Akses ditolak" });
          }
          return;
        }
        case 422: {
          const body = (await response.json().catch(() => null)) as ApiErrorResponse | null;
          if (body?.details && body.details.length > 0) {
            const messages = body.details.map((d) => `${d.field}: ${d.message}`).join("; ");
            set({ error: messages });
          } else {
            set({ error: body?.message ?? "Validasi gagal" });
          }
          return;
        }
        case 429: {
          const retryAfter = response.headers.get("Retry-After");
          const seconds = retryAfter ? Number.parseInt(retryAfter, 10) : null;
          set({
            error: "Terlalu banyak percobaan login",
            rateLimitRetryAfter: Number.isFinite(seconds) ? seconds : null,
          });
          return;
        }
        case 503: {
          set({ error: "Layanan sedang tidak tersedia" });
          return;
        }
        default: {
          const body = (await response.json().catch(() => null)) as ApiErrorResponse | null;
          set({ error: body?.message ?? "Terjadi kesalahan. Silakan coba lagi." });
        }
      }
    } catch {
      set({ error: "Gagal terhubung ke server. Silakan coba lagi." });
    }
  },

  logout: async () => {
    set({
      user: null,
      accessToken: null,
      isAuthenticated: false,
      isAuthLoading: false,
      error: null,
      rateLimitRetryAfter: null,
    });

    try {
      await fetch(`${AUTH_API_BASE}/logout`, {
        method: "POST",
        credentials: "include",
      });
    } catch {
      // Intentionally ignore — local state is already cleared
    }

    window.location.href = "/login";
  },

  refreshToken: async () => {
    // Single-flight: if a refresh is already in progress, return its promise
    if (refreshPromise) {
      return refreshPromise;
    }

    refreshPromise = (async () => {
      try {
        const response = await fetch(`${AUTH_API_BASE}/refresh`, {
          method: "POST",
          credentials: "include",
        });

        if (!response.ok) {
          set({
            user: null,
            accessToken: null,
            isAuthenticated: false,
            isAuthLoading: false,
            error: null,
            rateLimitRetryAfter: null,
          });
          const currentPath = window.location.pathname + window.location.search;
          window.location.href = `/login?redirect=${encodeURIComponent(currentPath)}`;
          return false;
        }

        const data = (await response.json()) as LoginSuccessResponse;

        // Guard: reject VENDOR-USER on company portal refresh
        if (data.user.role === "VENDOR-USER") {
          set({
            user: null,
            accessToken: null,
            isAuthenticated: false,
            isAuthLoading: false,
            error: null,
            rateLimitRetryAfter: null,
          });
          const currentPath = window.location.pathname + window.location.search;
          window.location.href = `/login?redirect=${encodeURIComponent(currentPath)}`;
          return false;
        }

        set({
          user: mapUserResponse(data.user),
          accessToken: data.access_token,
          isAuthenticated: true,
          isAuthLoading: false,
        });
        return true;
      } catch {
        set({
          user: null,
          accessToken: null,
          isAuthenticated: false,
          isAuthLoading: false,
          error: null,
          rateLimitRetryAfter: null,
        });
        const currentPath = window.location.pathname + window.location.search;
        window.location.href = `/login?redirect=${encodeURIComponent(currentPath)}`;
        return false;
      } finally {
        refreshPromise = null;
      }
    })();

    return refreshPromise;
  },

  initialize: async () => {
    set({ isAuthLoading: true });

    try {
      const response = await fetch(`${AUTH_API_BASE}/refresh`, {
        method: "POST",
        credentials: "include",
      });

      if (response.ok) {
        const data = (await response.json()) as LoginSuccessResponse;

        // Guard: reject VENDOR-USER on the company portal.
        // Prevents cross-portal session leakage when both portals
        // share the same refresh token cookie on localhost.
        if (data.user.role === "VENDOR-USER") {
          set({
            user: null,
            accessToken: null,
            isAuthenticated: false,
            isAuthLoading: false,
          });
          return;
        }

        set({
          user: mapUserResponse(data.user),
          accessToken: data.access_token,
          isAuthenticated: true,
          isAuthLoading: false,
        });
      } else {
        set({
          user: null,
          accessToken: null,
          isAuthenticated: false,
          isAuthLoading: false,
        });
      }
    } catch {
      set({
        user: null,
        accessToken: null,
        isAuthenticated: false,
        isAuthLoading: false,
      });
    }
  },

  clearError: () => {
    set({ error: null, rateLimitRetryAfter: null });
  },
}));
