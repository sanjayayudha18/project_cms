import { useQuery } from '@tanstack/react-query';
import { useAuth } from '@/features/auth/useAuth';
import ordersData from '@/data/orders.json';
import type { CITOrder } from '@/lib/types';

export function useOrders() {
  const { state } = useAuth();
  const vendorId = state.user?.vendorId;

  return useQuery({
    queryKey: ['orders', vendorId],
    queryFn: () => {
      const allOrders = ordersData as CITOrder[];
      return allOrders.filter((o) => o.vendorId === vendorId);
    },
    enabled: !!vendorId,
  });
}
