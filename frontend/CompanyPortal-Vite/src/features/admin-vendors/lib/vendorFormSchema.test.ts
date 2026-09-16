import { describe, expect, it } from "vitest";
import { vendorFormSchema } from "./vendorFormSchema";

function baseValues(overrides: Partial<Record<string, string>> = {}) {
  return {
    code: "VDR001",
    name: "PT Vendor Sejahtera",
    contact_email: "",
    contact_phone: "",
    hq_address: "",
    ...overrides,
  };
}

describe("vendorFormSchema", () => {
  it("passes with only the required fields (code, name)", () => {
    expect(vendorFormSchema.safeParse(baseValues()).success).toBe(true);
  });

  it("fails when code is empty", () => {
    const result = vendorFormSchema.safeParse(baseValues({ code: "" }));
    expect(result.success).toBe(false);
  });

  it("fails when name is empty", () => {
    const result = vendorFormSchema.safeParse(baseValues({ name: "" }));
    expect(result.success).toBe(false);
  });

  it("passes with a valid contact_email", () => {
    const result = vendorFormSchema.safeParse(baseValues({ contact_email: "vendor@example.com" }));
    expect(result.success).toBe(true);
  });

  it("fails with an invalid contact_email format", () => {
    const result = vendorFormSchema.safeParse(baseValues({ contact_email: "not-an-email" }));
    expect(result.success).toBe(false);
  });
});
