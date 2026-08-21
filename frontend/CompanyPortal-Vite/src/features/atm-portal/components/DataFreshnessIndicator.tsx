/**
 * "Data terakhir: dd MMM yyyy, HH:mm" (Req 112.2), or "Data terakhir: belum
 * tersedia" when no itm_replenish records exist yet (Req 112.4).
 */

import { formatAtmDateTime } from "../lib/formatters";

interface DataFreshnessIndicatorProps {
  /** ISO timestamp from AtmPortalResponse.last_updated, or null. */
  lastUpdated: string | null;
}

export function DataFreshnessIndicator({ lastUpdated }: DataFreshnessIndicatorProps) {
  if (lastUpdated === null) {
    return <p className="text-sm text-[var(--n-600)]">Data terakhir: belum tersedia</p>;
  }

  return (
    <p className="text-sm text-[var(--n-600)]">
      Data terakhir: {formatAtmDateTime(new Date(lastUpdated))}
    </p>
  );
}
