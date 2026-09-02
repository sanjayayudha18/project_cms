import invoicesData from "@/data/invoices.json";
import { useAuth } from "@/features/auth/useAuth";
import type { Invoice } from "@/lib/types";
import { useQuery } from "@tanstack/react-query";

export function useInvoices() {
  const { state } = useAuth();
  const vendorId = state.user?.vendorId;

  return useQuery({
    queryKey: ["invoices", vendorId],
    queryFn: () => {
      const allInvoices = invoicesData as Invoice[];
      return allInvoices.filter((inv) => inv.vendorId === String(vendorId));
    },
    enabled: !!vendorId,
  });
}
