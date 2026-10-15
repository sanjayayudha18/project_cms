import { configureApiAuth } from "@/lib/api/client";
import { queryClient } from "@/lib/queryClient";
import type { AuthState, AuthUser } from "@/lib/types";
import {
  type ReactNode,
  createContext,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

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
  login(email: string, password: string): Promise<void>;
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

// Same for the mount-time refresh: React StrictMode runs the init effect twice in dev, and
// refresh tokens rotate, so a second concurrent /refresh presents the already-used cookie,
// gets a 401 and clears the session the first call just restored.
let initPromise: Promise<LoginSuccessResponse | null> | null = null;

async function fetchInitialSession(): Promise<LoginSuccessResponse | null> {
  try {
    const response = await fetch(`${AUTH_API_BASE}/refresh`, {
      method: "POST",
      credentials: "include",
    });
    return response.ok ? ((await response.json()) as LoginSuccessResponse) : null;
  } catch {
    // No refresh token available — user is not authenticated
    return null;
  }
}

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

  // The api client's getter (wired once, see the configureApiAuth effect) reads
  // this ref. applyToken sets it in the same call as the state, so a request made
  // right after login/logout sees the new token instead of waiting for an effect.
  const accessTokenRef = useRef<string | null>(null);
  const applyToken = useCallback((token: string | null) => {
    accessTokenRef.current = token;
    setAccessToken(token);
  }, []);

  // ─── Initialize: attempt token refresh on mount ─────────────────────────────

  useEffect(() => {
    let cancelled = false;

    async function initialize() {
      initPromise ??= fetchInitialSession().finally(() => {
        initPromise = null;
      });
      const data = await initPromise;

      if (cancelled) return;

      // Guard: only accept VENDOR-USER role on the vendor portal.
      // If a company/internal user's refresh token is present (shared
      // cookie on localhost), reject it so they can't access vendor portal.
      if (data && data.user.role === "VENDOR-USER") {
        applyToken(data.access_token);
        setUser(mapUserResponse(data.user));
      }
      setIsAuthLoading(false);
    }

    void initialize();

    return () => {
      cancelled = true;
    };
  }, [applyToken]);

  // ─── Login ──────────────────────────────────────────────────────────────────

  const login = useCallback(
    async (email: string, password: string) => {
      setError(null);
      setRateLimitRetryAfter(null);

      try {
        const response = await fetch(`${AUTH_API_BASE}/login`, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            "X-Portal-Type": "vendor",
          },
          body: JSON.stringify({ email, password }),
          credentials: "include",
        });

        if (response.ok) {
          const data = (await response.json()) as LoginSuccessResponse;
          applyToken(data.access_token);
          setUser(mapUserResponse(data.user));
          setError(null);
          setRateLimitRetryAfter(null);
          return;
        }

        // Handle error responses
        switch (response.status) {
          case 401: {
            setError("Email atau password salah");
            throw new Error("Email atau password salah");
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
    },
    [applyToken],
  );

  // ─── Logout ─────────────────────────────────────────────────────────────────

  const logout = useCallback(async () => {
    applyToken(null);
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
  }, [applyToken]);

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
          applyToken(null);
          setUser(null);
          setError("Sesi berakhir, silakan login kembali");
          return false;
        }

        const data = (await response.json()) as LoginSuccessResponse;

        // Guard: reject non-vendor users on token refresh
        if (data.user.role !== "VENDOR-USER") {
          applyToken(null);
          setUser(null);
          return false;
        }

        applyToken(data.access_token);
        setUser(mapUserResponse(data.user));
        return true;
      } catch {
        applyToken(null);
        setUser(null);
        return false;
      } finally {
        refreshPromise = null;
      }
    })();

    return refreshPromise;
  }, [applyToken]);

  // ─── Wire the api client's auth hooks ───────────────────────────────────────
  // Without this, api.* calls (e.g. dsrUploadApi) never get an Authorization
  // header and always 401, regardless of login state.

  useEffect(() => {
    configureApiAuth({
      getToken: () => accessTokenRef.current,
      refresh: refreshToken,
      onFailure: () => {
        applyToken(null);
        setUser(null);
      },
    });
  }, [refreshToken, applyToken]);

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
