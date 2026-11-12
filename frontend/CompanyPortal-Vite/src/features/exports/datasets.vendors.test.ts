import { describe, expect, it } from "vitest";
import { type MasterDataset, findMasterDataset, parseFilterSearch, toParams } from "./datasets";

const vendors = findMasterDataset("vendors") as MasterDataset;
const vendorPks = findMasterDataset("vendor-pks") as MasterDataset;

describe("dataset vendors", () => {
  it("deskripsi profil vendor tanpa Direktur Utama dan PKS", () => {
    expect(vendors.description).toBe("Daftar vendor beserta kontak dan alamat kantor pusat.");
  });

  it("numericColumns kosong", () => {
    expect(vendors.numericColumns).toEqual([]);
    expect(vendors.moneyColumns).toEqual([]);
  });

  it("filter hanya Status (tanpa vendor/cabang/level harga)", () => {
    expect(vendors.filters).toEqual({ vendor: "none", branch: false, priceLevel: false });
    expect(
      parseFilterSearch("vendors", { status: "all", vendorId: "5", pks_status: "aktif" }),
    ).toEqual({ status: "all" });
    expect(toParams("vendors", { status: "disabled" })).toEqual({
      kind: "master_data_global",
      entity: "vendors",
      status: "disabled",
    });
  });

  it("peran tetap ADMIN dan ADMIN_PARAM", () => {
    expect(vendors.roles).toEqual(["ADMIN", "ADMIN_PARAM"]);
  });
});

describe("dataset vendor-pks", () => {
  it("label dan deskripsi menyebut Direktur Utama dan PKS aktif", () => {
    expect(vendorPks.label).toBe("PKS vendor");
    expect(vendorPks.description).toBe("Daftar vendor beserta Direktur Utama dan PKS aktif.");
  });

  it("numericColumns", () => {
    expect(vendorPks.numericColumns).toEqual(["pks_validity_months", "pks_remaining_days"]);
    expect(vendorPks.moneyColumns).toEqual([]);
  });

  it("filter vendor bersifat opsional", () => {
    expect(vendorPks.filters).toEqual({ vendor: "optional", branch: false, priceLevel: false });
    expect(parseFilterSearch("vendor-pks", { status: "all", vendorId: "5" })).toEqual({
      status: "all",
      vendorId: 5,
    });
    expect(toParams("vendor-pks", { status: "active" })).toEqual({
      kind: "master_data_global",
      entity: "vendor-pks",
      status: "active",
    });
    expect(toParams("vendor-pks", { status: "disabled", vendorId: 3 })).toEqual({
      kind: "vendor",
      vendorId: 3,
      entity: "vendor-pks",
      status: "disabled",
    });
  });

  it("peran ADMIN dan ADMIN_PARAM", () => {
    expect(vendorPks.roles).toEqual(["ADMIN", "ADMIN_PARAM"]);
  });
});
