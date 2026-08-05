import { useQuery } from '@tanstack/react-query';

import citOrdersData from '@/data/cit-orders.json';
import atmsData from '@/data/atms.json';
import vendorsData from '@/data/vendors.json';

import type { CitOrder, EnrichedCitOrder } from './types';

const atmsMap = new Map(atmsData.map((atm) => [atm.id, atm]));
const vendorsMap = new Map(vendorsData.map((v) => [v.id, v]));

function enrichOrders(orders: CitOrder[]): EnrichedCitOrder[] {
  return orders.map((order) => ({
    ...order,
    vendorName: vendorsMap.get(order.vendorId)?.name ?? 'Unknown',
    atmLocation: atmsMap.get(order.atmId)?.location ?? 'Unknown',
  }));
}

export function useCitData() {
  return useQuery({
    queryKey: ['cit-orders'],
    queryFn: () => enrichOrders(citOrdersData as CitOrder[]),
  });
}
