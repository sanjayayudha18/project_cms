import type { StatsCardData, TrendDirection } from './types';

interface StatsCardProps {
  readonly data: StatsCardData;
}

export function StatsCard({ data }: StatsCardProps) {
  const { label, icon: Icon, value, subtitle, trend } = data;

  return (
    <div className="p-5">
      <div className="flex items-center gap-1.5 text-xs text-[var(--n-500)]">
        <Icon size={14} aria-hidden="true" />
        <span>{label}</span>
      </div>
      <div className="mt-1 text-2xl font-bold tabular-nums text-[var(--n-900)]">
        {value}
      </div>
      <div className="mt-1 flex items-center gap-2 text-xs">
        {subtitle && <span className="text-[var(--n-500)]">{subtitle}</span>}
        {trend && (
          <TrendIndicator
            direction={trend.direction}
            percentage={trend.percentage}
          />
        )}
      </div>
    </div>
  );
}

export function TrendIndicator({
  direction,
  percentage,
}: {
  direction: TrendDirection;
  percentage: number;
}) {
  const isUp = direction === 'up';
  const colorClass = isUp ? 'text-success-fg' : 'text-danger-fg';
  const arrow = isUp ? '↑' : '↓';
  const srLabel = isUp ? 'Increased' : 'Decreased';

  return (
    <span className={`inline-flex items-center gap-0.5 ${colorClass}`}>
      <span aria-hidden="true">{arrow}</span>
      <span className="sr-only">{srLabel}</span>
      <span>{percentage}%</span>
    </span>
  );
}
