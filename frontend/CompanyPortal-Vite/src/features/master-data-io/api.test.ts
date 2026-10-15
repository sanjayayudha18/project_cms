import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// fe-coverage C3: master-data CSV export/import client. The HTTP client is mocked;
// jsdom has no object URLs, so those and the anchor click are stubbed.
const client = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }));
vi.mock("@/lib/api", () => ({ api: client }));

import {
  ImportApiError,
  confirmImport,
  downloadExport,
  downloadTemplate,
  dryRunImport,
} from "./api";

const createObjectURL = vi.fn(() => "blob:fake");
const revokeObjectURL = vi.fn();
let clicked: { href: string; download: string }[] = [];

beforeEach(() => {
  client.get.mockReset();
  client.post.mockReset();
  createObjectURL.mockClear();
  revokeObjectURL.mockClear();
  clicked = [];
  vi.stubGlobal("URL", Object.assign(URL, { createObjectURL, revokeObjectURL }));
  vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(function (
    this: HTMLAnchorElement,
  ) {
    clicked.push({ href: this.getAttribute("href") ?? "", download: this.download });
  });
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

const blobResponse = (disposition: string | null) => ({
  data: new Blob(["code,name\n"]),
  status: 200,
  headers: new Headers(disposition ? { "Content-Disposition": disposition } : {}),
});

describe("master-data CSV download", () => {
  it("export asks for a blob and saves it under the server's file name", async () => {
    client.get.mockResolvedValue(blobResponse('attachment; filename="vendors-2026-10-15.csv"'));

    await downloadExport("vendors", "all");

    expect(client.get).toHaveBeenCalledWith("/admin/master-data/export/vendors?status=all", {
      responseType: "blob",
    });
    expect(clicked).toEqual([{ href: "blob:fake", download: "vendors-2026-10-15.csv" }]);
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:fake");
  });

  it("template falls back to a local file name when there is no Content-Disposition", async () => {
    client.get.mockResolvedValue(blobResponse(null));

    await downloadTemplate("atm-assignments");

    expect(client.get).toHaveBeenCalledWith("/admin/master-data/export/atm-assignments/template", {
      responseType: "blob",
    });
    expect(clicked[0]?.download).toBe("master-data-atm-assignments-template.csv");
  });

  it("a failed download becomes an ImportApiError and nothing is saved", async () => {
    client.get.mockRejectedValue({ status: 403, message: "Akses ditolak" });

    const err = await downloadExport("atms", "active").catch((e: unknown) => e);

    expect(err).toBeInstanceOf(ImportApiError);
    expect(err).toMatchObject({ status: 403, message: "Akses ditolak", errors: [] });
    expect(clicked).toEqual([]);
  });
});

describe("master-data CSV import", () => {
  const file = new File(["code,name\nV1,Vendor\n"], "vendors.csv", { type: "text/csv" });

  it("dry-run and confirm post the file as multipart to the entity endpoints", async () => {
    client.post.mockResolvedValueOnce({ data: { total_rows: 1, errors: [] } });
    client.post.mockResolvedValueOnce({ data: { batch_id: 3, approval_request_id: 9 } });

    await expect(dryRunImport("vendors", file)).resolves.toEqual({ total_rows: 1, errors: [] });
    await expect(confirmImport("vendors", file)).resolves.toEqual({
      batch_id: 3,
      approval_request_id: 9,
    });

    const [[dryPath, dryBody], [confirmPath]] = client.post.mock.calls as [string, FormData][];
    expect(dryPath).toBe("/admin/master-data/import/vendors/dry-run");
    expect(confirmPath).toBe("/admin/master-data/import/vendors/confirm");
    expect(dryBody).toBeInstanceOf(FormData);
    expect((dryBody.get("file") as File).name).toBe("vendors.csv");
  });

  it("a 422 keeps the per-row errors so the screen can list them", async () => {
    const rows = [{ row: 2, field: "code", message: "wajib diisi" }];
    client.post.mockRejectedValue({ status: 422, message: "CSV tidak valid", details: rows });

    const err = await dryRunImport("vendor-pics", file).catch((e: unknown) => e);

    expect(err).toBeInstanceOf(ImportApiError);
    expect(err).toMatchObject({ status: 422, message: "CSV tidak valid", errors: rows });
  });

  it("an API error without a message gets a default; a non-API error is rethrown as-is", async () => {
    client.post.mockRejectedValueOnce({ status: 500, message: "" });
    await expect(confirmImport("atms", file)).rejects.toMatchObject({
      message: "Permintaan gagal",
      status: 500,
    });

    const boom = new TypeError("Failed to fetch");
    client.post.mockRejectedValueOnce(boom);
    await expect(confirmImport("atms", file)).rejects.toBe(boom);
  });
});
