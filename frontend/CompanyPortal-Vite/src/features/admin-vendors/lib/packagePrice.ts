import type { AdminVendorPackagePrice } from "../types";

export type PriceStatus = "Berlaku" | "Dijadwalkan" | "Berakhir";

/** Single source of truth for price lifecycle status (Req 9.5). `today` passed in as ISO yyyy-mm-dd. */
export function priceStatus(p: AdminVendorPackagePrice, todayISO: string): PriceStatus {
  if (p.effective_start_date > todayISO) return "Dijadwalkan";
  if (p.effective_end_date !== null && p.effective_end_date <= todayISO) return "Berakhir";
  return "Berlaku";
}

export function machineLabel(g: AdminVendorPackagePrice["machine_group"]): string {
  return g === "CDM_CRM" ? "CDM/CRM" : "ATM";
}

export function classLabel(c: AdminVendorPackagePrice["price_class"]): string {
  return c === "VIP_INDUSTRI" ? "VIP/Industri" : "Regular";
}

export function levelLabel(p: AdminVendorPackagePrice): string {
  if (p.atm_id !== null) return `ATM #${p.atm_id}`;
  if (p.vendor_branch_id !== null) return `Cabang #${p.vendor_branch_id}`;
  return "PT (dasar)";
}

export function tierLabel(p: AdminVendorPackagePrice): string {
  return p.tier_max === null ? `${p.tier_min}+` : `${p.tier_min}-${p.tier_max}`;
}

export function periodLabel(p: AdminVendorPackagePrice): { start: string; end: string } {
  return { start: p.effective_start_date, end: p.effective_end_date ?? "Tidak ditentukan" };
}

/** Grouping only, string-in/string-out. Never Number()/parseFloat() (Req 12.5, Property 3). */
export function formatIDR(decimalString: string): string {
  const [intPart = "", fracPart] = decimalString.split(".");
  const negative = intPart.startsWith("-");
  const digits = negative ? intPart.slice(1) : intPart;
  const grouped = digits.replace(/\B(?=(\d{3})+(?!\d))/g, ".");
  const sign = negative ? "-" : "";
  const frac = fracPart !== undefined ? `,${fracPart}` : "";
  return `IDR ${sign}${grouped}${frac}`;
}

/** String-safe decimal compare (no float). Returns <0, 0, >0 like a normal comparator. */
export function compareDecimalStrings(a: string, b: string): number {
  const parse = (s: string) => {
    const negative = s.startsWith("-");
    const unsigned = negative ? s.slice(1) : s;
    const [intPart = "", fracPart = ""] = unsigned.split(".");
    return { negative, intPart, fracPart };
  };
  const pa = parse(a);
  const pb = parse(b);
  if (pa.negative !== pb.negative) return pa.negative ? -1 : 1;
  const fracLen = Math.max(pa.fracPart.length, pb.fracPart.length);
  const intLen = Math.max(pa.intPart.length, pb.intPart.length);
  const na = pa.intPart.padStart(intLen, "0") + pa.fracPart.padEnd(fracLen, "0");
  const nb = pb.intPart.padStart(intLen, "0") + pb.fracPart.padEnd(fracLen, "0");
  const cmp = na === nb ? 0 : na > nb ? 1 : -1;
  return pa.negative ? -cmp : cmp;
}

export function activePriceRange(
  prices: AdminVendorPackagePrice[],
  todayISO: string,
): { minLabel: string; maxLabel: string } | null {
  const eligible = prices.filter(
    (p) => p.base_price !== null && priceStatus(p, todayISO) === "Berlaku",
  ) as (AdminVendorPackagePrice & { base_price: string })[];
  const first = eligible[0];
  if (first === undefined) return null;
  let min = first.base_price;
  let max = first.base_price;
  for (const p of eligible) {
    if (compareDecimalStrings(p.base_price, min) < 0) min = p.base_price;
    if (compareDecimalStrings(p.base_price, max) > 0) max = p.base_price;
  }
  return { minLabel: formatIDR(min), maxLabel: formatIDR(max) };
}
