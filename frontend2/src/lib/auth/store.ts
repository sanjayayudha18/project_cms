import { create } from "zustand";

// ─── Types ────────────────────────────────────────────────────────────────────

export type Role =
  | "Admin"
  | "ATM_Support"
  | "Cash_Management"
  | "Vendor"
  | "WMO"
  | "Finance"
  | "Cash_Count_PIC"
  | "Cash_Count_Lead"
  | "Branch"
  | "Approver";

export interface AuthUser {
  id: string;
  fullName: string;
  email: string;
  roles: Role[];
  primaryRole: Role;
}

export interface AuthState {
  user: AuthUser | null;
  accessToken: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;
}

export interface LoginCredentials {
  email: string;
  password: string;
}

export type LoginResult = { success: true; user: AuthUser } | { success: false; error: string };

export interface AuthActions {
  login: (credentials: LoginCredentials) => Promise<LoginResult>;
  logout: () => void;
  refreshToken: () => Promise<boolean>;
  initialize: () => Promise<void>;
}

// ─── Role-Permission Mapping ──────────────────────────────────────────────────

export const ROLE_NAV_PERMISSIONS: Record<Role, string[]> = {
  Admin: ["*"],
  ATM_Support: ["dashboard", "forecasting/*"],
  Cash_Management: ["dashboard", "forecasting/*"],
  Vendor: ["dashboard", "dsr-upload", "fill-instruction-download", "invoice-upload"],
  WMO: ["dashboard", "invoice/*"],
  Finance: ["dashboard", "invoice/*"],
  Cash_Count_PIC: ["dashboard", "cash-count/*"],
  Cash_Count_Lead: ["dashboard", "cash-count/*"],
  Branch: ["dashboard", "forecasting/h2-projection"],
  Approver: ["dashboard", "forecasting/*", "invoice/*", "cash-count/*"],
};

// ─── API Endpoints (placeholder — will be wired to real API client later) ─────

const AUTH_API_BASE = "/api/v1/auth";

async function postLogin(
  credentials: LoginCredentials,
): Promise<{ user: AuthUser; accessToken: string } | { error: string }> {
  try {
    const { apiConfig } = await import("@/lib/api/config");

    if (apiConfig.mode === "stub") {
      const { handleStubRequest } = await import("@/lib/api/stubs/index");
      const stubResponse = await handleStubRequest(`${apiConfig.baseURL}/auth/login`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(credentials),
      });

      if (stubResponse && stubResponse.ok) {
        return (await stubResponse.json()) as { user: AuthUser; accessToken: string };
      }
      return { error: "Kredensial tidak valid" };
    }

    const response = await fetch(`${AUTH_API_BASE}/login`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(credentials),
      credentials: "include",
    });

    if (!response.ok) {
      const body = (await response.json().catch(() => null)) as {
        message?: string;
      } | null;
      return { error: body?.message ?? "Kredensial tidak valid" };
    }

    return (await response.json()) as { user: AuthUser; accessToken: string };
  } catch {
    return { error: "Gagal terhubung ke server. Silakan coba lagi." };
  }
}

async function postRefreshToken(): Promise<{
  accessToken: string;
  user: AuthUser;
} | null> {
  try {
    const { apiConfig } = await import("@/lib/api/config");

    // In stub mode, refresh only works if already authenticated (simulates no existing session on first boot)
    if (apiConfig.mode === "stub") {
      return null;
    }

    const response = await fetch(`${AUTH_API_BASE}/refresh`, {
      method: "POST",
      credentials: "include",
    });

    if (!response.ok) return null;

    return (await response.json()) as { accessToken: string; user: AuthUser };
  } catch {
    return null;
  }
}

// ─── Store ────────────────────────────────────────────────────────────────────

export type AuthStore = AuthState & AuthActions;

export const useAuthStore = create<AuthStore>((set, get) => ({
  user: null,
  accessToken: null,
  isAuthenticated: false,
  isLoading: true,

  login: async (credentials) => {
    const result = await postLogin(credentials);

    if ("error" in result) {
      return { success: false, error: result.error };
    }

    set({
      user: result.user,
      accessToken: result.accessToken,
      isAuthenticated: true,
      isLoading: false,
    });

    return { success: true, user: result.user };
  },

  logout: () => {
    set({
      user: null,
      accessToken: null,
      isAuthenticated: false,
      isLoading: false,
    });

    fetch(`${AUTH_API_BASE}/logout`, {
      method: "POST",
      credentials: "include",
    }).catch(() => {
      // Intentionally ignore — local state is already cleared
    });

    window.location.href = "/login";
  },

  refreshToken: async () => {
    const result = await postRefreshToken();

    if (!result) {
      const { logout } = get();
      logout();
      return false;
    }

    set({
      user: result.user,
      accessToken: result.accessToken,
      isAuthenticated: true,
      isLoading: false,
    });

    return true;
  },

  initialize: async () => {
    set({ isLoading: true });

    try {
      const result = await postRefreshToken();

      if (result) {
        set({
          user: result.user,
          accessToken: result.accessToken,
          isAuthenticated: true,
          isLoading: false,
        });
      } else {
        // No valid session — just set unauthenticated (don't call logout which hard-redirects)
        set({
          user: null,
          accessToken: null,
          isAuthenticated: false,
          isLoading: false,
        });
      }
    } catch {
      // Network error or backend unavailable — treat as unauthenticated
      set({
        user: null,
        accessToken: null,
        isAuthenticated: false,
        isLoading: false,
      });
    }
  },
}));
