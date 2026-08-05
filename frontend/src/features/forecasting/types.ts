// ─── DSR Upload Types ──────────────────────────────────────────────────────────

export interface DSRRow {
  rowNumber: number;
  terminalId: string;
  vaultBalance: number;
  vendorFillPlan: number;
  reconciliationResult: string;
  shortageClaim: number;
}

export interface ParsedDSR {
  headers: string[];
  rows: DSRRow[];
  totalRows: number;
  filename: string;
  fileSize: number;
}

export const REQUIRED_DSR_COLUMNS = [
  "terminal_id",
  "vault_balance",
  "vendor_fill_plan",
  "reconciliation_result",
  "shortage_claim",
] as const;

export type RequiredDSRColumn = (typeof REQUIRED_DSR_COLUMNS)[number];

// ─── Validation Types ─────────────────────────────────────────────────────────

export interface FileValidationResult {
  valid: boolean;
  error?: string;
}

export interface ColumnValidationResult {
  valid: boolean;
  missingColumns: string[];
  unexpectedColumns: string[];
}

// ─── Upload Flow Types ────────────────────────────────────────────────────────

export type DSRUploadStep = "idle" | "preview" | "submitting" | "success" | "error";

export interface DSRUploadRequest {
  rows: DSRRow[];
  filename: string;
  uploadDate: string;
}

export interface DSRUploadResponse {
  id: string;
  timestamp: string;
  rowCount: number;
  status: "accepted" | "rejected";
  errors?: ValidationError[];
}

export interface ValidationError {
  row: number;
  field: string;
  message: string;
}

export interface DSRUploadRecord {
  id: string;
  date: string;
  filename: string;
  rowCount: number;
  status: "accepted" | "rejected" | "processing";
}
