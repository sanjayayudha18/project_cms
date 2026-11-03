// Feature: app-shell-revamp, Property 5: Hitung mundur cut-off berbasis WIB
import * as fc from "fast-check";
import { afterEach, describe, expect, it } from "vitest";
import { cutoffState, formatHms, todayWib, wibParts } from "./time";

const HOUR_MS = 3_600_000;
const WIB_OFFSET_MS = 7 * HOUR_MS;
const ZONES = ["UTC", "America/New_York", "Asia/Tokyo", "Pacific/Kiritimati", "Asia/Jakarta"];
const originalTz = process.env.TZ;

// Referensi independen: tanggal WIB lewat ISO string, cut-off lewat offset +07:00 eksplisit.
const wibDate = (t: number) => new Date(t + WIB_OFFSET_MS).toISOString().slice(0, 10);
const refCutoff = (t: number) => Date.parse(`${wibDate(t)}T15:00:00+07:00`);

const instant = fc.integer({ min: Date.UTC(2000, 0, 1), max: Date.UTC(2100, 0, 1) });
// kasus tepi: sekitar cut-off, 1 jam sebelum cut-off, dan pergantian hari WIB
const nearBoundary = fc
  .tuple(instant, fc.constantFrom(0, -HOUR_MS, 9 * HOUR_MS), fc.integer({ min: -2000, max: 2000 }))
  .map(([t, base, jitter]) => refCutoff(t) + base + jitter);

afterEach(() => {
  if (originalTz === undefined) {
    // `= undefined` akan menjadi string "undefined" di process.env
    Reflect.deleteProperty(process.env, "TZ");
  } else {
    process.env.TZ = originalTz;
  }
});

function expectMatchesReference(t: number) {
  const diff = refCutoff(t) - t;
  const got = cutoffState(new Date(t));
  expect(got.remainingMs).toBe(Math.max(0, diff));
  expect(got.kind).toBe(diff <= 0 ? "closed" : diff < HOUR_MS ? "warning" : "open");
}

describe("Property 5: cutoffState / formatHms / todayWib", () => {
  it("cutoffState sama dengan perhitungan referensi untuk instant apa pun", () => {
    fc.assert(fc.property(fc.oneof(instant, nearBoundary), expectMatchesReference), {
      numRuns: 300,
    });
  });

  it("todayWib dan wibParts konsisten dengan tanggal WIB referensi", () => {
    fc.assert(
      fc.property(instant, (t) => {
        const p = wibParts(new Date(t));
        const ref = new Date(t + WIB_OFFSET_MS);
        expect(todayWib(new Date(t))).toBe(wibDate(t));
        expect([p.hh, p.mm, p.ss]).toEqual([
          ref.getUTCHours(),
          ref.getUTCMinutes(),
          ref.getUTCSeconds(),
        ]);
      }),
      { numRuns: 300 },
    );
  });

  it("formatHms berpola HH:MM:SS dan bolak-balik ke detik yang sama", () => {
    fc.assert(
      fc.property(fc.integer({ min: -10_000, max: 99 * HOUR_MS }), (ms) => {
        const s = formatHms(ms);
        expect(s).toMatch(/^\d{2}:\d{2}:\d{2}$/);
        const [h = 0, m = 0, sec = 0] = s.split(":").map(Number);
        expect(m).toBeLessThan(60);
        expect(sec).toBeLessThan(60);
        expect(h * 3600 + m * 60 + sec).toBe(Math.floor(Math.max(0, ms) / 1000));
      }),
      { numRuns: 300 },
    );
  });

  // Implementasi hanya memakai API UTC; process.env.TZ diubah saat runtime (Node mereset cache
  // zona waktu pada assignment) untuk membuktikan hasil tidak bergantung zona proses.
  it.each(ZONES)("hasil identik saat TZ proses = %s", (zone) => {
    process.env.TZ = zone;
    fc.assert(
      fc.property(fc.oneof(instant, nearBoundary), (t) => {
        expectMatchesReference(t);
        expect(todayWib(new Date(t))).toBe(wibDate(t));
      }),
      { numRuns: 100 },
    );
  });

  it("TZ proses benar-benar berubah di test ini (guard agar tes TZ bermakna)", () => {
    const probe = new Date(Date.UTC(2026, 0, 1, 12, 0, 0));
    process.env.TZ = "Asia/Tokyo";
    const tokyo = probe.getHours();
    process.env.TZ = "America/New_York";
    expect(probe.getHours()).not.toBe(tokyo);
  });
});
