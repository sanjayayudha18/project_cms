import { formatWibDateTime } from "../utils";

interface LastUpdatedIndicatorProps {
  updatedAt: number | null;
}

/** Displays the most recent successful data fetch time in WIB. */
export function LastUpdatedIndicator({ updatedAt }: LastUpdatedIndicatorProps) {
  return (
    <p className="flex min-h-[44px] items-center text-xs text-[var(--n-500)]">
      Terakhir diperbarui: {updatedAt ? formatWibDateTime(new Date(updatedAt).toISOString()) : "-"}
    </p>
  );
}
