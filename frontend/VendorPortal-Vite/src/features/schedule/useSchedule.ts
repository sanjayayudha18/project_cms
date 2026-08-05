import { useQuery } from '@tanstack/react-query';
import { useAuth } from '@/features/auth/useAuth';
import schedulesData from '@/data/schedules.json';
import type { ReplenishmentSchedule } from '@/lib/types';

export function useSchedule() {
  const { state } = useAuth();
  const vendorId = state.user?.vendorId;

  return useQuery({
    queryKey: ['schedules', vendorId],
    queryFn: () => {
      const allSchedules = schedulesData as ReplenishmentSchedule[];
      return allSchedules.filter((s) => s.vendorId === vendorId);
    },
    enabled: !!vendorId,
  });
}
