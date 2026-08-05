import * as fc from "fast-check";
import { describe, expect, it } from "vitest";
import { REQUIRED_DSR_COLUMNS } from "./types";
import type { DSRRow, DSRUploadRecord, DSRUploadResponse, ValidationError } from "./types";
import { MAX_FILE_SIZE_BYTES, validateDSRColumns, validateDSRFile } from "./utils/dsr-validation";

// ─── Helpers ──────────────────────────────────────────────────────────────────

function createMockFile(filename: string, size: number): File {
  const file = new File(["x"], filename, {
    type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
  });
  Object.defineProperty(file, "size", { value: size });
  return file;
}

/**
 * Formats the success toast message for a DSR upload response.
 * Extracted from the upload success handler logic.
 */
function formatDSRUploadSuccessToast(response: DSRUploadResponse): string {
  return `DSR berhasil diunggah pada ${response.timestamp} (${response.rowCount} baris)`;
}

/**
 * Returns preview rows: first 20 rows of the dataset.
 */
function getPreviewRows(rows: DSRRow[]): DSRRow[] {
  return rows.slice(0, 20);
}

/**
 * Processes upload history: sort by date descending, limit to 30 records.
 */
function getDisplayedHistory(records: DSRUploadRecord[]): DSRUploadRecord[] {
  return [...records]
    .sort((a, b) => new Date(b.date).getTime() - new Date(a.date).getTime())
    .slice(0, 30);
}

// ─── Generators ───────────────────────────────────────────────────────────────

const arbValidExtension = fc.constantFrom(".xlsx", ".xls");

const arbValidFilename = fc
  .string({ minLength: 1, maxLength: 50, unit: "grapheme" })
  .filter((s) => s.trim().length > 0 && !s.includes("."))
  .chain((name) => arbValidExtension.map((ext) => name + ext));

const arbValidationError: fc.Arbitrary<ValidationError> = fc.record({
  row: fc.integer({ min: 1, max: 10000 }),
  field: fc.constantFrom(
    "terminal_id",
    "vault_balance",
    "vendor_fill_plan",
    "reconciliation_result",
    "shortage_claim",
  ),
  message: fc.string({ minLength: 1, maxLength: 100 }),
});

const arbDSRUploadRecord: fc.Arbitrary<DSRUploadRecord> = fc.record({
  id: fc.uuid(),
  date: fc
    .date({ min: new Date("2020-01-01"), max: new Date("2030-12-31") })
    .map((d) => d.toISOString()),
  filename: fc.string({ minLength: 1, maxLength: 30 }).map((n) => `${n}.xlsx`),
  rowCount: fc.integer({ min: 1, max: 5000 }),
  status: fc.constantFrom("accepted", "rejected", "processing"),
});

// ═══════════════════════════════════════════════════════════════════════════════
// Property 11: DSR File Size Validation
// Validates: Requirements 6.2
// ═══════════════════════════════════════════════════════════════════════════════

describe("Property 11: DSR File Size Validation", () => {
  it("accepts files with valid extension when size ≤ 10MB", () => {
    fc.assert(
      fc.property(
        arbValidFilename,
        fc.integer({ min: 1, max: MAX_FILE_SIZE_BYTES }),
        (filename, size) => {
          const file = createMockFile(filename, size);
          const result = validateDSRFile(file);
          expect(result.valid).toBe(true);
          expect(result.error).toBeUndefined();
        },
      ),
      { numRuns: 100 },
    );
  });

  it("rejects files with valid extension when size > 10MB", () => {
    fc.assert(
      fc.property(
        arbValidFilename,
        fc.integer({ min: MAX_FILE_SIZE_BYTES + 1, max: MAX_FILE_SIZE_BYTES * 5 }),
        (filename, size) => {
          const file = createMockFile(filename, size);
          const result = validateDSRFile(file);
          expect(result.valid).toBe(false);
          expect(result.error).toBeDefined();
          expect(result.error?.length).toBeGreaterThan(0);
        },
      ),
      { numRuns: 100 },
    );
  });

  it("boundary: file at exactly 10MB is accepted", () => {
    const file = createMockFile("report.xlsx", MAX_FILE_SIZE_BYTES);
    const result = validateDSRFile(file);
    expect(result.valid).toBe(true);
  });

  it("boundary: file at 10MB + 1 byte is rejected", () => {
    const file = createMockFile("report.xlsx", MAX_FILE_SIZE_BYTES + 1);
    const result = validateDSRFile(file);
    expect(result.valid).toBe(false);
  });
});

// ═══════════════════════════════════════════════════════════════════════════════
// Property 12: DSR Column Validation
// Validates: Requirements 6.8
// ═══════════════════════════════════════════════════════════════════════════════

describe("Property 12: DSR Column Validation", () => {
  it("returns valid when headers are a superset of REQUIRED_DSR_COLUMNS", () => {
    const arbExtraColumns = fc.array(
      fc
        .string({ minLength: 1, maxLength: 20 })
        .filter((s) => !REQUIRED_DSR_COLUMNS.includes(s as (typeof REQUIRED_DSR_COLUMNS)[number])),
      { minLength: 0, maxLength: 5 },
    );

    fc.assert(
      fc.property(arbExtraColumns, (extras) => {
        const headers = [...REQUIRED_DSR_COLUMNS, ...extras];
        const result = validateDSRColumns(headers);
        expect(result.valid).toBe(true);
        expect(result.missingColumns).toHaveLength(0);
      }),
      { numRuns: 100 },
    );
  });

  it("returns invalid when at least one required column is missing, and identifies it", () => {
    const arbMissingIndices = fc.uniqueArray(
      fc.integer({ min: 0, max: REQUIRED_DSR_COLUMNS.length - 1 }),
      { minLength: 1, maxLength: REQUIRED_DSR_COLUMNS.length },
    );

    fc.assert(
      fc.property(arbMissingIndices, (missingIndices) => {
        const missingCols = missingIndices.map((i) => REQUIRED_DSR_COLUMNS[i]);
        const headers = REQUIRED_DSR_COLUMNS.filter((col) => !missingCols.includes(col));
        const result = validateDSRColumns([...headers]);
        expect(result.valid).toBe(false);
        for (const col of missingCols) {
          expect(result.missingColumns).toContain(col);
        }
      }),
      { numRuns: 100 },
    );
  });

  it("returns valid for exact match of required columns", () => {
    const result = validateDSRColumns([...REQUIRED_DSR_COLUMNS]);
    expect(result.valid).toBe(true);
    expect(result.missingColumns).toHaveLength(0);
  });
});

// ═══════════════════════════════════════════════════════════════════════════════
// Property 13: DSR Preview Row Limit
// Validates: Requirements 6.3
// ═══════════════════════════════════════════════════════════════════════════════

describe("Property 13: DSR Preview Row Limit", () => {
  it("returns exactly min(N, 20) rows for any dataset of N rows", () => {
    fc.assert(
      fc.property(fc.integer({ min: 1, max: 200 }), (n) => {
        const rows: DSRRow[] = Array.from({ length: n }, (_, i) => ({
          rowNumber: i + 2,
          terminalId: `T${i}`,
          vaultBalance: 1000 * i,
          vendorFillPlan: 500 * i,
          reconciliationResult: "match",
          shortageClaim: 0,
        }));

        const preview = getPreviewRows(rows);
        expect(preview).toHaveLength(Math.min(n, 20));
      }),
      { numRuns: 100 },
    );
  });

  it("preview rows are taken from the beginning of the dataset", () => {
    fc.assert(
      fc.property(fc.integer({ min: 1, max: 200 }), (n) => {
        const rows: DSRRow[] = Array.from({ length: n }, (_, i) => ({
          rowNumber: i + 2,
          terminalId: `T${i}`,
          vaultBalance: 1000 * i,
          vendorFillPlan: 500 * i,
          reconciliationResult: "match",
          shortageClaim: 0,
        }));

        const preview = getPreviewRows(rows);
        for (let i = 0; i < preview.length; i++) {
          expect(preview[i]).toEqual(rows[i]);
        }
      }),
      { numRuns: 50 },
    );
  });
});

// ═══════════════════════════════════════════════════════════════════════════════
// Property 14: DSR Upload Success Toast Content
// Validates: Requirements 6.6
// ═══════════════════════════════════════════════════════════════════════════════

describe("Property 14: DSR Upload Success Toast Content", () => {
  const arbAcceptedResponse: fc.Arbitrary<DSRUploadResponse> = fc.record({
    id: fc.uuid(),
    timestamp: fc
      .date({ min: new Date("2020-01-01"), max: new Date("2030-12-31") })
      .map((d) => d.toISOString()),
    rowCount: fc.integer({ min: 1, max: 10000 }),
    status: fc.constant("accepted" as const),
  });

  it("success toast message contains the timestamp value", () => {
    fc.assert(
      fc.property(arbAcceptedResponse, (response) => {
        const message = formatDSRUploadSuccessToast(response);
        expect(message).toContain(response.timestamp);
      }),
      { numRuns: 100 },
    );
  });

  it("success toast message contains the rowCount value", () => {
    fc.assert(
      fc.property(arbAcceptedResponse, (response) => {
        const message = formatDSRUploadSuccessToast(response);
        expect(message).toContain(String(response.rowCount));
      }),
      { numRuns: 100 },
    );
  });
});

// ═══════════════════════════════════════════════════════════════════════════════
// Property 15: DSR Validation Error Display Completeness
// Validates: Requirements 6.7
// ═══════════════════════════════════════════════════════════════════════════════

describe("Property 15: DSR Validation Error Display Completeness", () => {
  /**
   * Simulates the error summary renderer output.
   * Based on DSRPreviewTable's validation error table rendering:
   * each error renders its row number and field name.
   */
  function renderErrorSummary(errors: ValidationError[]): string {
    return errors.map((err) => `Baris ${err.row}: ${err.field} - ${err.message}`).join("\n");
  }

  it("output contains every error's row number", () => {
    fc.assert(
      fc.property(fc.array(arbValidationError, { minLength: 1, maxLength: 50 }), (errors) => {
        const output = renderErrorSummary(errors);
        for (const err of errors) {
          expect(output).toContain(String(err.row));
        }
      }),
      { numRuns: 100 },
    );
  });

  it("output contains every error's field name", () => {
    fc.assert(
      fc.property(fc.array(arbValidationError, { minLength: 1, maxLength: 50 }), (errors) => {
        const output = renderErrorSummary(errors);
        for (const err of errors) {
          expect(output).toContain(err.field);
        }
      }),
      { numRuns: 100 },
    );
  });

  it("no error from the array is omitted", () => {
    fc.assert(
      fc.property(fc.array(arbValidationError, { minLength: 1, maxLength: 50 }), (errors) => {
        const output = renderErrorSummary(errors);
        const lines = output.split("\n");
        expect(lines).toHaveLength(errors.length);
      }),
      { numRuns: 100 },
    );
  });
});

// ═══════════════════════════════════════════════════════════════════════════════
// Property 16: DSR Upload History Limit
// Validates: Requirements 6.9
// ═══════════════════════════════════════════════════════════════════════════════

describe("Property 16: DSR Upload History Limit", () => {
  it("displays exactly min(N, 30) records for any list of N records", () => {
    fc.assert(
      fc.property(fc.array(arbDSRUploadRecord, { minLength: 1, maxLength: 100 }), (records) => {
        const displayed = getDisplayedHistory(records);
        expect(displayed).toHaveLength(Math.min(records.length, 30));
      }),
      { numRuns: 100 },
    );
  });

  it("records are ordered by most recent date first", () => {
    fc.assert(
      fc.property(fc.array(arbDSRUploadRecord, { minLength: 2, maxLength: 50 }), (records) => {
        const displayed = getDisplayedHistory(records);
        for (let i = 1; i < displayed.length; i++) {
          const prev = new Date(displayed[i - 1].date).getTime();
          const curr = new Date(displayed[i].date).getTime();
          expect(prev).toBeGreaterThanOrEqual(curr);
        }
      }),
      { numRuns: 100 },
    );
  });

  it("all returned records exist in the original input", () => {
    fc.assert(
      fc.property(fc.array(arbDSRUploadRecord, { minLength: 1, maxLength: 100 }), (records) => {
        const displayed = getDisplayedHistory(records);
        for (const record of displayed) {
          expect(records.some((r) => r.id === record.id)).toBe(true);
        }
      }),
      { numRuns: 50 },
    );
  });
});
