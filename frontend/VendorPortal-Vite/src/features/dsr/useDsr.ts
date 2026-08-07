import { useQuery } from '@tanstack/react-query';
import { useAuth } from '@/features/auth/useAuth';
import dsrData from '@/data/dsr.json';
import type { DsrRecord } from '@/lib/types';

export function useDsr(date?: string) {
  const { state } = useAuth();
  const vendorId = state.user?.vendorId;

  return useQuery({
    queryKey: ['dsr', vendorId, date],
    queryFn: () => {
      const allRecords = dsrData as DsrRecord[];
      let filtered = allRecords.filter((r) => r.vendorId === String(vendorId));

      if (date) {
        filtered = filtered.filter((r) => r.date === date);
      }

      return filtered;
    },
    enabled: !!vendorId,
  });
}
