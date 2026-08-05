import { Calendar, Truck } from 'lucide-react';

import { Card } from '@/components/ui/Card';
import { formatIDR } from '@/lib/utils/formatCurrency';

import type { ScheduleEntry } from './types';

/**
 * Data jadwal pengisian ulang untuk 3 hari ke depan (H+1, H+2, H+3).
 * Diambil dari data forecast — ATM dengan rekomendasi pengisian > 0.
 */
const scheduleData: ScheduleEntry[] = [
  { id: 'SCH-001', atmId: 'ATM-JKT-001', location: 'Sudirman Plaza', vendorName: 'PT Gardanet', scheduledDate: '2024-01-21', amount: 250000000 },
  { id: 'SCH-002', atmId: 'ATM-JKT-002', location: 'Thamrin City', vendorName: 'PT Gardanet', scheduledDate: '2024-01-21', amount: 350000000 },
  { id: 'SCH-003', atmId: 'ATM-BDG-001', location: 'Dago Plaza', vendorName: 'PT Gardanet', scheduledDate: '2024-01-22', amount: 250000000 },
  { id: 'SCH-004', atmId: 'ATM-JKT-005', location: 'Menteng Square', vendorName: 'PT G4S', scheduledDate: '2024-01-21', amount: 300000000 },
  { id: 'SCH-005', atmId: 'ATM-BDG-003', location: 'Cihampelas Walk', vendorName: 'PT G4S', scheduledDate: '2024-01-22', amount: 300000000 },
  { id: 'SCH-006', atmId: 'ATM-SBY-001', location: 'Tunjungan Plaza', vendorName: 'PT Gardanet', scheduledDate: '2024-01-23', amount: 250000000 },
  { id: 'SCH-007', atmId: 'ATM-SBY-003', location: 'Pakuwon Mall', vendorName: 'PT G4S', scheduledDate: '2024-01-23', amount: 350000000 },
  { id: 'SCH-008', atmId: 'ATM-JKT-004', location: 'Senayan Park', vendorName: 'PT SSI', scheduledDate: '2024-01-21', amount: 200000000 },
  { id: 'SCH-009', atmId: 'ATM-BDG-005', location: 'Buah Batu Square', vendorName: 'PT SSI', scheduledDate: '2024-01-22', amount: 200000000 },
  { id: 'SCH-010', atmId: 'ATM-SBY-002', location: 'Galaxy Mall', vendorName: 'PT SSI', scheduledDate: '2024-01-23', amount: 200000000 },
];

interface GroupedSchedule {
  vendorName: string;
  dates: {
    date: string;
    entries: ScheduleEntry[];
  }[];
}

function groupSchedule(entries: ScheduleEntry[]): GroupedSchedule[] {
  const vendorMap = new Map<string, Map<string, ScheduleEntry[]>>();

  for (const entry of entries) {
    if (!vendorMap.has(entry.vendorName)) {
      vendorMap.set(entry.vendorName, new Map());
    }
    const dateMap = vendorMap.get(entry.vendorName)!;
    if (!dateMap.has(entry.scheduledDate)) {
      dateMap.set(entry.scheduledDate, []);
    }
    dateMap.get(entry.scheduledDate)!.push(entry);
  }

  const result: GroupedSchedule[] = [];
  for (const [vendorName, dateMap] of vendorMap) {
    const dates = Array.from(dateMap.entries())
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([date, entries]) => ({ date, entries }));
    result.push({ vendorName, dates });
  }

  return result.sort((a, b) => a.vendorName.localeCompare(b.vendorName));
}

function formatDate(iso: string): string {
  const date = new Date(iso + 'T00:00:00');
  return date.toLocaleDateString('id-ID', {
    weekday: 'short',
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  });
}

export function ScheduleList() {
  const grouped = groupSchedule(scheduleData);

  if (scheduleData.length === 0) {
    return (
      <Card>
        <h3 className="text-lg font-semibold text-[var(--n-900)] mb-4">
          Jadwal Pengisian (3 Hari ke Depan)
        </h3>
        <p className="text-sm text-[var(--n-500)] text-center py-8">
          Tidak ada jadwal pengisian tersedia.
        </p>
      </Card>
    );
  }

  return (
    <Card>
      <h3 className="text-lg font-semibold text-[var(--n-900)] mb-4">
        Jadwal Pengisian (3 Hari ke Depan)
      </h3>
      <div className="space-y-6">
        {grouped.map((group) => (
          <div key={group.vendorName}>
            <div className="flex items-center gap-2 mb-3">
              <Truck className="h-4 w-4 text-[var(--n-500)]" aria-hidden="true" />
              <h4 className="text-sm font-semibold text-[var(--n-700)]">{group.vendorName}</h4>
            </div>
            <div className="space-y-3 pl-6">
              {group.dates.map((dateGroup) => (
                <div key={dateGroup.date}>
                  <div className="flex items-center gap-1.5 mb-1.5">
                    <Calendar className="h-3.5 w-3.5 text-[var(--n-400)]" aria-hidden="true" />
                    <span className="text-xs font-medium text-[var(--n-600)]">
                      {formatDate(dateGroup.date)}
                    </span>
                  </div>
                  <ul className="space-y-1 pl-5">
                    {dateGroup.entries.map((entry) => (
                      <li
                        key={entry.id}
                        className="flex items-center justify-between text-sm text-[var(--n-800)]"
                      >
                        <span>
                          {entry.atmId} — {entry.location}
                        </span>
                        <span className="tabular-nums font-medium">
                          {formatIDR(entry.amount)}
                        </span>
                      </li>
                    ))}
                  </ul>
                </div>
              ))}
            </div>
          </div>
        ))}
      </div>
    </Card>
  );
}
