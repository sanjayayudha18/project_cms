import { api } from "@/lib/api/client";
import { useQuery } from "@tanstack/react-query";
import type { ActivityEvent, DashboardMetrics } from "../types";

// ─── Query Keys ───────────────────────────────────────────────────────────────

export const dashboardKeys = {
  all: ["dashboard"] as const,
  metrics: () => [...dashboardKeys.all, "metrics"] as const,
  activity: () => [...dashboardKeys.all, "activity"] as const,
};

// ─── Hooks ────────────────────────────────────────────────────────────────────

export function useDashboardMetrics() {
  return useQuery({
    queryKey: dashboardKeys.metrics(),
    queryFn: async () => {
      const response = await api.get<DashboardMetrics>("/dashboard/metrics");
      return response.data;
    },
  });
}

export function useDashboardActivity() {
  return useQuery({
    queryKey: dashboardKeys.activity(),
    queryFn: async () => {
      const response = await api.get<ActivityEvent[]>("/dashboard/activity?limit=10");
      return response.data;
    },
  });
}
