import type { StatsCardData } from "./types";
import { StatsCard } from "./StatsCard";

interface StatsCardGridProps {
  readonly stats: readonly StatsCardData[];
}

export function StatsCardGrid({ stats }: StatsCardGridProps) {
  return (
    <div className="grid grid-cols-1 min-[480px]:grid-cols-2 min-[768px]:grid-cols-4 gap-4">
      {stats.map((stat) => (
        <div key={stat.label} className="rounded-lg bg-[var(--n-0)] shadow-sm">
          <StatsCard data={stat} />
        </div>
      ))}
    </div>
  );
}
