import { useQuery } from '@tanstack/react-query';
import { useAuth } from '@/features/auth/useAuth';
import invoicesData from '@/data/invoices.json';
import type { Invoice } from '@/lib/types';

export function useInvoices() {
  const { state } = useAuth();
  const vendorId = state.user?.vendorId;

  return useQuery({
    queryKey: ['invoices', vendorId],
    queryFn: () => {
      const allInvoices = invoicesData as Invoice[];
      return allInvoices.filter((inv) => inv.vendorId === String(vendorId));
    },
    enabled: !!vendorId,
  });
}
