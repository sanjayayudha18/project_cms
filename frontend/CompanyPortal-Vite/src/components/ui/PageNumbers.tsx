/**
 * Page numbers to render (1-based), with null for an ellipsis gap: always the
 * first and last page plus current ±1, e.g. 12 pages at page 5 -> 1 … 4 5 6 … 12.
 */
export function pageNumbers(current: number, total: number): (number | null)[] {
  const keep = new Set([1, total, current - 1, current, current + 1]);
  const out: (number | null)[] = [];
  for (let n = 1; n <= total; n++) {
    if (!keep.has(n)) continue;
    const prev = out[out.length - 1];
    if (typeof prev === "number" && n - prev > 1) out.push(n - prev === 2 ? n - 1 : null);
    out.push(n);
  }
  return out;
}

interface PageNumbersProps {
  /** 1-based current page. */
  current: number;
  total: number;
  onChange: (page: number) => void;
}

/** Numbered page buttons; place between the Sebelumnya/Berikutnya buttons. */
export function PageNumbers({ current, total, onChange }: PageNumbersProps) {
  return (
    <>
      {pageNumbers(current, total).map((n, i) =>
        n === null ? (
          // biome-ignore lint/suspicious/noArrayIndexKey: ellipsis slots have no identity
          <span key={`gap-${i}`} aria-hidden="true" className="px-1">
            …
          </span>
        ) : (
          <button
            key={n}
            type="button"
            aria-label={`Halaman ${n} dari ${total}`}
            aria-current={n === current ? "page" : undefined}
            onClick={() => onChange(n)}
            className={[
              "inline-flex min-h-[44px] min-w-[44px] items-center justify-center rounded-[var(--radius-md)] border tabular-nums",
              "outline-none focus-visible:ring-2 focus-visible:ring-[var(--red-100)]",
              n === current
                ? "border-[var(--red-600)] bg-[var(--red-600)] font-semibold text-[var(--n-0)]"
                : "border-[var(--n-300)] bg-[var(--n-0)] text-[var(--n-800)] hover:bg-[var(--red-50)]",
            ].join(" ")}
          >
            {n}
          </button>
        ),
      )}
    </>
  );
}
