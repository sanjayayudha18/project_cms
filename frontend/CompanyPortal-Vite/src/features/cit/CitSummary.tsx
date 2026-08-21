import { Calendar, CheckCircle, Truck, XCircle } from "lucide-react";

import type { CitStatus, EnrichedCitOrder } from "./types";

const statusMeta: Record<CitStatus, { icon: typeof Calendar; colorClass: string }> = {
  Scheduled: { icon: Calendar, colorClass: "text-[var(--info-fg)]" },
  "In Transit": { icon: Truck, colorClass: "text-[var(--warning-fg)]" },
  Completed: { icon: CheckCircle, colorClass: "text-[var(--success-fg)]" },
  Failed: { icon: XCircle, colorClass: "text-[var(--danger-fg)]" },
};

const statuses: CitStatus[] = ["Scheduled", "In Transit", "Completed", "Failed"];

interface CitSummaryProps {
  data: EnrichedCitOrder[];
}

export function CitSummary({ data }: CitSummaryProps) {
  const counts = statuses.reduce(
    (acc, status) => {
      acc[status] = data.filter((order) => order.status === status).length;
      return acc;
    },
    {} as Record<CitStatus, number>,
  );

  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
      {statuses.map((status) => {
        const { icon: Icon, colorClass } = statusMeta[status];
        return (
          <div
            key={status}
            className="flex items-center gap-3 rounded-[var(--radius-lg)] bg-[var(--n-0)] p-4 shadow-sm"
          >
            <Icon className={`h-5 w-5 shrink-0 ${colorClass}`} aria-hidden="true" />
            <div>
              <p className="text-xs font-medium uppercase tracking-wider text-[var(--n-500)]">
                {status}
              </p>
              <p className="text-lg font-semibold text-[var(--n-900)] tabular-nums">
                {counts[status]}
              </p>
            </div>
          </div>
        );
      })}
    </div>
  );
}
