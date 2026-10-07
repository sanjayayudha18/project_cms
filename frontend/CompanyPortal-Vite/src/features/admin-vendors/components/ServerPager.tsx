import { Button } from "@/components/ui/Button";

/** Rows per server page for vendor child lists (backend caps page_size at 100). */
export const CHILD_PAGE_SIZE = 20;

interface ServerPagerProps {
  page: number;
  total: number;
  pageSize?: number;
  unit: string;
  onPageChange: (page: number) => void;
}

/**
 * Prev/next pager for server-paginated lists (same layout as the Vendors
 * list). The server owns paging, so rows past the first page stay reachable
 * -- e.g. ROH's 308 branches.
 */
export function ServerPager({
  page,
  total,
  pageSize = CHILD_PAGE_SIZE,
  unit,
  onPageChange,
}: ServerPagerProps) {
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  return (
    <div className="flex flex-wrap items-center justify-between gap-4 text-sm text-[var(--n-600)]">
      <span>
        Halaman {page} dari {totalPages} ({total} {unit})
      </span>
      <div className="flex items-center gap-2">
        <Button variant="secondary" disabled={page <= 1} onClick={() => onPageChange(page - 1)}>
          Sebelumnya
        </Button>
        <Button
          variant="secondary"
          disabled={page >= totalPages}
          onClick={() => onPageChange(page + 1)}
        >
          Berikutnya
        </Button>
      </div>
    </div>
  );
}
