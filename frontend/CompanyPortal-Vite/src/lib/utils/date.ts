/**
 * Formats a Date object using Indonesian locale ("id-ID").
 *
 * Default format outputs: "2 Januari 2024" (day month year).
 */
export function formatDate(
  date: Date,
  options: Intl.DateTimeFormatOptions = {
    day: "numeric",
    month: "long",
    year: "numeric",
  },
): string {
  return date.toLocaleDateString("id-ID", options);
}

/**
 * Formats a Date object as a short Indonesian date.
 *
 * Output example: "02/01/2024" (DD/MM/YYYY).
 */
export function formatDateShort(date: Date): string {
  return date.toLocaleDateString("id-ID", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}

/**
 * Formats a Date object with time in Indonesian locale.
 *
 * Output example: "2 Januari 2024, 14:30".
 */
export function formatDateTime(date: Date): string {
  return date.toLocaleDateString("id-ID", {
    day: "numeric",
    month: "long",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}
