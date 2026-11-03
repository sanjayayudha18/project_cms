import { useEffect, useState } from "react";

/** Jam cut-off CIT (WIB), tiap hari — keputusan D10. */
export const CIT_CUTOFF_WIB = "15:00";

const WIB_OFFSET_HOURS = 7; // UTC+7, tanpa DST
const HOUR_MS = 3_600_000;

/** Komponen kalender/jam di WIB, tidak bergantung zona waktu perangkat. */
export function wibParts(now: Date): {
  y: number;
  m: number;
  d: number;
  hh: number;
  mm: number;
  ss: number;
} {
  const s = new Date(now.getTime() + WIB_OFFSET_HOURS * HOUR_MS);
  return {
    y: s.getUTCFullYear(),
    m: s.getUTCMonth() + 1,
    d: s.getUTCDate(),
    hh: s.getUTCHours(),
    mm: s.getUTCMinutes(),
    ss: s.getUTCSeconds(),
  };
}

const opsDateFmt = new Intl.DateTimeFormat("id-ID", {
  timeZone: "Asia/Jakarta",
  day: "numeric",
  month: "short",
});

/** Tanggal operasional, mis. "20 Okt". */
export function formatOpsDate(now: Date): string {
  return opsDateFmt.format(now);
}

/** "YYYY-MM-DD" tanggal kalender WIB. */
export function todayWib(now: Date): string {
  const { y, m, d } = wibParts(now);
  return `${y}-${String(m).padStart(2, "0")}-${String(d).padStart(2, "0")}`;
}

export type CutoffState = { kind: "open" | "warning" | "closed"; remainingMs: number };

/** Status menuju cut-off hari ini (WIB). warning bila sisa < 1 jam, closed bila <= 0. */
export function cutoffState(now: Date, cutoff: string = CIT_CUTOFF_WIB): CutoffState {
  const [ch = 0, cm = 0] = cutoff.split(":").map(Number);
  const { y, m, d } = wibParts(now);
  const cutoffUtcMs = Date.UTC(y, m - 1, d, ch - WIB_OFFSET_HOURS, cm, 0);
  const remainingMs = cutoffUtcMs - now.getTime();
  if (remainingMs <= 0) return { kind: "closed", remainingMs: 0 };
  return { kind: remainingMs < HOUR_MS ? "warning" : "open", remainingMs };
}

/** "HH:MM:SS", dipotong ke >= 0. */
export function formatHms(ms: number): string {
  const total = Math.floor(Math.max(0, ms) / 1000);
  const p = (n: number) => String(n).padStart(2, "0");
  return `${p(Math.floor(total / 3600))}:${p(Math.floor((total % 3600) / 60))}:${p(total % 60)}`;
}

/** Waktu sekarang, diperbarui tiap `intervalMs`. */
export function useNow(intervalMs: number): Date {
  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    const id = setInterval(() => setNow(new Date()), intervalMs);
    return () => clearInterval(id);
  }, [intervalMs]);
  return now;
}
