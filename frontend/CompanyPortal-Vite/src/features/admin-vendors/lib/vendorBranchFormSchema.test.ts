import { describe, expect, it } from "vitest";
import { toRegionCode, vendorBranchFormSchema } from "./vendorBranchFormSchema";

function values(region_code: string) {
  return {
    branch_code: "B1",
    branch_name: "Cabang 1",
    location_id: "",
    region: "",
    region_code,
    category: "ATM",
  };
}

describe("vendorBranchFormSchema region_code (replenish-ticket FR11)", () => {
  it("accepts blank and 2-10 letters/digits", () => {
    for (const code of ["", "JKT", "jktbar", "  JABAR ", "SUMATERA"]) {
      expect(vendorBranchFormSchema.safeParse(values(code)).success).toBe(true);
    }
  });

  it("rejects a single char, separators and more than 10 chars", () => {
    for (const code of ["J", "JKT-1", "JKT UTR", "ABCDEFGHIJK"]) {
      expect(vendorBranchFormSchema.safeParse(values(code)).success).toBe(false);
    }
  });

  it("toRegionCode upper-cases and maps blank to null", () => {
    expect(toRegionCode(" jkt ")).toBe("JKT");
    expect(toRegionCode("  ")).toBeNull();
  });
});
