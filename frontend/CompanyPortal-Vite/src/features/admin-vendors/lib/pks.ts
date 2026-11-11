import type { PksStatus } from "../types";

/** Masa berlaku PKS (bulan penuh) → "N tahun M bulan", bagian nol dibuang, 0 → "< 1 bulan". */
export function formatValidity(months: number): string {
  const years = Math.floor(months / 12);
  const rest = months % 12;
  const parts: string[] = [];
  if (years > 0) parts.push(`${years} tahun`);
  if (rest > 0) parts.push(`${rest} bulan`);
  return parts.length > 0 ? parts.join(" ") : "< 1 bulan";
}

/** Monitoring PKS: sisa hari ("N hari"), lewat expired ("lewat N hari"), selain itu "-". Tidak pernah negatif. */
export function formatMonitoring(
  status: PksStatus | null | undefined,
  days: number | null | undefined,
): string {
  if (days == null) return "-";
  if (status === "aktif" || status === "need_renewal") return days >= 0 ? `${days} hari` : "-";
  if (status === "expired") return days < 0 ? `lewat ${-days} hari` : "-";
  return "-";
}
