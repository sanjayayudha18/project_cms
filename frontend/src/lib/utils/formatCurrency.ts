/**
 * Currency formatting utilities for Indonesian Rupiah (IDR).
 * Uses dot-separated thousands (Indonesian locale), no decimals.
 *
 * @module formatCurrency
 */

const idrFormatter = new Intl.NumberFormat('id-ID', {
  style: 'decimal',
  minimumFractionDigits: 0,
  maximumFractionDigits: 0,
});

/**
 * Format a number as Indonesian Rupiah display value.
 * Output uses dot-separated thousands grouping with no decimal places.
 *
 * @example formatIDR(1250000) // "1.250.000"
 * @example formatIDR(0) // "0"
 */
export function formatIDR(amount: number): string {
  return idrFormatter.format(amount);
}

/**
 * Parse an IDR-formatted string back to a number.
 * Strips all dot separators and converts to integer.
 * Round-trip guarantee: parseIDR(formatIDR(n)) === n for any non-negative integer n.
 *
 * @example parseIDR("1.250.000") // 1250000
 * @example parseIDR("0") // 0
 */
export function parseIDR(formatted: string): number {
  return Number(formatted.replace(/\./g, ''));
}
