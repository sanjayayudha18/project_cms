/**
 * Current page / total pages display, Previous/Next navigation, and a page
 * size selector (Req 4.8). Nav buttons carry an aria-label naming the
 * target page and total pages (Req 11.4, e.g. "Halaman 1 dari 10") — not
 * just a generic "Previous"/"Next" — so a screen reader announces where
 * the button goes, not just that it moves.
 */

import { ChevronLeft, ChevronRight } from "lucide-react";

const PAGE_SIZE_OPTIONS = [10, 25, 50, 100] as const;

// Button (@/components/ui/Button) doesn't accept/forward aria-label, which
// these nav controls need (Req 11.4) — plain <button> styled to match
// Button's "secondary" variant instead.
const NAV_BUTTON_CLASS =
  "inline-flex min-h-[44px] min-w-[44px] items-center justify-center rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)] disabled:cursor-not-allowed disabled:bg-[var(--n-200)] disabled:text-[var(--n-400)]";

interface PaginationControlsProps {
  page: number;
  pageSize: number;
  total: number;
  onPageChange: (page: number) => void;
  onPageSizeChange: (pageSize: number) => void;
}

export function PaginationControls({
  page,
  pageSize,
  total,
  onPageChange,
  onPageSizeChange,
}: PaginationControlsProps) {
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const canGoPrevious = page > 1;
  const canGoNext = page < totalPages;

  return (
    <div className="flex flex-wrap items-center justify-between gap-4">
      <div className="flex items-center gap-2">
        <button
          type="button"
          disabled={!canGoPrevious}
          onClick={() => onPageChange(page - 1)}
          aria-label={
            canGoPrevious
              ? `Halaman ${page - 1} dari ${totalPages}`
              : "Tidak ada halaman sebelumnya"
          }
          className={NAV_BUTTON_CLASS}
        >
          <ChevronLeft className="h-4 w-4" aria-hidden="true" />
        </button>

        <span className="text-sm text-[var(--n-700)]">
          Halaman {page} dari {totalPages}
        </span>

        <button
          type="button"
          disabled={!canGoNext}
          onClick={() => onPageChange(page + 1)}
          aria-label={
            canGoNext ? `Halaman ${page + 1} dari ${totalPages}` : "Tidak ada halaman berikutnya"
          }
          className={NAV_BUTTON_CLASS}
        >
          <ChevronRight className="h-4 w-4" aria-hidden="true" />
        </button>
      </div>

      <div className="flex items-center gap-2">
        <label htmlFor="atm-portal-page-size" className="text-sm text-[var(--n-600)]">
          Baris per halaman
        </label>
        <select
          id="atm-portal-page-size"
          value={pageSize}
          onChange={(e) => onPageSizeChange(Number(e.target.value))}
          className="min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
        >
          {PAGE_SIZE_OPTIONS.map((option) => (
            <option key={option} value={option}>
              {option}
            </option>
          ))}
        </select>
      </div>
    </div>
  );
}
