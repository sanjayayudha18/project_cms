/**
 * Dashboard formatter utilities for the CMS redesign.
 * Pure functions for currency display, greetings, dates, progress, badges, and initials.
 *
 * @module formatters
 */

/**
 * Format IDR amount with abbreviation suffix.
 * - Values >= 1 trillion -> "IDR X.XT"
 * - Values >= 1 billion -> "IDR X.XB"
 * - Values >= 1 million -> "IDR X.XM"
 * - Below -> "IDR X" (raw number)
 */
export function formatIDRAbbreviated(value: number): string {
  const abs = Math.abs(value);

  if (abs >= 1_000_000_000_000) {
    return `IDR ${(value / 1_000_000_000_000).toFixed(1)}T`;
  }
  if (abs >= 1_000_000_000) {
    return `IDR ${(value / 1_000_000_000).toFixed(1)}B`;
  }
  if (abs >= 1_000_000) {
    return `IDR ${(value / 1_000_000).toFixed(1)}M`;
  }
  return `IDR ${value}`;
}

/**
 * Format IDR amount with dot-separated thousands (Indonesian convention).
 *
 * @example formatIDRFull(12800000000) // "IDR 12.800.000.000"
 * @example formatIDRFull(0) // "IDR 0"
 */
export function formatIDRFull(value: number): string {
  const formatted = new Intl.NumberFormat('id-ID', {
    style: 'decimal',
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(value);

  return `IDR ${formatted}`;
}

/**
 * Get time-of-day greeting based on hour (0-23).
 *
 * @example getGreeting(9) // "Good morning"
 * @example getGreeting(14) // "Good afternoon"
 * @example getGreeting(20) // "Good evening"
 */
export function getGreeting(hour: number): string {
  if (hour >= 0 && hour <= 11) return 'Good morning';
  if (hour >= 12 && hour <= 17) return 'Good afternoon';
  return 'Good evening';
}

/**
 * Format a Date as full weekday + date string in English locale.
 * Format: "Tuesday, 21 July 2026"
 */
export function formatFullDate(date: Date): string {
  return date.toLocaleDateString('en-GB', {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
    year: 'numeric',
  });
}

/**
 * Calculate progress percentage, guarded against division by zero.
 *
 * @example progressPercent(15, 18) // 83
 * @example progressPercent(0, 0) // 0
 */
export function progressPercent(completed: number, total: number): number {
  if (total === 0) return 0;
  return Math.round((completed / total) * 100);
}

/**
 * Format badge count for display.
 * Returns null for 0 or negative (no badge shown), string for 1-99, "99+" for > 99.
 */
export function formatBadgeCount(count: number): string | null {
  if (count <= 0) return null;
  if (count > 99) return '99+';
  return String(count);
}

/**
 * Extract initials from a name. First letter of first name + first letter of last name, uppercase.
 *
 * @example getInitials("Raden Budiman") // "RB"
 * @example getInitials("Raden") // "R"
 * @example getInitials("") // ""
 */
export function getInitials(name: string): string {
  const trimmed = name.trim();
  if (trimmed === '') return '';

  const parts = trimmed.split(/\s+/);
  if (parts.length === 1) {
    return parts[0]![0]!.toUpperCase();
  }
  return (parts[0]![0]! + parts[parts.length - 1]![0]!).toUpperCase();
}

/**
 * Format a numeric difference with sign prefix and semantic color class.
 * Uses dot-separated thousands for IDR formatting.
 */
export function formatDifference(value: number): { text: string; colorClass: string } {
  if (value === 0) {
    return { text: 'IDR 0', colorClass: '' };
  }

  const absFormatted = new Intl.NumberFormat('id-ID', {
    style: 'decimal',
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(Math.abs(value));

  if (value < 0) {
    return { text: `- IDR ${absFormatted}`, colorClass: 'text-danger' };
  }

  return { text: `+ IDR ${absFormatted}`, colorClass: 'text-success' };
}
