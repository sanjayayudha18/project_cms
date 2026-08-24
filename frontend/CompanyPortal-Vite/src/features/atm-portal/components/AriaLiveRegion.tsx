/**
 * Visually hidden `aria-live="polite"` region announcing table state
 * transitions for the active ATM Portal dataset.
 */

import type { AtmPortalMode } from "../types";

interface AriaLiveRegionProps {
  isLoading: boolean;
  isError: boolean;
  resultCount: number;
  mode?: AtmPortalMode;
}

function datasetLabel(mode: AtmPortalMode): string {
  return mode === "cashpos" ? "ATM Cashpos" : "ATM";
}

function announcementFor({
  isLoading,
  isError,
  resultCount,
  mode = "replenish",
}: AriaLiveRegionProps): string {
  const label = datasetLabel(mode);
  if (isLoading) {
    return `Memuat data ${label}…`;
  }
  if (isError) {
    return `Gagal memuat data ${label}`;
  }
  if (resultCount === 0) {
    return mode === "cashpos"
      ? "Tidak ada data cashpos yang sesuai filter"
      : "Tidak ada ATM yang sesuai filter";
  }
  return mode === "cashpos"
    ? `Menampilkan ${resultCount} baris ATM Cashpos`
    : `Menampilkan ${resultCount} ATM`;
}

export function AriaLiveRegion(props: AriaLiveRegionProps) {
  return (
    <div aria-live="polite" aria-atomic="true" className="sr-only">
      {announcementFor(props)}
    </div>
  );
}
