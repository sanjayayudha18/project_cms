import ordersData from "@/data/orders.json";
import { useAuth } from "@/features/auth/useAuth";
import type { CITOrder } from "@/lib/types";
import { useQuery } from "@tanstack/react-query";

export function useOrders() {
  const { state } = useAuth();
  const vendorId = state.user?.vendorId;

  return useQuery({
    queryKey: ["orders", vendorId],
    queryFn: () => {
      const allOrders = ordersData as CITOrder[];
      return allOrders.filter((o) => o.vendorId === String(vendorId));
    },
    enabled: !!vendorId,
  });
}
