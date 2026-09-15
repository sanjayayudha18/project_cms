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

/**
 * Calendar-day arithmetic in Asia/Jakarta (WIB, fixed UTC+7 offset — no DST)
 * for CIT-2's Replenish_Date default and category locks (Req 2.4, 3.3-3.5).
 * Distinct from nextBusinessDayISO above: that skips weekends for the
 * unrelated Req 11.1 forecast-date default; this is plain calendar H+N,
 * matching the backend's jakartaCalendarDate (vendor_request.go) so the
 * client and server never disagree at the day boundary.
 */
const JAKARTA_OFFSET_MS = 7 * 60 * 60 * 1000;

export function jakartaCalendarDateISO(offsetDays: number, from: Date = new Date()): string {
  const jakartaNow = new Date(from.getTime() + JAKARTA_OFFSET_MS);
  const target = new Date(
    Date.UTC(
      jakartaNow.getUTCFullYear(),
      jakartaNow.getUTCMonth(),
      jakartaNow.getUTCDate() + offsetDays,
    ),
  );
  return `${target.getUTCFullYear()}-${pad2(target.getUTCMonth() + 1)}-${pad2(target.getUTCDate())}`;
}

/** Calendar H+1 in Asia/Jakarta (Req 2.4's Replenish_Date default) — NOT the
 * next business day; see nextBusinessDayISO above for that distinction. */
export function tomorrowJakartaISO(from?: Date): string {
  return jakartaCalendarDateISO(1, from);
}
