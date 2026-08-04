import type { BalanceStatus } from '@/lib/types';

/**
 * Format an amount as IDR with dot-separated thousands (Indonesian locale).
 * Example: formatIDR(250000000) → "IDR 250.000.000"
 */
export function formatIDR(amount: number): string {
  return `IDR ${amount.toLocaleString('id-ID')}`;
}

/**
 * Format an amount with "Rp" prefix and dot-separated thousands (Indonesian locale).
 * Used for replenishment schedule amounts.
 * Example: formatRp(150000000) → "Rp 150.000.000"
 */
export function formatRp(amount: number): string {
  return `Rp ${amount.toLocaleString('id-ID')}`;
}

/**
 * Format a badge count for display.
 * Returns null if count is 0 (badge hidden), "99+" if count > 99,
 * or the numeric string for counts 1–99.
 */
export function formatBadgeCount(count: number): string | null {
  if (count <= 0) return null;
  if (count > 99) return '99+';
  return String(count);
}

/**
 * Truncate a string to maxLength characters, appending "..." if truncated.
 * Returns the original string unchanged if within maxLength.
 */
export function truncate(text: string, maxLength: number): string {
  if (text.length <= maxLength) return text;
  return text.slice(0, maxLength) + '...';
}

/**
 * Determine the balance status based on ending balance thresholds.
 * - Critical: < 50,000,000
 * - Low: 50,000,000 – 150,000,000 (inclusive)
 * - Normal: > 150,000,000
 */
export function getBalanceStatus(endingBalance: number): BalanceStatus {
  if (endingBalance < 50_000_000) return 'Critical';
  if (endingBalance <= 150_000_000) return 'Low';
  return 'Normal';
}
