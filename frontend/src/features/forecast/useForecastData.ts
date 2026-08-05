import { useQuery } from '@tanstack/react-query';

import forecastData from '@/data/forecast.json';
import atmsData from '@/data/atms.json';
import vendorsData from '@/data/vendors.json';

import type { EnrichedForecastRecord, ForecastRecord } from './types';

const atmMap = new Map(atmsData.map((atm) => [atm.id, atm]));
const vendorMap = new Map(vendorsData.map((v) => [v.id, v]));

function enrichRecords(records: ForecastRecord[]): EnrichedForecastRecord[] {
  return records.map((record) => {
    const atm = atmMap.get(record.atmId);
    const vendor = atm ? vendorMap.get(atm.vendorId) : undefined;

    return {
      ...record,
      location: atm?.location ?? '',
      vendorName: vendor?.name ?? '',
    };
  });
}

export function useForecastData() {
  return useQuery({
    queryKey: ['forecast'],
    queryFn: () => {
      const enriched = enrichRecords(forecastData as ForecastRecord[]);
      return Promise.resolve(enriched);
    },
    staleTime: Infinity,
  });
}
