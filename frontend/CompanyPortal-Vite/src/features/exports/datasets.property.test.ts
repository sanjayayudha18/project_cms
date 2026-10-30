import type { DbRole } from "@/lib/auth/store";
import fc from "fast-check";
import { beforeEach, describe, expect, it, vi } from "vitest";

const client = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }));
vi.mock("@/lib/api", () => ({ api: client }));
vi.mock("@/lib/download", () => ({ saveBlobResponse: vi.fn() }));

import { masterPath, previewExport, requestExport } from "./api";
import {
  type DatasetId,
  EXPORT_DATASETS,
  type MasterDataset,
  type PreviewFilter,
  datasetsForRole,
  parseFilterSearch,
  toParams,
  toSearch,
} from "./datasets";
import type { PriceLevel, StatusFilter } from "./types";

/** Runs per property (fast-check). */
const NUM_RUNS = 100;

const MASTER = EXPORT_DATASETS.filter((d): d is MasterDataset => d.group === "master");
const masterId = fc.constantFrom(...MASTER.map((d) => d.id));
const byId = (id: DatasetId) => MASTER.find((d) => d.id === id) as MasterDataset;

const status = fc.constantFrom<StatusFilter>("active", "disabled", "all");
const priceLevel = fc.constantFrom<PriceLevel>("all", "vendor", "branch", "atm");
const posId = fc.integer({ min: 1, max: 1_000_000 });

/** Any filter, including ones a dataset would not accept (branch without vendor, ...). */
const anyFilter = fc.record(
  { vendorId: posId, branchId: posId, status, priceLevel },
  { requiredKeys: ["status"] },
) as fc.Arbitrary<PreviewFilter>;

/** A filter valid for the dataset: only the fields it offers, branch only with a vendor. */
const validFilterFor = (id: DatasetId): fc.Arbitrary<PreviewFilter> => {
  const ds = byId(id);
  return fc
    .record({
      vendorId: fc.option(posId, { nil: undefined }),
      branchId: fc.option(posId, { nil: undefined }),
      status,
      priceLevel,
    })
    .map((r) => {
      const f: PreviewFilter = { status: r.status };
      if (ds.filters.vendor !== "none" && r.vendorId !== undefined) {
        f.vendorId = r.vendorId;
        if (ds.filters.branch && r.branchId !== undefined) f.branchId = r.branchId;
      }
      if (ds.filters.priceLevel) f.priceLevel = r.priceLevel;
      return f;
    });
};

describe("Property 6: one filter mapping for preview and export", () => {
  beforeEach(() => {
    client.get.mockReset();
    client.post.mockReset();
    client.get.mockResolvedValue({ data: { success: true, data: {} } });
    client.post.mockResolvedValue({ status: 202, data: { success: true, data: { id: 1 } } });
  });

  it("is null iff a required filter is missing or a branch has no vendor", () => {
    fc.assert(
      fc.property(masterId, anyFilter, (id, f) => {
        const missing =
          (byId(id).filters.vendor === "required" && f.vendorId === undefined) ||
          (f.branchId !== undefined && f.vendorId === undefined);
        expect(toParams(id, f) === null).toBe(missing);
      }),
      { numRuns: NUM_RUNS },
    );
  });

  it("maps kind/entity as in the Req 3.1 table", () => {
    fc.assert(
      fc.property(masterId, anyFilter, (id, f) => {
        const p = toParams(id, f);
        if (p === null) return;
        const { vendorId, branchId } = f;
        let want: object;
        switch (id) {
          case "vendors":
          case "atms":
          case "atm-assignments":
            want = { kind: "master_data_global", entity: id };
            break;
          case "vendor-branches":
            want =
              vendorId === undefined
                ? { kind: "master_data_global", entity: "vendor-branches" }
                : { kind: "vendor", vendorId, entity: "branches" };
            break;
          case "vendor-vaults":
          case "vendor-pics": {
            const entity = id === "vendor-vaults" ? "vaults" : "pics";
            if (vendorId === undefined) want = { kind: "master_data_global", entity: id };
            else if (branchId === undefined) want = { kind: "vendor", vendorId, entity };
            else want = { kind: "cabang", vendorId, branchId, entity };
            break;
          }
          case "dsr-location-maps":
            want =
              branchId === undefined
                ? { kind: "vendor", vendorId, entity: "dsr-location-maps" }
                : { kind: "cabang", vendorId, branchId, entity: "dsr-location-maps" };
            break;
          case "package-prices":
            want = { kind: "harga_paket", vendorId, branchId };
            break;
        }
        expect(p).toMatchObject({ ...want, status: f.status });
      }),
      { numRuns: NUM_RUNS },
    );
  });

  it("preview path = request path without /preview and without after/limit", async () => {
    await fc.assert(
      fc.asyncProperty(
        masterId,
        anyFilter,
        fc.option(posId, { nil: undefined }),
        fc.integer({ min: 1, max: 100 }),
        async (id, f, after, limit) => {
          const p = toParams(id, f);
          if (p === null) return;
          client.get.mockClear();
          client.post.mockClear();
          const path = masterPath(p);
          expect(path).not.toContain("/preview");
          expect(path).not.toMatch(/[?&](after|limit)=/);

          await requestExport(p);
          expect(client.post).toHaveBeenCalledWith(`/exports${path}`);

          await previewExport(p, after, limit);
          const url = client.get.mock.calls[0][0] as string;
          const [base, query = ""] = url.split("?");
          const [reqBase, reqQuery = ""] = path.split("?");
          expect(base).toBe(`/exports/preview${reqBase}`);
          const sent = new URLSearchParams(query);
          expect(sent.get("limit")).toBe(String(limit));
          expect(sent.get("after")).toBe(after === undefined ? null : String(after));
          sent.delete("after");
          sent.delete("limit");
          expect(sent.toString()).toBe(new URLSearchParams(reqQuery).toString());
        },
      ),
      { numRuns: NUM_RUNS },
    );
  });
});

describe("Property 7: URL filter round trip", () => {
  it("parseFilterSearch is valid for arbitrary input on the known keys", () => {
    fc.assert(
      fc.property(
        masterId,
        fc.record({
          status: fc.anything(),
          vendorId: fc.anything(),
          branchId: fc.anything(),
          priceLevel: fc.anything(),
        }),
        fc.dictionary(fc.string(), fc.anything()),
        (id, known, noise) => {
          const f = parseFilterSearch(id, { ...noise, ...known });
          const ds = byId(id);
          expect(["active", "disabled", "all"]).toContain(f.status);
          for (const v of [f.vendorId, f.branchId]) {
            if (v !== undefined) expect(Number.isInteger(v) && v > 0).toBe(true);
          }
          if (f.branchId !== undefined) expect(f.vendorId).toBeDefined();
          if (ds.filters.vendor === "none") expect(f.vendorId).toBeUndefined();
          if (!ds.filters.branch) expect(f.branchId).toBeUndefined();
          if (ds.filters.priceLevel) {
            expect(["all", "vendor", "branch", "atm"]).toContain(f.priceLevel);
          } else {
            expect(f.priceLevel).toBeUndefined();
          }
        },
      ),
      { numRuns: NUM_RUNS },
    );
  });

  it("parseFilterSearch(toSearch(f)) = f for every valid filter", () => {
    fc.assert(
      fc.property(
        masterId.chain((id) => fc.tuple(fc.constant(id), validFilterFor(id))),
        ([id, f]) => {
          expect(parseFilterSearch(id, toSearch(f))).toEqual(f);
        },
      ),
      { numRuns: NUM_RUNS },
    );
  });
});

describe("Property 8: catalog follows export roles", () => {
  const ALL_ROLES: DbRole[] = [
    "ADMIN",
    "ADMIN_PARAM",
    "APPACCESS",
    "ATM-USER",
    "ATM-SPV",
    "BRANCH-USER",
    "BRANCH-SPV",
    "BRANCH-ATM-USER",
    "BRANCH-ATM-SPV",
    "VENDOR-USER",
    "VENDOR-ATM-USER",
    "VENDOR-VAULT-USER",
    "ACM-USER",
    "ACM-SPV",
  ];
  // Explicit copy of backend-exq KindRoles (kinds.go) and pkg/reports viewer roles.
  const EXPECTED: Record<string, string[]> = {
    vendors: ["ADMIN", "ADMIN_PARAM"],
    "vendor-branches": ["ADMIN", "ADMIN_PARAM"],
    "vendor-vaults": ["ADMIN", "ADMIN_PARAM"],
    "vendor-pics": ["ADMIN", "ADMIN_PARAM"],
    "dsr-location-maps": ["ADMIN", "ADMIN_PARAM"],
    "package-prices": ["ADMIN", "ADMIN_PARAM"],
    atms: ["ADMIN", "ADMIN_PARAM"],
    "atm-assignments": ["ADMIN", "ADMIN_PARAM"],
    "dsr-late-report": ["ATM-USER", "ATM-SPV", "BRANCH-ATM-SPV", "ADMIN"],
    replenishment: ["ADMIN", "ATM-USER", "ATM-SPV", "BRANCH-ATM-USER", "BRANCH-ATM-SPV"],
  };

  it("each dataset's roles equal the explicit copy of KindRoles", () => {
    expect(EXPORT_DATASETS.map((d) => d.id).sort()).toEqual(Object.keys(EXPECTED).sort());
    for (const d of EXPORT_DATASETS) {
      expect([...d.roles].sort()).toEqual([...EXPECTED[d.id]].sort());
    }
  });

  it("datasetsForRole contains a dataset iff the role is in its roles", () => {
    fc.assert(
      fc.property(fc.constantFrom(...ALL_ROLES), (r) => {
        const got = datasetsForRole(r).map((d) => d.id);
        const want = Object.entries(EXPECTED)
          .filter(([, roles]) => roles.includes(r))
          .map(([id]) => id);
        expect([...got].sort()).toEqual([...want].sort());
      }),
      { numRuns: NUM_RUNS },
    );
  });

  it("ADMIN_PARAM never sees report datasets", () => {
    expect(datasetsForRole("ADMIN_PARAM").some((d) => d.group === "report")).toBe(false);
  });
});
