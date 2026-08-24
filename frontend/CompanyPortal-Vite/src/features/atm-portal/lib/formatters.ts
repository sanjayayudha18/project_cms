/**
 * ATM Portal feature formatters: Rupiah currency, and Indonesian
 * "dd MMM yyyy" date / "HH:mm" time formats (Properties 8, 9 in design.md).
 *
 * Month abbreviations are hand-rolled rather than delegated to
 * `Intl.DateTimeFormat("id-ID", { month: "short" })` — Intl's id-ID short-month
 * output depends on the runtime's ICU data (varies between browsers, Node
 * builds, and CI), which risks producing something other than the exact
 * "15 Jul 2026" shape Property 9 requires. A fixed lookup table is
 * deterministic everywhere.
 *
 * `formatAtmDate` uses UTC getters: `last_replenish_date` is a day-only
 * value ("YYYY-MM-DD", no time component) from the API, so parsing it with
 * `new Date(...)` yields UTC midnight — reading it back with local getters
 * would show the wrong day for any viewer west of UTC. `formatAtmTime`
 * uses local getters instead: `last_updated` is a real instant (an actual
 * UTC timestamp), so converting it to the viewer's local wall-clock time is
 * the standard, correct behavior for a "data terakhir" freshness display.
 */

const INDONESIAN_MONTH_ABBREVIATIONS = [
  "Jan",
  "Feb",
  "Mar",
  "Apr",
  "Mei",
  "Jun",
  "Jul",
  "Agu",
  "Sep",
  "Okt",
  "Nov",
  "Des",
] as const;

const rupiahFormatter = new Intl.NumberFormat("id-ID", {
  style: "decimal",
  minimumFractionDigits: 0,
  maximumFractionDigits: 0,
});

function pad2(n: number): string {
  return n.toString().padStart(2, "0");
}

/**
 * Format a numeric amount as "Rp X.XXX.XXX" (dot-separated thousands, no
 * decimal fraction). Returns "—" for null.
 *
 * @example formatRupiah(1250000) // "Rp 1.250.000"
 * @example formatRupiah(0) // "Rp 0"
 * @example formatRupiah(null) // "—"
 */
export function formatRupiah(value: number | null): string {
  if (value === null) {
    return "—";
  }
  return `Rp ${rupiahFormatter.format(value)}`;
}

/**
 * Format an exact decimal string amount (from cashpos API) as Rupiah without
 * float conversion. Integer part gets thousand separators; fractional part
 * is shown only when non-zero (or always when present and not ".00").
 *
 * @example formatRupiahDecimal("1250000.00") // "Rp 1.250.000"
 * @example formatRupiahDecimal("1250000.50") // "Rp 1.250.000,50"
 * @example formatRupiahDecimal("9999999999999999.99") // "Rp 9.999.999.999.999.999,99"
 */
export function formatRupiahDecimal(value: string | null | undefined): string {
  if (value === null || value === undefined || value === "") {
    return "—";
  }
  const neg = value.startsWith("-");
  const raw = neg ? value.slice(1) : value;
  const [intRaw, fracRaw = ""] = raw.split(".");
  const intDigits = (intRaw || "0").replace(/\D/g, "") || "0";
  // Thousand-separate integer digits from the right without Number().
  let grouped = "";
  for (let i = 0; i < intDigits.length; i++) {
    if (i > 0 && (intDigits.length - i) % 3 === 0) {
      grouped += ".";
    }
    grouped += intDigits[i];
  }
  const fracSignificant = fracRaw.replace(/0+$/, "");
  const body =
    fracSignificant.length > 0 ? `${grouped},${fracSignificant}` : grouped;
  return `Rp ${neg ? `-${body}` : body}`;
}

/**
 * Format a Date as "dd MMM yyyy" using Indonesian month abbreviations and
 * UTC date components (see module doc for why). Returns "—" for null.
 *
 * @example formatAtmDate(new Date("2026-07-15")) // "15 Jul 2026"
 * @example formatAtmDate(null) // "—"
 */
export function formatAtmDate(date: Date | null): string {
  if (date === null) {
    return "—";
  }
  const day = pad2(date.getUTCDate());
  const month = INDONESIAN_MONTH_ABBREVIATIONS[date.getUTCMonth()];
  const year = date.getUTCFullYear();
  return `${day} ${month} ${year}`;
}

/**
 * Format a Date's time-of-day as "HH:mm" using local (viewer) wall-clock
 * time (see module doc for why). Returns "—" for null.
 *
 * @example formatAtmTime(new Date("2026-07-15T14:30:00+07:00")) // e.g. "14:30" when viewed in WIB
 * @example formatAtmTime(null) // "—"
 */
export function formatAtmTime(date: Date | null): string {
  if (date === null) {
    return "—";
  }
  return `${pad2(date.getHours())}:${pad2(date.getMinutes())}`;
}

/**
 * Format a real timestamp as "dd MMM yyyy, HH:mm" (Req 112.2) using local
 * getters throughout — for the data-freshness indicator, where the date
 * and time both derive from one instant and must be self-consistent.
 * Deliberately NOT `formatAtmDate` (UTC) + `formatAtmTime` (local)
 * composed together: near a viewer's local-midnight boundary that pairing
 * can show the wrong calendar day next to a correct local time (e.g. a
 * UTC-Sunday-23:30 instant is already Monday in WIB — formatAtmDate would
 * still say Sunday). Returns "—" for null, though DataFreshnessIndicator
 * handles null with its own "belum tersedia" message before ever calling
 * this.
 *
 * @example formatAtmDateTime(new Date("2026-07-15T07:30:00Z")) // "15 Jul 2026, 14:30" when viewed in WIB
 * @example formatAtmDateTime(null) // "—"
 */
export function formatAtmDateTime(date: Date | null): string {
  if (date === null) {
    return "—";
  }
  const day = pad2(date.getDate());
  const month = INDONESIAN_MONTH_ABBREVIATIONS[date.getMonth()];
  const year = date.getFullYear();
  return `${day} ${month} ${year}, ${pad2(date.getHours())}:${pad2(date.getMinutes())}`;
}
