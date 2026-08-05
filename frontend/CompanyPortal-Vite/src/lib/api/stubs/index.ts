import { apiConfig } from "../config";
import { handleLogin, handleRefresh } from "./auth";
import { handleDashboardActivity, handleDashboardMetrics } from "./dashboard";
import { handleDSRSubmit, handleDSRUploads } from "./forecasting";

// ─── Latency Simulation ───────────────────────────────────────────────────────

/**
 * Computes a latency delay value from a random input.
 * Exported for testability in property-based tests.
 */
export function computeDelay(random: number, min: number, max: number): number {
  return Math.floor(random * (max - min + 1)) + min;
}

function simulateLatency(): Promise<void> {
  const { min, max } = apiConfig.stubLatency;
  const delay = computeDelay(Math.random(), min, max);
  return new Promise((resolve) => setTimeout(resolve, delay));
}

// ─── Route Matching ───────────────────────────────────────────────────────────

interface StubRoute {
  method: string;
  pattern: RegExp;
  handler: (body: unknown) => Response;
}

const STUB_ROUTES: StubRoute[] = [
  {
    method: "GET",
    pattern: /\/dashboard\/metrics$/,
    handler: () => handleDashboardMetrics(),
  },
  {
    method: "GET",
    pattern: /\/dashboard\/activity/,
    handler: () => handleDashboardActivity(),
  },
  {
    method: "GET",
    pattern: /\/forecasting\/dsr\/uploads$/,
    handler: () => handleDSRUploads(),
  },
  {
    method: "POST",
    pattern: /\/forecasting\/dsr$/,
    handler: (body) => handleDSRSubmit(body),
  },
  {
    method: "POST",
    pattern: /\/auth\/login$/,
    handler: (body) => handleLogin(body),
  },
  {
    method: "POST",
    pattern: /\/auth\/refresh$/,
    handler: () => handleRefresh(),
  },
];

// ─── Public API ───────────────────────────────────────────────────────────────

/**
 * Intercepts a request and returns mock data with simulated latency.
 * Returns `null` if no matching stub route is found (allows fallthrough).
 */
export async function handleStubRequest(
  url: string,
  options?: RequestInit,
): Promise<Response | null> {
  const method = (options?.method ?? "GET").toUpperCase();

  const matched = STUB_ROUTES.find((route) => route.method === method && route.pattern.test(url));

  if (!matched) {
    return null;
  }

  await simulateLatency();

  let body: unknown = undefined;
  if (options?.body) {
    try {
      body = JSON.parse(options.body as string);
    } catch {
      body = options.body;
    }
  }

  return matched.handler(body);
}
