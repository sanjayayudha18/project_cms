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
    vault_name: "ABA_Bali_V001",
    category: "CASH",
    currency_code: "IDR",
    alamat: "",
    min_balance: "",
    max_balance: "",
    cis_limit_amount: "",
    escrow_cash: "",
    escrow_atm: "",
    operating_hours: "",
  };

  it("accepts ATM, CASH and ATM_CASH vaults (including empty vault_name for auto-generation)", () => {
    for (const category of ["ATM", "CASH", "ATM_CASH"]) {
      expect(vendorVaultFormSchema.safeParse({ ...vault, category }).success).toBe(true);
    }
    expect(vendorVaultFormSchema.safeParse({ ...vault, vault_name: "" }).success).toBe(true);
    expect(
      vendorVaultFormSchema.safeParse({
        ...vault,
        cis_limit_amount: "2500000000.00",
        escrow_cash: "001234567890",
        escrow_atm: "998877665544",
      }).success,
    ).toBe(true);
  });

  it.each([
    [{ category: "GOLD" }, "category", "Wajib dipilih"],
    [{ currency_code: "" }, "currency_code", "Wajib diisi"],
    [{ currency_code: "ID1" }, "currency_code", "Hanya huruf"],
    [{ currency_code: "IDRX" }, "currency_code", "Wajib 3 huruf"],
    [{ vault_name: "x".repeat(101) }, "vault_name", "Maksimal 100 karakter"],
    [{ alamat: "x".repeat(501) }, "alamat", "Maksimal 500 karakter"],
    [{ cis_limit_amount: "0" }, "cis_limit_amount", "Harus angka desimal > 0, maksimal 2 desimal"],
    [
      { cis_limit_amount: "-100" },
      "cis_limit_amount",
      "Harus angka desimal > 0, maksimal 2 desimal",
    ],
    [
      { cis_limit_amount: "100.123" },
      "cis_limit_amount",
      "Harus angka desimal > 0, maksimal 2 desimal",
    ],
    [{ escrow_cash: "12345678901" }, "escrow_cash", "Harus tepat 12 digit angka"],
    [{ escrow_atm: "12345678901A" }, "escrow_atm", "Harus tepat 12 digit angka"],
  ])("rejects %j", (patch, field, message) => {
    expect(issues(vendorVaultFormSchema.safeParse({ ...vault, ...patch }))[field]).toBe(message);
  });
});
