import { describe, expect, it } from "vitest";
import { vendorPackageFormSchema } from "./vendorPackageFormSchema";
import { vendorPackagePriceFormSchema } from "./vendorPackagePriceFormSchema";
import { vendorPicFormSchema } from "./vendorPicFormSchema";
import { vendorVaultFormSchema } from "./vendorVaultFormSchema";

// fe-coverage C3: client-side rules of the master-data forms (server re-validates).

interface ParseResult {
  success: boolean;
  error?: { issues: { path: PropertyKey[]; message: string }[] };
}

function issues(result: ParseResult): Record<string, string> {
  return Object.fromEntries((result.error?.issues ?? []).map((i) => [i.path.join("."), i.message]));
}

const pkg = {
  package_code: "PKG-1",
  machine_group: "ATM",
  price_class: "REGULAR",
  tier_min: "1",
  tier_max: "",
  atm_id: "",
  currency: "IDR",
  effective_start_date: "2026-11-01",
  base_price: "1500000.50",
  sla_note: "",
  effective_end_date: "",
};

describe("vendorPackageFormSchema (paket khusus cabang)", () => {
  it("accepts a valid package with optional fields empty", () => {
    expect(vendorPackageFormSchema.safeParse(pkg).success).toBe(true);
  });

  it.each([
    [{ base_price: "1500000.505" }, "base_price", "Harus angka desimal tidak negatif"],
    [{ base_price: "-1" }, "base_price", "Harus angka desimal tidak negatif"],
    [{ base_price: "1,5" }, "base_price", "Harus angka desimal tidak negatif"],
    [{ tier_min: "" }, "tier_min", "Wajib angka bulat, minimal 1"],
    [{ tier_max: "2.5" }, "tier_max", "Harus angka bulat"],
    [{ atm_id: "abc" }, "atm_id", "Harus angka bulat"],
    [{ currency: "RP" }, "currency", "Kode mata uang 3 huruf"],
    [{ package_code: "" }, "package_code", "Wajib diisi"],
    [{ effective_start_date: "" }, "effective_start_date", "Wajib diisi"],
    [
      { effective_end_date: "2026-10-31" },
      "effective_end_date",
      "Tidak boleh sebelum tanggal mulai",
    ],
  ])("rejects %j", (patch, field, message) => {
    expect(issues(vendorPackageFormSchema.safeParse({ ...pkg, ...patch }))[field]).toBe(message);
  });

  it("allows an end date on or after the start date", () => {
    expect(
      vendorPackageFormSchema.safeParse({ ...pkg, effective_end_date: "2026-11-01" }).success,
    ).toBe(true);
  });
});

const price = {
  package: "PAKET 3",
  machine_group: "CDM_CRM",
  price_class: "VIP_INDUSTRI",
  tier_min: "1",
  tier_max: "10",
  vendor_branch_id: "",
  atm_id: "",
  currency: "IDR",
  effective_start_date: "2026-11-01",
  base_price: "250000",
  sla_note: "",
  effective_end_date: "",
};

describe("vendorPackagePriceFormSchema (tarif vendor-wide)", () => {
  it("accepts a vendor-wide price, a branch override and an ATM override", () => {
    expect(vendorPackagePriceFormSchema.safeParse(price).success).toBe(true);
    expect(
      vendorPackagePriceFormSchema.safeParse({ ...price, vendor_branch_id: "3" }).success,
    ).toBe(true);
    expect(vendorPackagePriceFormSchema.safeParse({ ...price, atm_id: "77" }).success).toBe(true);
  });

  it("an override is for a branch OR an ATM, never both", () => {
    const result = vendorPackagePriceFormSchema.safeParse({
      ...price,
      vendor_branch_id: "3",
      atm_id: "77",
    });
    expect(issues(result).atm_id).toBe("Tidak boleh diisi bersamaan dengan cabang");
  });

  it.each([
    [{ base_price: "0.123" }, "base_price"],
    [{ package: "" }, "package"],
    [{ vendor_branch_id: "x" }, "vendor_branch_id"],
    [{ machine_group: "KIOSK" }, "machine_group"],
    [{ effective_end_date: "2026-01-01" }, "effective_end_date"],
  ])("rejects %j", (patch, field) => {
    expect(issues(vendorPackagePriceFormSchema.safeParse({ ...price, ...patch }))).toHaveProperty(
      field,
    );
  });
});

describe("vendorPicFormSchema", () => {
  const pic = {
    name: "Andi",
    position: "",
    phone: "",
    email: "",
    is_notification_recipient: true,
    roles: ["replenish"],
  };

  it("needs only a name; email is optional but must be valid when given", () => {
    expect(vendorPicFormSchema.safeParse(pic).success).toBe(true);
    expect(vendorPicFormSchema.safeParse({ ...pic, email: "andi@vendor.co.id" }).success).toBe(
      true,
    );
    expect(issues(vendorPicFormSchema.safeParse({ ...pic, email: "andi@" })).email).toBe(
      "Format email tidak valid",
    );
    expect(issues(vendorPicFormSchema.safeParse({ ...pic, name: "" })).name).toBe("Wajib diisi");
  });
});

describe("vendorVaultFormSchema", () => {
  const vault = {
    vault_code: "VLT-JKT-1",
    category: "CASH",
    currency_code: "IDR",
    min_capacity_amount: "",
    max_capacity_amount: "",
    latitude: "",
    longitude: "",
    operating_hours: "",
    location_id: "",
  };

  it("accepts ATM, CASH and ATM_CASH vaults", () => {
    for (const category of ["ATM", "CASH", "ATM_CASH"]) {
      expect(vendorVaultFormSchema.safeParse({ ...vault, category }).success).toBe(true);
    }
  });

  it.each([
    [{ category: "GOLD" }, "category", "Wajib dipilih"],
    [{ currency_code: "" }, "currency_code", "Wajib diisi"],
    [{ currency_code: "ID1" }, "currency_code", "Hanya huruf"],
    [{ currency_code: "IDRX" }, "currency_code", "Wajib 3 huruf"],
    [{ vault_code: "" }, "vault_code", "Wajib diisi"],
    [{ location_id: "x".repeat(21) }, "location_id", "Maksimal 20 karakter"],
  ])("rejects %j", (patch, field, message) => {
    expect(issues(vendorVaultFormSchema.safeParse({ ...vault, ...patch }))[field]).toBe(message);
  });
});
