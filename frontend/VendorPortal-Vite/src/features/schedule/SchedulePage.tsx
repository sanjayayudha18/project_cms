import { Badge } from "@/components/ui/Badge";
import { DatePicker } from "@/components/ui/DatePicker";
import { EmptyState } from "@/components/ui/EmptyState";
import { useSchedule } from "@/features/schedule/useSchedule";
import { formatRp } from "@/lib/formatters";
import type { ReplenishmentSchedule } from "@/lib/types";
import { CalendarClock } from "lucide-react";
import { useMemo, useState } from "react";

type Priority = ReplenishmentSchedule["priority"];
type ScheduleStatus = ReplenishmentSchedule["status"];

const priorityBadgeMap: Record<Priority, "danger" | "warning" | "neutral"> = {
  High: "danger",
  Medium: "warning",
  Low: "neutral",
};

const statusBadgeMap: Record<ScheduleStatus, "warning" | "info" | "success" | "danger"> = {
  Pending: "warning",
  Confirmed: "info",
  Executed: "success",
  Cancelled: "danger",
};

const priorityOrder: Record<Priority, number> = {
  High: 3,
  Medium: 2,
  Low: 1,
};

interface DateGroup {
  readonly date: string;
  readonly formattedDate: string;
  readonly totalAmount: number;
  readonly count: number;
  readonly schedules: ReplenishmentSchedule[];
}

function formatDateHeader(isoDate: string): string {
  const date = new Date(`${isoDate}T00:00:00`);
  return date.toLocaleDateString("en-GB", {
    day: "2-digit",
    month: "short",
    year: "numeric",
  });
}

function groupAndSort(schedules: ReplenishmentSchedule[]): DateGroup[] {
  const groupMap = new Map<string, ReplenishmentSchedule[]>();

  for (const schedule of schedules) {
    const existing = groupMap.get(schedule.scheduledDate);
    if (existing) {
      existing.push(schedule);
    } else {
      groupMap.set(schedule.scheduledDate, [schedule]);
    }
  }

  const sortedDates = [...groupMap.keys()].sort((a, b) => a.localeCompare(b));

  return sortedDates.map((date) => {
    const items = groupMap.get(date)!;

    items.sort((a, b) => priorityOrder[b.priority] - priorityOrder[a.priority]);

    const totalAmount = items.reduce((sum, s) => sum + s.recommendedAmount, 0);

    return {
      date,
      formattedDate: formatDateHeader(date),
      totalAmount,
      count: items.length,
      schedules: items,
    };
  });
}

export function SchedulePage() {
  const { data: schedules = [], isLoading } = useSchedule();

  const [startDate, setStartDate] = useState<string | null>(null);
  const [endDate, setEndDate] = useState<string | null>(null);

  const filteredSchedules = useMemo(() => {
    let result = schedules;

    if (startDate) {
      result = result.filter((s) => s.scheduledDate >= startDate);
    }
    if (endDate) {
      result = result.filter((s) => s.scheduledDate <= endDate);
    }

    return result;
  }, [schedules, startDate, endDate]);

  const groups = useMemo(() => groupAndSort([...filteredSchedules]), [filteredSchedules]);

  if (isLoading) {
    return (
      <div className="flex flex-col gap-6">
        <h1 className="text-xl font-semibold text-surface-text">Replenishment Schedule</h1>
        <p className="text-sm text-neutral-500">Memuat data...</p>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-xl font-semibold text-surface-text">Replenishment Schedule</h1>

      <DatePicker
        startDate={startDate}
        endDate={endDate}
        onStartDateChange={setStartDate}
        onEndDateChange={setEndDate}
      />

      {groups.length === 0 ? (
        <EmptyState
          icon={CalendarClock}
          title="Tidak ada jadwal ditemukan"
          description="Tidak ditemukan jadwal replenishment untuk periode yang dipilih."
        />
      ) : (
        <div className="flex flex-col gap-6">
          {groups.map((group) => (
            <section key={group.date} aria-label={`Jadwal ${group.formattedDate}`}>
              <div className="flex items-baseline justify-between gap-4 border-b border-neutral-200 pb-2 mb-3">
                <h2 className="text-sm font-semibold text-surface-text">{group.formattedDate}</h2>
                <div className="flex items-center gap-4 text-xs text-neutral-500">
                  <span className="tabular-nums font-medium">{formatRp(group.totalAmount)}</span>
                  <span>{group.count} jadwal</span>
                </div>
              </div>

              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="bg-neutral-50 text-neutral-500 uppercase text-xs tracking-wider">
                      <th className="px-4 py-3 text-left font-medium">Schedule ID</th>
                      <th className="px-4 py-3 text-left font-medium">ATM ID</th>
                      <th className="px-4 py-3 text-left font-medium">Location</th>
                      <th className="px-4 py-3 text-right font-medium">Recommended Amount</th>
                      <th className="px-4 py-3 text-left font-medium">Priority</th>
                      <th className="px-4 py-3 text-left font-medium">Status</th>
                    </tr>
                  </thead>
                  <tbody>
                    {group.schedules.map((schedule) => (
                      <tr
                        key={schedule.id}
                        className="border-b border-neutral-100 hover:bg-red-50/30"
                      >
                        <td className="px-4 py-3">{schedule.id}</td>
                        <td className="px-4 py-3">{schedule.atmId}</td>
                        <td className="px-4 py-3">{schedule.location}</td>
                        <td className="px-4 py-3 text-right tabular-nums">
                          {formatRp(schedule.recommendedAmount)}
                        </td>
                        <td className="px-4 py-3">
                          <Badge variant={priorityBadgeMap[schedule.priority]}>
                            {schedule.priority}
                          </Badge>
                        </td>
                        <td className="px-4 py-3">
                          <Badge variant={statusBadgeMap[schedule.status]}>{schedule.status}</Badge>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </section>
          ))}
        </div>
      )}
    </div>
  );
}
