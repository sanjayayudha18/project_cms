import { beforeEach, describe, expect, it, vi } from "vitest";

const client = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }));
vi.mock("@/lib/api", () => ({ api: client }));
const save = vi.hoisted(() => vi.fn());
vi.mock("@/lib/download", () => ({ saveBlobResponse: save }));

import {
  downloadExportFile,
  downloadMasterDataTemplate,
  getExportJob,
  isTooManyExports,
  listMyExports,
  requestBranchExport,
  requestMasterDataExport,
  requestPackagePriceExport,
  requestReportExport,
  requestVendorExport,
} from "./api";
import { claimAutoDownload, resetAutoDownloadClaims } from "./autoDownload";
import { KIND_LABEL, REPORT_KEY_BY_KIND, STATUS_META, isReportKind } from "./status";

const job = { id: 7, status: "queued" };

beforeEach(() => {
  client.get.mockReset();
  client.post.mockReset();
  save.mockReset();
});

describe("request endpoints", () => {
  it("POSTs vendor export and maps 202 to created", async () => {
    client.post.mockResolvedValue({ status: 202, data: { success: true, data: job } });
    const r = await requestVendorExport(3, "branches", "all");
    expect(client.post).toHaveBeenCalledWith("/exports/vendors/3/branches?status=all");
    expect(r).toEqual({ job, created: true });
  });

  it("maps 200 to created=false (existing job reused)", async () => {
    client.post.mockResolvedValue({ status: 200, data: { success: true, data: job } });
    expect((await requestVendorExport(3, "vendor")).created).toBe(false);
    expect(client.post).toHaveBeenCalledWith("/exports/vendors/3/vendor");
  });

  it("builds branch, package-price and master-data paths", async () => {
    client.post.mockResolvedValue({ status: 202, data: { success: true, data: job } });
    await requestBranchExport(3, 9, "pics", "active");
    await requestPackagePriceExport(3, { branchId: 9, priceLevel: "atm" });
    await requestMasterDataExport("atms", "disabled");
    expect(client.post.mock.calls.map((c) => c[0])).toEqual([
      "/exports/vendors/3/branches/9/pics?status=active",
      "/exports/vendors/3/package-prices?branch_id=9&price_level=atm",
      "/exports/master-data/atms?status=disabled",
    ]);
  });
});

describe("requestReportExport", () => {
  it("POSTs to /exports/reports/{report} with legacy query names, skipping undefined", async () => {
    client.post.mockResolvedValue({ status: 202, data: { success: true, data: job } });
    const r = await requestReportExport("replenishment-trips", {
      date_from: "2026-10-01",
      date_to: "2026-10-31",
      vendor_id: 4,
      region_code: undefined,
    });
    expect(client.post).toHaveBeenCalledWith(
      "/exports/reports/replenishment-trips?date_from=2026-10-01&date_to=2026-10-31&vendor_id=4",
    );
    expect(r).toEqual({ job, created: true });
  });

  it("sends no query string for empty params and maps 200 to created=false", async () => {
    client.post.mockResolvedValue({ status: 200, data: { success: true, data: job } });
    const r = await requestReportExport("dsr-late-summary", {});
    expect(client.post).toHaveBeenCalledWith("/exports/reports/dsr-late-summary");
    expect(r.created).toBe(false);
  });
});

describe("report kind labels", () => {
  it("labels all 8 kinds in Indonesian and maps report kinds to URL keys", () => {
    expect(Object.keys(KIND_LABEL)).toHaveLength(8);
    expect(KIND_LABEL.dsr_late_daily).toBe("Laporan DSR telat (harian)");
    expect(KIND_LABEL.replenishment_trips).toBe("Replenish: trip per ATM");
    expect(REPORT_KEY_BY_KIND.replenishment_amount_mismatch).toBe("replenishment-amount-mismatch");
    expect(isReportKind("dsr_late_summary")).toBe(true);
    expect(isReportKind("vendor")).toBe(false);
  });
});

describe("reads and downloads", () => {
  it("lists and gets jobs", async () => {
    client.get.mockResolvedValueOnce({ data: { success: true, data: [job] } });
    expect(await listMyExports(20)).toEqual([job]);
    expect(client.get).toHaveBeenLastCalledWith("/exports/jobs?limit=20");
    client.get.mockResolvedValueOnce({ data: { success: true, data: job } });
    expect(await getExportJob(7)).toEqual(job);
    expect(client.get).toHaveBeenLastCalledWith("/exports/jobs/7");
  });

  it("downloads file and template as blobs through saveBlobResponse", async () => {
    const res = { data: new Blob(["x"]), headers: new Headers() };
    client.get.mockResolvedValue(res);
    await downloadExportFile(7, "a.csv");
    expect(client.get).toHaveBeenLastCalledWith("/exports/jobs/7/file", { responseType: "blob" });
    expect(save).toHaveBeenLastCalledWith(res, "a.csv");
    await downloadMasterDataTemplate("vendors");
    expect(client.get).toHaveBeenLastCalledWith("/exports/master-data/vendors/template", {
      responseType: "blob",
    });
  });
});

describe("isTooManyExports", () => {
  it("matches 429 too_many_exports only", () => {
    expect(isTooManyExports({ status: 429, code: "too_many_exports", message: "x" })).toBe(true);
    expect(isTooManyExports({ status: 429, code: "other", message: "x" })).toBe(false);
    expect(isTooManyExports({ status: 500, message: "x" })).toBe(false);
    expect(isTooManyExports(new Error("x"))).toBe(false);
  });
});

describe("claimAutoDownload", () => {
  it("is true only the first time per job", () => {
    resetAutoDownloadClaims();
    expect(claimAutoDownload(1)).toBe(true);
    expect(claimAutoDownload(1)).toBe(false);
    expect(claimAutoDownload(2)).toBe(true);
  });
});

describe("STATUS_META", () => {
  it("covers every status with a label", () => {
    expect(Object.keys(STATUS_META).sort()).toEqual(
      ["downloaded", "downloading", "expired", "failed", "queued", "running", "succeeded"].sort(),
    );
    expect(STATUS_META.succeeded.label).toBe("Siap diunduh");
  });
});
