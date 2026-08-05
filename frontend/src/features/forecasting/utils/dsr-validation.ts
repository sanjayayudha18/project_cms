import {
  type ColumnValidationResult,
  type FileValidationResult,
  REQUIRED_DSR_COLUMNS,
} from "../types";

// ─── Constants ────────────────────────────────────────────────────────────────

const MAX_FILE_SIZE_BYTES = 10_485_760; // 10MB
const ALLOWED_EXTENSIONS = [".xlsx", ".xls"] as const;

// ─── File Validation ──────────────────────────────────────────────────────────

/**
 * Validates file type and size before parsing.
 * Pure function — no side effects.
 */
export function validateDSRFile(file: File): FileValidationResult {
  const extension = getFileExtension(file.name);

  if (!isAllowedExtension(extension)) {
    return {
      valid: false,
      error: `Format file tidak didukung: "${extension || "(tanpa ekstensi)"}". Hanya file .xlsx dan .xls yang diterima.`,
    };
  }

  if (file.size > MAX_FILE_SIZE_BYTES) {
    const sizeMB = (file.size / (1024 * 1024)).toFixed(1);
    return {
      valid: false,
      error: `Ukuran file (${sizeMB} MB) melebihi batas maksimum 10 MB. Silakan kompres atau pecah file Anda.`,
    };
  }

  if (file.size === 0) {
    return {
      valid: false,
      error: "File kosong. Silakan pilih file DSR yang valid.",
    };
  }

  return { valid: true };
}

// ─── Column Validation ────────────────────────────────────────────────────────

/**
 * Validates that parsed column headers contain all required DSR columns.
 * Identifies both missing required columns and unexpected extra columns.
 * Pure function — no side effects.
 */
export function validateDSRColumns(headers: string[]): ColumnValidationResult {
  const normalizedHeaders = headers.map(normalizeColumnName);

  const missingColumns = REQUIRED_DSR_COLUMNS.filter((col) => !normalizedHeaders.includes(col));

  const unexpectedColumns = normalizedHeaders.filter(
    (header) => !REQUIRED_DSR_COLUMNS.includes(header as (typeof REQUIRED_DSR_COLUMNS)[number]),
  );

  return {
    valid: missingColumns.length === 0,
    missingColumns: [...missingColumns],
    unexpectedColumns,
  };
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

/**
 * Extracts file extension from filename (lowercased, including the dot).
 */
export function getFileExtension(filename: string): string {
  const lastDot = filename.lastIndexOf(".");
  if (lastDot === -1 || lastDot === filename.length - 1) return "";
  return filename.slice(lastDot).toLowerCase();
}

/**
 * Checks if extension is in the allowed list.
 */
function isAllowedExtension(extension: string): boolean {
  return (ALLOWED_EXTENSIONS as readonly string[]).includes(extension);
}

/**
 * Normalizes a column header to snake_case for comparison.
 * Trims whitespace, lowercases, replaces spaces/hyphens with underscores.
 */
export function normalizeColumnName(name: string): string {
  return name
    .trim()
    .toLowerCase()
    .replace(/[\s-]+/g, "_");
}

export { MAX_FILE_SIZE_BYTES, ALLOWED_EXTENSIONS };
