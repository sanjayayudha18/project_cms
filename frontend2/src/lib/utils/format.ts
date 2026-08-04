/**
 * Formats a non-negative integer as Indonesian Rupiah (IDR).
 *
 * Uses dot-separated thousands per Indonesian locale convention.
 * Examples:
 *   formatIDR(0)       → "Rp0"
 *   formatIDR(1000)    → "Rp1.000"
 *   formatIDR(1500000) → "Rp1.500.000"
 */
export function formatIDR(value: number): string {
  const formatted = value.toLocaleString("id-ID", {
    maximumFractionDigits: 0,
  });
  return `Rp${formatted}`;
}
