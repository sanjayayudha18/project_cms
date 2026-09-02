import { queryClient } from "@/lib/queryClient";
import type { AuthState, AuthUser } from "@/lib/types";
import { type ReactNode, createContext, useCallback, useEffect, useMemo, useState } from "react";

// ─── API Config ───────────────────────────────────────────────────────────────

const AUTH_API_BASE = "/api/v1/auth";

// ─── Backend Response Types ───────────────────────────────────────────────────

interface LoginSuccessResponse {
  access_token: string;
  user: {
    id: number;
    username: string;
    full_name: string;
    email: string;
    role: string; // Intentionally broad — backend may return any role during refresh
    is_karyawan: boolean;
    vendor_id: number | null;
  };
}

interface ApiErrorResponse {
  error: string;
  message: string;
  details?: Array<{ field: string; message: string }>;
}

// ─── Context Types ────────────────────────────────────────────────────────────

export interface AuthContextValue {
  readonly state: AuthState;
  login(username: string, password: string): Promise<void>;
  logout(): Promise<void>;
  refreshToken(): Promise<boolean>;
}

export const AuthContext = createContext<AuthContextValue | null>(null);

// ─── Helpers ──────────────────────────────────────────────────────────────────

function mapUserResponse(raw: LoginSuccessResponse["user"]): AuthUser {
  return {
    id: raw.id,
    username: raw.username,
    fullName: raw.full_name,
    email: raw.email,
    role: raw.role as AuthUser["role"],
    isKaryawan: raw.is_karyawan,
    vendorId: raw.vendor_id,
  };
}

// ─── Single-flight refresh ────────────────────────────────────────────────────

let refreshPromise: Promise<boolean> | null = null;

// ─── Provider ─────────────────────────────────────────────────────────────────

interface AuthProviderProps {
  readonly children: ReactNode;
}

export function AuthProvider({ children }: AuthProviderProps) {
  const [accessToken, setAccessToken] = useState<string | null>(null);
  const [user, setUser] = useState<AuthUser | null>(null);
  const [isAuthLoading, setIsAuthLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [rateLimitRetryAfter, setRateLimitRetryAfter] = useState<number | null>(null);

  // ─── Initialize: attempt token refresh on mount ─────────────────────────────

  useEffect(() => {
    let cancelled = false;

    async function initialize() {
      try {
        const response = await fetch(`${AUTH_API_BASE}/refresh`, {
          method: "POST",
          credentials: "include",
        });

        if (cancelled) return;

        if (response.ok) {
          const data = (await response.json()) as LoginSuccessResponse;

          // Guard: only accept VENDOR-USER role on the vendor portal.
          // If a company/internal user's refresh token is present (shared
          // cookie on localhost), reject it so they can't access vendor portal.
          if (data.user.role !== "VENDOR-USER") {
            return;
          }

          setAccessToken(data.access_token);
          setUser(mapUserResponse(data.user));
        }
      } catch {
        // No refresh token available — user is not authenticated
      } finally {
        if (!cancelled) {
          setIsAuthLoading(false);
        }
      }
    }

    void initialize();

    return () => {
      cancelled = true;
    };
  }, []);

  // ─── Login ──────────────────────────────────────────────────────────────────

  const login = useCallback(async (username: string, password: string) => {
    setError(null);
    setRateLimitRetryAfter(null);

    try {
      const response = await fetch(`${AUTH_API_BASE}/login`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Portal-Type": "vendor",
        },
        body: JSON.stringify({ username, password }),
        credentials: "include",
      });

      if (response.ok) {
        const data = (await response.json()) as LoginSuccessResponse;
        setAccessToken(data.access_token);
        setUser(mapUserResponse(data.user));
        setError(null);
        setRateLimitRetryAfter(null);
        return;
      }

      // Handle error responses
      switch (response.status) {
        case 401: {
          setError("Username atau password salah");
          throw new Error("Username atau password salah");
        }
        case 403: {
          const body = (await response.json().catch(() => null)) as ApiErrorResponse | null;
          if (body?.error === "account_inactive") {
            setError("Akun tidak aktif");
            throw new Error("Akun tidak aktif");
          }
          if (body?.error === "portal_mismatch") {
            setError("Akun tidak memiliki akses ke portal ini");
            throw new Error("Akun tidak memiliki akses ke portal ini");
          }
          const msg = body?.message ?? "Akses ditolak";
          setError(msg);
          throw new Error(msg);
        }
        case 422: {
          const body = (await response.json().catch(() => null)) as ApiErrorResponse | null;
          const msg = body?.message ?? "Validasi gagal";
          setError(msg);
          throw new Error(msg);
        }
        case 429: {
          const retryAfter = response.headers.get("Retry-After");
          const seconds = retryAfter ? Number.parseInt(retryAfter, 10) : null;
          const retryValue = Number.isFinite(seconds) ? seconds : null;
          setError("Terlalu banyak percobaan login");
          setRateLimitRetryAfter(retryValue);
          throw new Error("Terlalu banyak percobaan login");
        }
        case 503: {
          setError("Layanan sedang tidak tersedia");
          throw new Error("Layanan sedang tidak tersedia");
        }
        default: {
          const body = (await response.json().catch(() => null)) as ApiErrorResponse | null;
          const msg = body?.message ?? "Terjadi kesalahan. Silakan coba lagi.";
          setError(msg);
          throw new Error(msg);
        }
      }
    } catch (err) {
      if (err instanceof Error && err.message !== "Failed to fetch") {
        throw err;
      }
      // Network error
      const msg = "Gagal terhubung ke server. Silakan coba lagi.";
      setError(msg);
      throw new Error(msg);
    }
  }, []);

  // ─── Logout ─────────────────────────────────────────────────────────────────

  const logout = useCallback(async () => {
    setAccessToken(null);
    setUser(null);
    setError(null);
    setRateLimitRetryAfter(null);
    queryClient.clear();

    try {
      await fetch(`${AUTH_API_BASE}/logout`, {
        method: "POST",
        credentials: "include",
      });
    } catch {
      // Intentionally ignore — local state is already cleared
    }
  }, []);

  // ─── Refresh Token ──────────────────────────────────────────────────────────

  const refreshToken = useCallback(async (): Promise<boolean> => {
    // Single-flight: reuse existing refresh if in progress
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
          setAccessToken(null);
          setUser(null);
          return false;
        }

        const data = (await response.json()) as LoginSuccessResponse;

        // Guard: reject non-vendor users on token refresh
        if (data.user.role !== "VENDOR-USER") {
          setAccessToken(null);
          setUser(null);
          return false;
        }

        setAccessToken(data.access_token);
        setUser(mapUserResponse(data.user));
        return true;
      } catch {
        setAccessToken(null);
        setUser(null);
        return false;
      } finally {
        refreshPromise = null;
      }
    })();

    return refreshPromise;
  }, []);

  // ─── State ──────────────────────────────────────────────────────────────────

  const state: AuthState = useMemo(
    () => ({
      user,
      isAuthenticated: accessToken !== null && user !== null,
      isAuthLoading,
      error,
      rateLimitRetryAfter,
    }),
    [accessToken, user, isAuthLoading, error, rateLimitRetryAfter],
  );

  const value: AuthContextValue = useMemo(
    () => ({ state, login, logout, refreshToken }),
    [state, login, logout, refreshToken],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
