import { useVendorBranches, useVendorPackagePrices, useVendorPics } from "../hooks";
import { activePriceRange, priceStatus } from "../lib/packagePrice";

const PLACEHOLDER = "—";

function todayISO(): string {
  return new Date().toISOString().slice(0, 10);
}

function MetricCell({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-1 px-4 py-3">
      <span className="text-xs font-medium uppercase tracking-wide text-[var(--n-500)]">
        {label}
      </span>
      <span className="tabular-nums text-lg font-semibold text-[var(--n-900)]">{value}</span>
    </div>
  );
}

/**
 * Reuses the same list hooks/query keys the tabs already call (page_size 100,
 * status "all"), so TanStack Query dedupes -- no extra network cost (Req 2).
 */
export function SummaryStrip({ vendorId }: { vendorId: number }) {
  const branchesQuery = useVendorBranches(vendorId, { page: 1, page_size: 100, status: "all" });
  const picsQuery = useVendorPics(vendorId, {
    page: 1,
    page_size: 100,
    status: "all",
    vendor_wide_only: true,
  });
  const pricesQuery = useVendorPackagePrices(vendorId, { page: 1, page_size: 100, status: "all" });

  const branches = branchesQuery.data?.branches;
  const pics = picsQuery.data?.pics;
  const prices = pricesQuery.data?.package_prices;

  const branchLabel = branches
    ? `${branches.filter((b) => b.is_active).length}/${branches.length}`
    : PLACEHOLDER;

  const picLabel = pics ? String(pics.length) : PLACEHOLDER;

  const today = todayISO();
  const inEffectLabel = prices
    ? String(prices.filter((p) => priceStatus(p, today) === "Berlaku").length)
    : PLACEHOLDER;

  const range = prices ? activePriceRange(prices, today) : null;
  const rangeLabel = !prices
    ? PLACEHOLDER
    : range
      ? `${range.minLabel} - ${range.maxLabel}`
      : PLACEHOLDER;

  return (
    <div className="grid grid-cols-2 divide-x divide-[var(--n-200)] rounded-lg border border-[var(--n-200)] bg-[var(--n-0)] sm:grid-cols-4">
      <MetricCell label="Cabang aktif" value={branchLabel} />
      <MetricCell label="PIC vendor-wide" value={picLabel} />
      <MetricCell label="Paket berlaku" value={inEffectLabel} />
      <MetricCell label="Rentang harga aktif" value={rangeLabel} />
    </div>
  );
}
