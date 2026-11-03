import { beforeEach, describe, expect, it, vi } from "vitest";

// fe-coverage C3: master-data CSV import client. The HTTP client is mocked.
const client = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }));
vi.mock("@/lib/api", () => ({ api: client }));

import {
  ImportApiError,
  type ImportBatchDetail,
  type ImportBatchSummary,
  type ImportPreview,
  confirmImport,
  dryRunImport,
  getImportBatch,
  listImportBatches,
  retryApplyImport,
} from "./api";

beforeEach(() => {
  client.get.mockReset();
  client.post.mockReset();
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

// master-data-import-revamp 10.1: payloads below are written in the exact JSON the Go
// handlers emit (ImportPreview / history design), so a renamed field fails type-check.
describe("import preview contract", () => {
  it("carries the per-op counts and Change_Preview rows of the dry-run", () => {
    const preview: ImportPreview = {
      entity: "vendors",
      total_rows: 3,
      valid_rows: 3,
      creates: 1,
      updates: 2,
      unchanged: 0,
      errors: [],
      errors_truncated: false,
      data_updates: 1,
      enables: 0,
      disables: 1,
      changes: [
        { row: 2, op: "create", key: "V9", fields: [{ field: "code", before: "", after: "V9" }] },
        {
          row: 3,
          op: "disable",
          key: "V1",
          fields: [{ field: "is_active", before: "true", after: "false" }],
        },
      ],
    };

    expect(preview.updates).toBe(preview.data_updates + preview.enables + preview.disables);
    expect(preview.changes.map((c) => c.op)).toEqual(["create", "disable"]);
  });
});

describe("import history client", () => {
  const summary: ImportBatchSummary = {
    id: 12,
    entity: "vendors",
    file_name: null,
    row_count: 35,
    maker_id: 4,
    maker_name: "Siti A.",
    created_at: "2026-11-02T03:10:00Z",
    status: "pending",
    approval_request_id: null,
    error: null,
    decided_at: null,
    decided_by_name: null,
  };

  it("lists batches with only the filters that are set", async () => {
    client.get.mockResolvedValue({ data: { items: [summary], total: 1, page: 2, page_size: 20 } });

    const page = await listImportBatches({ entity: "vendors", mine: true, page: 2 });

    expect(page.items[0]?.file_name).toBeNull();
    expect(client.get).toHaveBeenCalledWith(
      "/admin/master-data/import/batches?entity=vendors&mine=true&page=2",
    );
  });

  it("sends no params for an empty filter and drops mine=false", async () => {
    client.get.mockResolvedValue({ data: { items: [], total: 0, page: 1, page_size: 20 } });

    await listImportBatches();
    await listImportBatches({ mine: false });

    expect(client.get).toHaveBeenNthCalledWith(1, "/admin/master-data/import/batches");
    expect(client.get).toHaveBeenNthCalledWith(2, "/admin/master-data/import/batches");
  });

  it("fetches one batch page with page and page_size", async () => {
    const detail: ImportBatchDetail = {
      ...summary,
      status: "stale",
      counts: { create: 1, update: 0, enable: 0, disable: 1 },
      changes: [],
      total: 2,
      page: 1,
      page_size: 50,
    };
    client.get.mockResolvedValue({ data: detail });

    await expect(getImportBatch(12, 1, 50)).resolves.toBe(detail);
    expect(client.get).toHaveBeenCalledWith(
      "/admin/master-data/import/batches/12?page=1&page_size=50",
    );
  });

  it("a missing batch surfaces the shared client's 404 untouched", async () => {
    const notFound = { status: 404, message: "tidak ditemukan" };
    client.get.mockRejectedValue(notFound);

    await expect(getImportBatch(99)).rejects.toBe(notFound);
  });

  it("retry-apply posts to the approval request and resolves without a body", async () => {
    client.post.mockResolvedValue({ data: { id: 88 } });

    await expect(retryApplyImport(88)).resolves.toBeUndefined();
    expect(client.post).toHaveBeenCalledWith("/approvals/88/retry-apply");
  });
});
