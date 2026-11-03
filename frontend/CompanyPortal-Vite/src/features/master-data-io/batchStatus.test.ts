import * as fc from "fast-check";
import { describe, expect, it } from "vitest";
import { BATCH_STATUS, BATCH_STATUS_CODES, canRetry } from "./batchStatus";

describe("batchStatus", () => {
  // Feature: master-data-import-revamp, Property 5: status mapping is total, retry only for approved
  it("maps every status to one label + icon, and canRetry is true only for approved", () => {
    fc.assert(
      fc.property(fc.constantFrom(...BATCH_STATUS_CODES), (status) => {
        const view = BATCH_STATUS[status];
        expect(view.label.length).toBeGreaterThan(0);
        expect(view.icon).toBeTruthy();
        expect(canRetry(status)).toBe(status === "approved");
      }),
    );
  });

  it("gives each status a distinct label", () => {
    const labels = BATCH_STATUS_CODES.map((s) => BATCH_STATUS[s].label);
    expect(new Set(labels).size).toBe(BATCH_STATUS_CODES.length);
  });
});
