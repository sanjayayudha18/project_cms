// Feature: frontend-consolidation, Property 1: Static JSON Data Integrity
// **Validates: Requirements 1.6, 12.2**

import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import * as fc from "fast-check";
import { describe, expect, it } from "vitest";

const REQUIRED_JSON_FILES = [
  "atms.json",
  "attention-items.json",
  "cit-orders.json",
  "dashboard-kpi.json",
  "dsr.json",
  "forecast.json",
  "invoices.json",
  "reconciliation-exceptions.json",
  "replenishment-schedules.json",
  "vendors.json",
] as const;

const SOURCE_DATA_DIR = resolve(__dirname, "../../../../frontend/src/data");
const TARGET_DATA_DIR = resolve(__dirname, "../../data");

describe("Property 1: Static JSON Data Integrity", () => {
  it("each static JSON file in Target_App is byte-for-byte identical to Source_App's copy", () => {
    const fileArbitrary = fc.constantFrom(...REQUIRED_JSON_FILES);

    fc.assert(
      fc.property(fileArbitrary, (fileName) => {
        const sourcePath = resolve(SOURCE_DATA_DIR, fileName);
        const targetPath = resolve(TARGET_DATA_DIR, fileName);

        const sourceContent = readFileSync(sourcePath);
        const targetContent = readFileSync(targetPath);

        expect(
          sourceContent.equals(targetContent),
          `File "${fileName}" in Target_App is not byte-for-byte identical to Source_App`,
        ).toBe(true);
      }),
      { numRuns: 100 },
    );
  });

  it("all 10 required files exist in both Source_App and Target_App", () => {
    for (const fileName of REQUIRED_JSON_FILES) {
      const sourcePath = resolve(SOURCE_DATA_DIR, fileName);
      const targetPath = resolve(TARGET_DATA_DIR, fileName);

      expect(() => readFileSync(sourcePath)).not.toThrow();
      expect(() => readFileSync(targetPath)).not.toThrow();
    }
  });
});
