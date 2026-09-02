// ─── VendorPortal API Client ──────────────────────────────────────────────────
// Fetch wrapper with Bearer token injection and 401 single-flight refresh logic.
// Access token is read from a module-level getter (set by AuthContext).

// ─── Types ────────────────────────────────────────────────────────────────────

export interface RequestConfig {
  method: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  path: string;
  body?: unknown;
  headers?: Record<string, string>;
  /** Skip auth header injection (e.g., for login/refresh endpoints) */
  skipAuth?: boolean;
}

export interface ApiError {
  status: number;
  message: string;
  details?: unknown;
}

export interface ApiResponse<T = unknown> {
  data: T;
  status: number;
}

// ─── Token Access ─────────────────────────────────────────────────────────────

let getAccessToken: (() => string | null) | null = null;
let onRefreshToken: (() => Promise<boolean>) | null = null;
let onAuthFailure: (() => void) | null = null;

/**
 * Wire the API client to the auth context.
 * Called once from AuthProvider on mount.
 */
export function configureApiAuth(config: {
  getToken: () => string | null;
  refresh: () => Promise<boolean>;
  onFailure: () => void;
}) {
  getAccessToken = config.getToken;
  onRefreshToken = config.refresh;
  onAuthFailure = config.onFailure;
}

// ─── Request Interceptor ──────────────────────────────────────────────────────

function injectAuthHeader(
  headers: Record<string, string>,
  skipAuth: boolean,
): Record<string, string> {
  if (skipAuth) return headers;

  const token = getAccessToken?.();
  if (token) {
    return { ...headers, Authorization: `Bearer ${token}` };
  }
  return headers;
}

// ─── Single-flight 401 refresh ────────────────────────────────────────────────

let isRefreshing = false;
let refreshPromise: Promise<boolean> | null = null;

async function handleUnauthorized(config: RequestConfig): Promise<Response | null> {
  if (!onRefreshToken) {
    onAuthFailure?.();
    return null;
  }

  // Share the same refresh promise for concurrent 401s
  if (!isRefreshing) {
    isRefreshing = true;
    refreshPromise = onRefreshToken();
  }

  const refreshed = await refreshPromise;
  isRefreshing = false;
  refreshPromise = null;

  if (!refreshed) {
    onAuthFailure?.();
    return null;
  }

  // Retry the original request with the new token
  return executeRequest(config, true);
}

// ─── Core Fetch Execution ─────────────────────────────────────────────────────

async function executeRequest(config: RequestConfig, isRetry = false): Promise<Response> {
  const url = config.path.startsWith("http") ? config.path : config.path;

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...config.headers,
  };

  const finalHeaders = injectAuthHeader(headers, config.skipAuth ?? false);

  const fetchOptions: RequestInit = {
    method: config.method,
    headers: finalHeaders,
    body: config.body ? JSON.stringify(config.body) : undefined,
    credentials: "include",
  };

  const response = await fetch(url, fetchOptions);

  // 401 handling: attempt refresh + retry once
  if (response.status === 401 && !isRetry && !config.skipAuth) {
    const retryResponse = await handleUnauthorized(config);
    if (retryResponse) return retryResponse;

    throw createApiError(response);
  }

  return response;
}

// ─── Error Factory ────────────────────────────────────────────────────────────

function createApiError(response: Response): ApiError {
  return {
    status: response.status,
    message: response.statusText || "Request failed",
  };
}

async function parseApiError(response: Response): Promise<ApiError> {
  try {
    const body = (await response.json()) as { message?: string; details?: unknown };
    return {
      status: response.status,
      message: body.message ?? response.statusText ?? "Request failed",
      details: body.details,
    };
  } catch {
    return createApiError(response);
  }
}

// ─── Public API ───────────────────────────────────────────────────────────────

export async function apiClient<T = unknown>(config: RequestConfig): Promise<ApiResponse<T>> {
  const response = await executeRequest(config);

  if (!response.ok) {
    throw await parseApiError(response);
  }

  // Handle 204 No Content
  if (response.status === 204) {
    return { data: undefined as T, status: 204 };
  }

  const data = (await response.json()) as T;
  return { data, status: response.status };
}

// ─── Convenience Methods ──────────────────────────────────────────────────────

export const api = {
  get<T = unknown>(path: string, options?: Partial<RequestConfig>) {
    return apiClient<T>({ method: "GET", path, ...options });
  },

  post<T = unknown>(path: string, body?: unknown, options?: Partial<RequestConfig>) {
    return apiClient<T>({ method: "POST", path, body, ...options });
  },

  put<T = unknown>(path: string, body?: unknown, options?: Partial<RequestConfig>) {
    return apiClient<T>({ method: "PUT", path, body, ...options });
  },

  patch<T = unknown>(path: string, body?: unknown, options?: Partial<RequestConfig>) {
    return apiClient<T>({ method: "PATCH", path, body, ...options });
  },

  delete<T = unknown>(path: string, options?: Partial<RequestConfig>) {
    return apiClient<T>({ method: "DELETE", path, ...options });
  },
} as const;
