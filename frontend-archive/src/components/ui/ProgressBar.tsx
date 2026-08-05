import { progressPercent } from '@/lib/formatters';

interface ProgressBarProps {
  completed: number;
  total: number;
  status: 'in-transit' | 'completed' | 'delayed';
}

const fillColorByStatus: Record<ProgressBarProps['status'], string> = {
  'in-transit': 'bg-[var(--red-500)]',
  completed: 'bg-[var(--success-solid)]',
  delayed: 'bg-[var(--warning-solid)]',
};

/**
 * Horizontal progress bar with colored fill based on status.
 * Displays completion as "X of Y" text label beside the bar.
 *
 * @validates Requirements 4.3, 4.4
 */
export function ProgressBar({ completed, total, status }: ProgressBarProps) {
  const percent = progressPercent(completed, total);

  return (
    <div className="flex items-center gap-2">
      <div className="flex-1 h-1.5 rounded-[var(--radius-sm)] bg-n-200">
        <div
          className={`h-full rounded-[var(--radius-sm)] ${fillColorByStatus[status]}`}
          style={{ width: `${percent}%` }}
        />
      </div>
      <span className="text-sm text-n-600 shrink-0">
        {completed} of {total}
      </span>
    </div>
  );
}

export type { ProgressBarProps };
