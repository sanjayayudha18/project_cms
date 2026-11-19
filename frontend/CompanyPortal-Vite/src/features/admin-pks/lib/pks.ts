import type { PksRecordStatus } from "../types";

/** Masa Berlaku: "N hari" (N ≥ 0), "lewat N hari" (N < 0), "-" for diganti/terminated. */
export function formatMasaBerlaku(status: PksRecordStatus, remainingDays: number): string {
  if (status === "diganti" || status === "terminated") return "-";
  return remainingDays >= 0 ? `${remainingDays} hari` : `lewat ${-remainingDays} hari`;
}
