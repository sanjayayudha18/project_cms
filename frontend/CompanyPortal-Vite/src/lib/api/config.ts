// ─── API Client Configuration ─────────────────────────────────────────────────
// Resolves environment variables at build time via Vite's import.meta.env

export type ApiMode = "stub" | "real";

export interface ApiClientConfig {
  mode: ApiMode;
  baseURL: string;
  stubLatency: { min: number; max: number };
}

function resolveMode(): ApiMode {
  const raw = import.meta.env.VITE_API_MODE;
  if (raw === "real") return "real";
  return "stub";
}

function resolveBaseURL(): string {
  const envValue = import.meta.env.VITE_API_BASE_URL;
  return typeof envValue === "string" && envValue.length > 0 ? envValue : "/api/v1";
}

export const apiConfig: ApiClientConfig = {
  mode: resolveMode(),
  baseURL: resolveBaseURL(),
  stubLatency: {
    min: 200,
    max: 800,
  },
};
