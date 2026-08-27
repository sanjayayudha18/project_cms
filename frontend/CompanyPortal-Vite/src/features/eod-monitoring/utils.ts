const WIB_TIME_ZONE = "Asia/Jakarta";

const dateTimePartsFormatter = new Intl.DateTimeFormat("en-GB", {
  timeZone: WIB_TIME_ZONE,
  day: "2-digit",
  month: "short",
  year: "numeric",
  hour: "2-digit",
  minute: "2-digit",
  second: "2-digit",
  hour12: false,
});

function getWibParts(iso: string): Record<string, string> {
  const parts = dateTimePartsFormatter.formatToParts(new Date(iso));
  const map: Record<string, string> = {};
  for (const part of parts) {
    map[part.type] = part.value;
  }
  return map;
}

/** Formats an ISO timestamp as "DD MMM YYYY HH:mm" in Asia/Jakarta. */
export function formatWibDateTime(iso: string): string {
  const p = getWibParts(iso);
  return `${p.day} ${p.month} ${p.year} ${p.hour}:${p.minute}`;
}

/** Formats an ISO timestamp as "DD MMM YYYY HH:mm:ss" in Asia/Jakarta. */
export function formatWibDateTimeSec(iso: string): string {
  const p = getWibParts(iso);
  return `${p.day} ${p.month} ${p.year} ${p.hour}:${p.minute}:${p.second}`;
}

/** Formats a "HH:mm" or "HH:mm:ss" time string as "HH:mm WIB". */
export function formatSlaTime(timeStr: string): string {
  const parts = timeStr.split(":");
  return `${parts[0]}:${parts[1]} WIB`;
}

/** Formats a millisecond duration as a locale-formatted number + " ms", or "-" when null. */
export function formatDuration(ms: number | null): string {
  if (ms === null) return "-";
  return `${ms.toLocaleString("id-ID")} ms`;
}

const isoDateFormatter = new Intl.DateTimeFormat("en-CA", {
  timeZone: WIB_TIME_ZONE,
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
});

/** Returns today's date as "YYYY-MM-DD" in Asia/Jakarta. */
export function getTodayWib(): string {
  return isoDateFormatter.format(new Date());
}
