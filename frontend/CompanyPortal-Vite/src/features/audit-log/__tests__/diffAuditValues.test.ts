import * as fc from "fast-check";
import { describe, expect, it } from "vitest";
import { diffAuditValues } from "../lib/diffAuditValues";

/** Property 11: before/after top-level classification. */
describe("diffAuditValues", () => {
  it("classifies added / removed / modified / unchanged keys", () => {
    const rows = diffAuditValues(
      { keep: 1, change: "a", gone: true },
      { keep: 1, change: "b", fresh: [1] },
    );

    expect(rows).toEqual([
      { key: "keep", status: "unchanged", before: 1, after: 1 },
      { key: "change", status: "modified", before: "a", after: "b" },
      { key: "gone", status: "removed", before: true, after: undefined },
      { key: "fresh", status: "added", before: undefined, after: [1] },
    ]);
  });

  it("treats a null before as all-added and a null after as all-removed", () => {
    expect(diffAuditValues(null, { a: 1 }).map((r) => r.status)).toEqual(["added"]);
    expect(diffAuditValues({ a: 1 }, null).map((r) => r.status)).toEqual(["removed"]);
    expect(diffAuditValues(null, null)).toEqual([]);
  });

  it("compares nested values structurally, ignoring key order", () => {
    const rows = diffAuditValues({ cfg: { a: 1, b: [1, 2] } }, { cfg: { b: [1, 2], a: 1 } });
    expect(rows[0]?.status).toBe("unchanged");
    expect(diffAuditValues({ cfg: { a: 1 } }, { cfg: { a: 2 } })[0]?.status).toBe("modified");
  });

  it("an object diffed against itself is entirely unchanged, for any object", () => {
    fc.assert(
      fc.property(fc.dictionary(fc.string(), fc.jsonValue()), (obj) => {
        const parsed = JSON.parse(JSON.stringify(obj)) as Record<string, unknown>;
        for (const row of diffAuditValues(parsed, parsed)) expect(row.status).toBe("unchanged");
      }),
    );
  });
});
