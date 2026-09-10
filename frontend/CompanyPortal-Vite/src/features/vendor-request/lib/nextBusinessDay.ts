/**
 * Next business day (Req 11.1: date picker defaults to the next business
 * day, Monday-Friday). Local-date formatting (not toISOString(), which is
 * UTC and can roll the day back/forward depending on timezone offset) —
 * same convention as atm-portal/lib/formatters.ts's todayISO().
 *
 * ponytail: does NOT exclude Indonesian public holidays, even though Req
 * 11.1 asks for that too — there is no holiday-calendar data source
 * anywhere in this codebase to drive it honestly, and hand-typing a year
 * of holiday dates here risks silently shipping wrong dates into an ops
 * tool that schedules cash replenishment. Wire a real source (a maintained
 * holiday list synced from an authoritative calendar, or a calendar API)
 * and filter on it here when one exists.
 */
function pad2(n: number): string {
  return String(n).padStart(2, "0");
}

function toISODateLocal(d: Date): string {
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`;
}

export function nextBusinessDayISO(from: Date = new Date()): string {
  const next = new Date(from);
  next.setDate(next.getDate() + 1);
  while (next.getDay() === 0 || next.getDay() === 6) {
    next.setDate(next.getDate() + 1);
  }
  return toISODateLocal(next);
}
