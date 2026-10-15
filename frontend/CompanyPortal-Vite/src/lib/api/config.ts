// ─── API Client Configuration ─────────────────────────────────────────────────
// Resolves environment variables at build time via Vite's import.meta.env

export interface ApiClientConfig {
  baseURL: string;
}

function resolveBaseURL(): string {
  const envValue = import.meta.env.VITE_API_BASE_URL;
  return typeof envValue === "string" && envValue.length > 0 ? envValue : "/api/v1";
}

export const apiConfig: ApiClientConfig = {
  baseURL: resolveBaseURL(),
};
