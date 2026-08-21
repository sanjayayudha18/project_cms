export type BalanceStatus = "Critical" | "Low" | "Normal";

const CRITICAL_THRESHOLD = 50_000_000;
const LOW_THRESHOLD = 150_000_000;

/**
 * Derives ATM status from the ending balance.
 *
 * - Critical: below 50,000,000 IDR
 * - Low: 50,000,000 to 150,000,000 IDR (inclusive)
 * - Normal: above 150,000,000 IDR
 */
export function deriveStatus(endingBalance: number): BalanceStatus {
  if (endingBalance < CRITICAL_THRESHOLD) {
    return "Critical";
  }
  if (endingBalance <= LOW_THRESHOLD) {
    return "Low";
  }
  return "Normal";
}
