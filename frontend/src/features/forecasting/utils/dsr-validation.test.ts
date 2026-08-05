import { describe, expect, it } from "vitest";
import {
  MAX_FILE_SIZE_BYTES,
  getFileExtension,
  normalizeColumnName,
  validateDSRColumns,
  validateDSRFile,
} from "./dsr-validation";

// ─── Helper ──────────────────────────────────────────────────────────────────

function createMockFile(name: string, size: number, type = ""): File {
  const content = new Uint8Array(size);
  return new File([content], name, { type });
}

// ─── validateDSRFile ──────────────────────────────────────────────────────────

describe("validateDSRFile", () => {
  it("accepts a valid .xlsx file under 10MB", () => {
    const file = createMockFile("report.xlsx", 1024);
    const result = validateDSRFile(file);
    expect(result.valid).toBe(true);
    expect(result.error).toBeUndefined();
  });

  it("accepts a valid .xls file under 10MB", () => {
    const file = createMockFile("report.xls", 5_000_000);
    const result = validateDSRFile(file);
    expect(result.valid).toBe(true);
  });

  it("rejects a file with unsupported extension", () => {
    const file = createMockFile("report.csv", 1024);
    const result = validateDSRFile(file);
    expect(result.valid).toBe(false);
    expect(result.error).toContain(".csv");
    expect(result.error).toContain(".xlsx");
  });

  it("rejects a file without extension", () => {
    const file = createMockFile("report", 1024);
    const result = validateDSRFile(file);
    expect(result.valid).toBe(false);
    expect(result.error).toContain("tanpa ekstensi");
  });

  it("rejects a file exceeding 10MB", () => {
    const file = createMockFile("report.xlsx", MAX_FILE_SIZE_BYTES + 1);
    const result = validateDSRFile(file);
    expect(result.valid).toBe(false);
    expect(result.error).toContain("10 MB");
  });

  it("accepts a file exactly at 10MB", () => {
    const file = createMockFile("report.xlsx", MAX_FILE_SIZE_BYTES);
    const result = validateDSRFile(file);
    expect(result.valid).toBe(true);
  });

  it("rejects an empty file (0 bytes)", () => {
    const file = createMockFile("report.xlsx", 0);
    const result = validateDSRFile(file);
    expect(result.valid).toBe(false);
    expect(result.error).toContain("kosong");
  });

  it("handles uppercase extension correctly", () => {
    const file = createMockFile("REPORT.XLSX", 1024);
    const result = validateDSRFile(file);
    expect(result.valid).toBe(true);
  });
});

// ─── validateDSRColumns ───────────────────────────────────────────────────────

describe("validateDSRColumns", () => {
  it("passes when all required columns are present", () => {
    const headers = [
      "terminal_id",
      "vault_balance",
      "vendor_fill_plan",
      "reconciliation_result",
      "shortage_claim",
    ];
    const result = validateDSRColumns(headers);
    expect(result.valid).toBe(true);
    expect(result.missingColumns).toHaveLength(0);
    expect(result.unexpectedColumns).toHaveLength(0);
  });

  it("passes with extra columns (reports them as unexpected)", () => {
    const headers = [
      "terminal_id",
      "vault_balance",
      "vendor_fill_plan",
      "reconciliation_result",
      "shortage_claim",
      "extra_column",
    ];
    const result = validateDSRColumns(headers);
    expect(result.valid).toBe(true);
    expect(result.unexpectedColumns).toContain("extra_column");
  });

  it("fails when a required column is missing", () => {
    const headers = [
      "terminal_id",
      "vault_balance",
      "vendor_fill_plan",
      "reconciliation_result",
      // missing 'shortage_claim'
    ];
    const result = validateDSRColumns(headers);
    expect(result.valid).toBe(false);
    expect(result.missingColumns).toContain("shortage_claim");
  });

  it("normalizes column names (spaces to underscores, case insensitive)", () => {
    const headers = [
      "Terminal ID",
      "Vault Balance",
      "Vendor Fill Plan",
      "Reconciliation Result",
      "Shortage Claim",
    ];
    const result = validateDSRColumns(headers);
    expect(result.valid).toBe(true);
  });

  it("normalizes column names with hyphens", () => {
    const headers = [
      "terminal-id",
      "vault-balance",
      "vendor-fill-plan",
      "reconciliation-result",
      "shortage-claim",
    ];
    const result = validateDSRColumns(headers);
    expect(result.valid).toBe(true);
  });

  it("reports multiple missing columns", () => {
    const headers = ["terminal_id"];
    const result = validateDSRColumns(headers);
    expect(result.valid).toBe(false);
    expect(result.missingColumns).toHaveLength(4);
  });

  it("handles empty headers array", () => {
    const result = validateDSRColumns([]);
    expect(result.valid).toBe(false);
    expect(result.missingColumns).toHaveLength(5);
  });
});

// ─── getFileExtension ─────────────────────────────────────────────────────────

describe("getFileExtension", () => {
  it("extracts .xlsx extension", () => {
    expect(getFileExtension("file.xlsx")).toBe(".xlsx");
  });

  it("extracts extension from filename with multiple dots", () => {
    expect(getFileExtension("my.report.2024.xlsx")).toBe(".xlsx");
  });

  it("returns empty string for no extension", () => {
    expect(getFileExtension("filename")).toBe("");
  });

  it("returns empty string for trailing dot", () => {
    expect(getFileExtension("filename.")).toBe("");
  });

  it("lowercases the extension", () => {
    expect(getFileExtension("FILE.XLSX")).toBe(".xlsx");
  });
});

// ─── normalizeColumnName ──────────────────────────────────────────────────────

describe("normalizeColumnName", () => {
  it("trims whitespace", () => {
    expect(normalizeColumnName("  terminal_id  ")).toBe("terminal_id");
  });

  it("lowercases", () => {
    expect(normalizeColumnName("Terminal_ID")).toBe("terminal_id");
  });

  it("replaces spaces with underscores", () => {
    expect(normalizeColumnName("vault balance")).toBe("vault_balance");
  });

  it("replaces hyphens with underscores", () => {
    expect(normalizeColumnName("vendor-fill-plan")).toBe("vendor_fill_plan");
  });

  it("handles multiple consecutive spaces", () => {
    expect(normalizeColumnName("shortage   claim")).toBe("shortage_claim");
  });
});
