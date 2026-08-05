import * as XLSX from "xlsx";
import type { DSRRow, ParsedDSR } from "../types";
import { normalizeColumnName } from "./dsr-validation";

// ─── Excel Parsing ────────────────────────────────────────────────────────────

/**
 * Parses a DSR Excel file into structured data.
 * Reads the first sheet only.
 * Returns parsed headers and rows with normalized column mapping.
 */
export function parseDSRExcel(buffer: ArrayBuffer, filename: string, fileSize: number): ParsedDSR {
  const workbook = XLSX.read(buffer, { type: "array" });

  const firstSheetName = workbook.SheetNames[0];
  if (!firstSheetName) {
    return {
      headers: [],
      rows: [],
      totalRows: 0,
      filename,
      fileSize,
    };
  }

  const sheet = workbook.Sheets[firstSheetName];
  if (!sheet) {
    return {
      headers: [],
      rows: [],
      totalRows: 0,
      filename,
      fileSize,
    };
  }

  // Convert to JSON with header row
  const rawData = XLSX.utils.sheet_to_json<Record<string, unknown>>(sheet, {
    defval: "",
  });

  if (rawData.length === 0) {
    return {
      headers: [],
      rows: [],
      totalRows: 0,
      filename,
      fileSize,
    };
  }

  // Extract headers from first row keys
  const headers = Object.keys(rawData[0] ?? {});

  // Build column index map (normalized name → original header)
  const columnMap = new Map<string, string>();
  for (const header of headers) {
    columnMap.set(normalizeColumnName(header), header);
  }

  // Map rows to DSRRow structure
  const rows: DSRRow[] = rawData.map((row, index) => ({
    rowNumber: index + 2, // +2 because row 1 is header, data starts at row 2
    terminalId: String(row[columnMap.get("terminal_id") ?? ""] ?? ""),
    vaultBalance: toNumber(row[columnMap.get("vault_balance") ?? ""]),
    vendorFillPlan: toNumber(row[columnMap.get("vendor_fill_plan") ?? ""]),
    reconciliationResult: String(row[columnMap.get("reconciliation_result") ?? ""] ?? ""),
    shortageClaim: toNumber(row[columnMap.get("shortage_claim") ?? ""]),
  }));

  return {
    headers,
    rows,
    totalRows: rows.length,
    filename,
    fileSize,
  };
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

function toNumber(value: unknown): number {
  if (typeof value === "number") return value;
  if (typeof value === "string") {
    const parsed = Number.parseFloat(value.replace(/[,\s]/g, ""));
    return Number.isNaN(parsed) ? 0 : parsed;
  }
  return 0;
}
