import { progressPercent } from "@/lib/utils/formatters";

export interface ProgressBarProps {
  completed: number;
  total: number;
  status: "in-transit" | "completed" | "delayed";
}

const fillColorByStatus: Record<ProgressBarProps["status"], string> = {
  "in-transit": "bg-[var(--red-500)]",
  completed: "bg-[var(--success-solid)]",
  delayed: "bg-[var(--warning-solid)]",
};

/**
 * Horizontal progress bar with colored fill based on status.
 * Displays completion as "X dari Y" text label beside the bar.
 */
export function ProgressBar({ completed, total, status }: ProgressBarProps) {
  const percent = progressPercent(completed, total);

  return (
    <div className="flex items-center gap-2">
      <div className="flex-1 h-1.5 rounded-[var(--radius-sm)] bg-[var(--n-200)]">
        <div
          className={`h-full rounded-[var(--radius-sm)] ${fillColorByStatus[status]}`}
          style={{ width: `${percent}%` }}
        />
      </div>
      <span className="text-xs text-[var(--n-600)] shrink-0 tabular-nums">
        {completed} dari {total}
      </span>
    </div>
  );
}
