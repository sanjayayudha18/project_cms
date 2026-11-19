import fc from "fast-check";
import { describe, expect, it } from "vitest";
import { formatMasaBerlaku } from "../lib/pks";
import { pksDurationDays, pksFormSchema } from "../lib/pksForm";
import type { PksRecordStatus } from "../types";
import { ADMIN_PKS_SEARCH_SCHEMA, omitDefaults, parseParams } from "../useAdminPksUrlState";

const ALL: PksRecordStatus[] = [
  "aktif",
  "need_renewal",
  "belum_berlaku",
  "expired",
  "terminated",
  "diganti",
  "menunggu_persetujuan",
];

describe("formatMasaBerlaku (Property 9)", () => {
  it("is '-' for diganti and terminated whatever the remaining days", () => {
    fc.assert(
      fc.property(
        fc.constantFrom<PksRecordStatus>("diganti", "terminated"),
        fc.integer({ min: -4000, max: 4000 }),
        (status, days) => formatMasaBerlaku(status, days) === "-",
      ),
    );
  });

  it("is 'N hari' for N >= 0 and 'lewat N hari' for N < 0 on every other status", () => {
    const others = ALL.filter((s) => s !== "diganti" && s !== "terminated");
    fc.assert(
      fc.property(
        fc.constantFrom(...others),
        fc.integer({ min: -4000, max: 4000 }),
        (s, n) => formatMasaBerlaku(s, n) === (n >= 0 ? `${n} hari` : `lewat ${-n} hari`),
      ),
    );
  });
});

describe("Daftar PKS URL state (Req 5.3)", () => {
  it("defaults to status terkini and omits defaults from the URL", () => {
    expect(parseParams({})).toMatchObject({ status: "terkini", page: 1, page_size: 25, q: "" });
    expect(omitDefaults(parseParams({}))).toEqual({});
  });

  it("round-trips filters and ignores an unknown status", () => {
    const p = parseParams({ status: "expired", pks_type: "FLM", vendor: "V1", q: "abc", page: 2 });
    expect(parseParams(omitDefaults(p))).toEqual(p);
    expect(parseParams({ status: "bogus" }).status).toBe("terkini");
  });

  it("search schema accepts known statuses only", () => {
    expect(ADMIN_PKS_SEARCH_SCHEMA.safeParse({ status: "semua" }).success).toBe(true);
    expect(ADMIN_PKS_SEARCH_SCHEMA.safeParse({ status: "bogus" }).success).toBe(false);
  });
});

describe("PKS form schema", () => {
  const valid = {
    vendor_id: "1",
    pks_type: "FLM",
    wod_number: "WOD-1",
    pks_name: "PKS",
    effective_date: "2026-01-01",
    expiry_date: "2027-01-01",
    renewal_days: 270,
  };

  it("accepts a valid form", () => {
    expect(pksFormSchema.safeParse(valid).success).toBe(true);
  });

  it("requires reminder, bounds it 30-3666 and caps it at the PKS duration", () => {
    expect(pksFormSchema.safeParse({ ...valid, renewal_days: Number.NaN }).success).toBe(false);
    expect(pksFormSchema.safeParse({ ...valid, renewal_days: 29 }).success).toBe(false);
    expect(pksFormSchema.safeParse({ ...valid, renewal_days: 3667 }).success).toBe(false);
    expect(pksFormSchema.safeParse({ ...valid, renewal_days: 366 }).success).toBe(false);
    expect(pksFormSchema.safeParse({ ...valid, renewal_days: 365 }).success).toBe(true);
  });

  it("computes the duration in days", () => {
    expect(pksDurationDays("2026-01-01", "2027-01-01")).toBe(365);
    expect(pksDurationDays("x", "2027-01-01")).toBeNull();
  });
});
