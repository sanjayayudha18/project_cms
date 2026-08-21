/**
 * Visually hidden `aria-live="polite"` region announcing ATM table state
 * transitions (loading/loaded/error/empty) to screen readers (Req 11.8).
 *
 * The announcement text must actually change between states — a static
 * region that never re-renders its text announces nothing after the first
 * mount, so each state maps to distinct wording (not just visibility).
 */

interface AriaLiveRegionProps {
  isLoading: boolean;
  isError: boolean;
  resultCount: number;
}

function announcementFor({ isLoading, isError, resultCount }: AriaLiveRegionProps): string {
  if (isLoading) {
    return "Memuat data ATM…";
  }
  if (isError) {
    return "Gagal memuat data ATM";
  }
  if (resultCount === 0) {
    return "Tidak ada ATM yang sesuai filter";
  }
  return `Menampilkan ${resultCount} ATM`;
}

export function AriaLiveRegion(props: AriaLiveRegionProps) {
  return (
    <div aria-live="polite" aria-atomic="true" className="sr-only">
      {announcementFor(props)}
    </div>
  );
}
