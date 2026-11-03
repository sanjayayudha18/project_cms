import fc from "fast-check";
import { beforeEach, describe, expect, it } from "vitest";
import { claimAutoDownload, resetAutoDownloadClaims } from "./autoDownload";

// Feature: vendor-data-export, Property 18 (pure guard): a job id is claimed at most once per tab.
describe("claimAutoDownload property", () => {
  beforeEach(resetAutoDownloadClaims);

  it("returns true exactly once per distinct id for any id sequence", () => {
    fc.assert(
      fc.property(fc.array(fc.integer({ min: 1, max: 20 }), { maxLength: 100 }), (ids) => {
        resetAutoDownloadClaims();
        const wins = new Map<number, number>();
        for (const id of ids) if (claimAutoDownload(id)) wins.set(id, (wins.get(id) ?? 0) + 1);
        expect([...wins.keys()].sort()).toEqual([...new Set(ids)].sort());
        for (const n of wins.values()) expect(n).toBe(1);
      }),
      { numRuns: 100 },
    );
  });
});
