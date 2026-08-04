import { useQuery } from '@tanstack/react-query';

import invoicesData from '@/data/invoices.json';
import vendorsData from '@/data/vendors.json';
import type { Invoice } from './invoice.types';

interface Vendor {
  id: string;
  name: string;
}

export interface InvoiceWithVendor extends Invoice {
  vendorName: string;
}

const vendors = vendorsData as Vendor[];
const vendorMap = new Map(vendors.map((v) => [v.id, v.name]));

export function useInvoiceData() {
  return useQuery({
    queryKey: ['invoices'],
    queryFn: () => {
      const enriched: InvoiceWithVendor[] = (invoicesData as Invoice[]).map(
        (invoice) => ({
          ...invoice,
          vendorName: vendorMap.get(invoice.vendorId) ?? 'Unknown Vendor',
        }),
      );
      return Promise.resolve(enriched);
    },
    staleTime: Infinity,
  });
}
