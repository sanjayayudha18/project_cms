import { beforeEach, describe, expect, it, vi } from "vitest";

// fe-coverage C3: master-data CSV import client. The HTTP client is mocked.
const client = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }));
vi.mock("@/lib/api", () => ({ api: client }));

import { ImportApiError, confirmImport, dryRunImport } from "./api";

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
