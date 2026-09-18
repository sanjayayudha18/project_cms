import { describe, expect, it } from "vitest";
import { atmFormSchema } from "./atmFormSchema";

function baseValues(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    terminal_id: "TATM001",
    location_id: 10,
    machine_type: "ATM",
    brand: "NCR",
    model: "SelfServ",
    operation_hours: "24 Hours",
    deployment_type: "ONSITE",
    capacity_amount: "",
    low_threshold_amount: "",
    critical_threshold_amount: "",
    blacklisted: false,
    escrow_account: "",
    priority_class: "",
    ...overrides,
  };
}

describe("atmFormSchema", () => {
  it("passes with only the required fields", () => {
    expect(atmFormSchema.safeParse(baseValues()).success).toBe(true);
  });

  it.each(["terminal_id", "machine_type", "brand", "model", "operation_hours", "deployment_type"])(
    "fails when %s is empty",
    (field) => {
      const result = atmFormSchema.safeParse(baseValues({ [field]: "" }));
      expect(result.success).toBe(false);
    },
  );

  it("fails when location_id is not selected (0)", () => {
    const result = atmFormSchema.safeParse(baseValues({ location_id: 0 }));
    expect(result.success).toBe(false);
  });

  it.each(["VIP", "Non VIP", "Industri", ""])("accepts priority_class=%s", (value) => {
    const result = atmFormSchema.safeParse(baseValues({ priority_class: value }));
    expect(result.success).toBe(true);
  });

  it("rejects an invalid priority_class", () => {
    const result = atmFormSchema.safeParse(baseValues({ priority_class: "Gold" }));
    expect(result.success).toBe(false);
  });

  it.each(["0", "0.00", "1234567890123456.78", "100"])(
    "accepts a valid decimal amount: %s",
    (value) => {
      const result = atmFormSchema.safeParse(baseValues({ capacity_amount: value }));
      expect(result.success).toBe(true);
    },
  );

  it.each(["-1", "-0.01", "abc", "1.234", "1,000", "1.2.3"])(
    "rejects an invalid decimal amount: %s",
    (value) => {
      const result = atmFormSchema.safeParse(baseValues({ capacity_amount: value }));
      expect(result.success).toBe(false);
    },
  );

  it("treats a blank amount as not-provided (valid)", () => {
    const result = atmFormSchema.safeParse(
      baseValues({ low_threshold_amount: "", critical_threshold_amount: "" }),
    );
    expect(result.success).toBe(true);
  });
});
