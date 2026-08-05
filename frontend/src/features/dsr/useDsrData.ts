import { useQuery } from '@tanstack/react-query';

import dsrData from '@/data/dsr.json';
import atmsData from '@/data/atms.json';
import vendorsData from '@/data/vendors.json';

import type { DsrRecord, EnrichedDsrRecord } from './types';

const atmsMap = new Map(atmsData.map((atm) => [atm.id, atm]));
const vendorsMap = new Map(vendorsData.map((v) => [v.id, v]));

function enrichRecord(record: DsrRecord): EnrichedDsrRecord {
  const atm = atmsMap.get(record.atmId);
  const vendor = atm ? vendorsMap.get(atm.vendorId) : undefined;

  return {
    ...record,
    location: atm?.location ?? '',
    vendorName: vendor?.name ?? '',
  };
}

export function useDsrData(date: string) {
  return useQuery({
    queryKey: ['dsr', date],
    queryFn: () => {
      const filtered = (dsrData as DsrRecord[]).filter((r) => r.date === date);
      return filtered.map(enrichRecord);
    },
  });
}
