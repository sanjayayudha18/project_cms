import schedulesData from "@/data/schedules.json";
import { useAuth } from "@/features/auth/useAuth";
import type { ReplenishmentSchedule } from "@/lib/types";
import { useQuery } from "@tanstack/react-query";

export function useSchedule() {
  const { state } = useAuth();
  const vendorId = state.user?.vendorId;

  return useQuery({
    queryKey: ["schedules", vendorId],
    queryFn: () => {
      const allSchedules = schedulesData as ReplenishmentSchedule[];
      return allSchedules.filter((s) => s.vendorId === String(vendorId));
    },
    enabled: !!vendorId,
  });
}
