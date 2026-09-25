import { describe, expect, it } from "vitest";
import type { AdminVendorPackagePrice } from "../types";
import {
  activePriceRange,
  classLabel,
  compareDecimalStrings,
  formatIDR,
  levelLabel,
  machineLabel,
  periodLabel,
  priceStatus,
  tierLabel,
} from "./packagePrice";

function basePrice(overrides: Partial<AdminVendorPackagePrice> = {}): AdminVendorPackagePrice {
  return {
    id: 1,
    vendor_id: 1,
    package: "PAKET 3",
    package_code: "PKG3_ABA_001",
    machine_group: "ATM",
    price_class: "REGULAR",
    tier_min: 1,
    tier_max: 50,
    base_price: "1000000.00",
    vendor_branch_id: null,
    atm_id: null,
    sla_note: null,
    currency: "IDR",
    effective_start_date: "2026-01-01",
    effective_end_date: null,
    ...overrides,
  };
}

const TODAY = "2026-06-15";

describe("priceStatus", () => {
  it("classifies a future start date as Dijadwalkan", () => {
    expect(priceStatus(basePrice({ effective_start_date: "2026-07-01" }), TODAY)).toBe(
      "Dijadwalkan",
    );
  });

  it("classifies start == today as not Dijadwalkan (Berlaku)", () => {
    expect(
      priceStatus(basePrice({ effective_start_date: TODAY, effective_end_date: null }), TODAY),
    ).toBe("Berlaku");
  });

  it("classifies end == today as Berakhir", () => {
    expect(
      priceStatus(
        basePrice({ effective_start_date: "2026-01-01", effective_end_date: TODAY }),
        TODAY,
      ),
    ).toBe("Berakhir");
  });

  it("classifies an end date in the past as Berakhir", () => {
    expect(
      priceStatus(
        basePrice({ effective_start_date: "2026-01-01", effective_end_date: "2026-06-01" }),
        TODAY,
      ),
    ).toBe("Berakhir");
  });

  it("classifies an open end date (null) with a past start as Berlaku", () => {
    expect(
      priceStatus(
        basePrice({ effective_start_date: "2026-01-01", effective_end_date: null }),
        TODAY,
      ),
    ).toBe("Berlaku");
  });

  it("classifies a future end date with a past start as Berlaku", () => {
    expect(
      priceStatus(
        basePrice({ effective_start_date: "2026-01-01", effective_end_date: "2026-12-31" }),
        TODAY,
      ),
    ).toBe("Berlaku");
  });
});

describe("formatIDR", () => {
  it("groups a whole-number integer part with dot separators", () => {
    expect(formatIDR("2003100.00")).toBe("IDR 2.003.100,00");
  });

  it("preserves the decimal part unchanged", () => {
    expect(formatIDR("1000000.50")).toBe("IDR 1.000.000,50");
  });

  it("preserves every digit from the input (no float rounding)", () => {
    const input = "123456789.99";
    const output = formatIDR(input);
    const digitsIn = input.replace(/[.,]/g, "");
    const digitsOut = output.replace(/^IDR\s*/, "").replace(/[.,]/g, "");
    expect(digitsOut).toBe(digitsIn);
  });

  it("handles a value with no decimal part", () => {
    expect(formatIDR("500")).toBe("IDR 500");
  });

  it("handles small integer parts without inserting a separator", () => {
    expect(formatIDR("999.00")).toBe("IDR 999,00");
  });

  it("handles a negative amount", () => {
    expect(formatIDR("-1000000.00")).toBe("IDR -1.000.000,00");
  });
});

describe("compareDecimalStrings", () => {
  it("treats numerically equal strings as equal regardless of padding", () => {
    expect(compareDecimalStrings("1000000.00", "1000000.0")).toBe(0);
  });

  it("orders a smaller integer part before a larger one", () => {
    expect(compareDecimalStrings("999.00", "1000.00")).toBeLessThan(0);
  });

  it("orders by decimal part when integer parts are equal", () => {
    expect(compareDecimalStrings("1000.10", "1000.50")).toBeLessThan(0);
  });

  it("orders a negative value before a positive one", () => {
    expect(compareDecimalStrings("-1.00", "1.00")).toBeLessThan(0);
  });
});

describe("activePriceRange", () => {
  it("returns null when there are no Berlaku rows", () => {
    const prices = [basePrice({ effective_start_date: "2026-07-01" })];
    expect(activePriceRange(prices, TODAY)).toBeNull();
  });

  it("returns null when the only Berlaku row has a null base_price", () => {
    const prices = [basePrice({ base_price: null })];
    expect(activePriceRange(prices, TODAY)).toBeNull();
  });

  it("returns the same value for min and max when there is one eligible row", () => {
    const prices = [basePrice({ base_price: "1500000.00" })];
    expect(activePriceRange(prices, TODAY)).toEqual({
      minLabel: "IDR 1.500.000,00",
      maxLabel: "IDR 1.500.000,00",
    });
  });

  it("picks min and max among several eligible rows, ignoring non-Berlaku and null rows", () => {
    const prices = [
      basePrice({ id: 1, base_price: "2000000.00" }),
      basePrice({ id: 2, base_price: "500000.00" }),
      basePrice({ id: 3, base_price: "9999999.00" }),
      basePrice({ id: 4, base_price: null }),
      basePrice({ id: 5, base_price: "1.00", effective_start_date: "2026-07-01" }),
    ];
    expect(activePriceRange(prices, TODAY)).toEqual({
      minLabel: "IDR 500.000,00",
      maxLabel: "IDR 9.999.999,00",
    });
  });
});

describe("label mappers", () => {
  it("maps machine_group CDM_CRM to the CDM/CRM label", () => {
    expect(machineLabel("CDM_CRM")).toBe("CDM/CRM");
  });

  it("maps machine_group ATM to the ATM label", () => {
    expect(machineLabel("ATM")).toBe("ATM");
  });

  it("maps price_class VIP_INDUSTRI to the VIP/Industri label", () => {
    expect(classLabel("VIP_INDUSTRI")).toBe("VIP/Industri");
  });

  it("maps price_class REGULAR to the Regular label", () => {
    expect(classLabel("REGULAR")).toBe("Regular");
  });

  it("labels a base PT-level row with no override", () => {
    expect(levelLabel(basePrice({ vendor_branch_id: null, atm_id: null }))).toBe("PT (dasar)");
  });

  it("labels a branch-scoped override row", () => {
    expect(levelLabel(basePrice({ vendor_branch_id: 7, atm_id: null }))).toBe("Cabang #7");
  });

  it("labels an ATM-scoped override row, taking priority over a branch id", () => {
    expect(levelLabel(basePrice({ vendor_branch_id: 7, atm_id: 42 }))).toBe("ATM #42");
  });

  it("renders an open-ended upper tier bound with a plus sign", () => {
    expect(tierLabel(basePrice({ tier_min: 251, tier_max: null }))).toBe("251+");
  });

  it("renders a closed tier range", () => {
    expect(tierLabel(basePrice({ tier_min: 1, tier_max: 50 }))).toBe("1-50");
  });

  it("renders an open-ended effective period distinctly from a fixed end date", () => {
    expect(periodLabel(basePrice({ effective_end_date: null })).end).toBe("Tidak ditentukan");
  });

  it("renders a fixed effective end date as-is", () => {
    expect(periodLabel(basePrice({ effective_end_date: "2026-12-31" })).end).toBe("2026-12-31");
  });
});
