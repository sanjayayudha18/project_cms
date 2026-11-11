import { describe, expect, it } from "vitest";
import { type MasterDataset, findMasterDataset, parseFilterSearch, toParams } from "./datasets";

const vendors = findMasterDataset("vendors") as MasterDataset;

describe("dataset vendors", () => {
  it("deskripsi menyebut Direktur Utama dan PKS aktif", () => {
    expect(vendors.description).toBe(
      "Daftar vendor beserta kontak, alamat kantor pusat, Direktur Utama, dan PKS aktif.",
    );
  });

  it("numericColumns", () => {
    expect(vendors.numericColumns).toEqual(["id", "pks_validity_months", "pks_remaining_days"]);
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
