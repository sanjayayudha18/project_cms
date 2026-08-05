import { AlertCircle, CheckCircle2, Loader2 } from "lucide-react";
import type { DSRRow, ValidationError } from "../types";

// ─── Props ────────────────────────────────────────────────────────────────────

interface DSRPreviewTableProps {
  rows: DSRRow[];
  totalRows: number;
  filename: string;
  isSubmitting: boolean;
  validationErrors: ValidationError[];
  onConfirm: () => void;
  onCancel: () => void;
}

// ─── DSR Preview Table ────────────────────────────────────────────────────────

export function DSRPreviewTable({
  rows,
  totalRows,
  filename,
  isSubmitting,
  validationErrors,
  onConfirm,
  onCancel,
}: DSRPreviewTableProps) {
  const previewRows = rows.slice(0, 20);
  const hasErrors = validationErrors.length > 0;

  return (
    <div className="flex flex-col gap-[var(--space-4)]">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex flex-col gap-[var(--space-1)]">
          <h3 className="text-sm font-semibold" style={{ color: "var(--n-900)" }}>
            Pratinjau Data
          </h3>
          <p className="text-xs" style={{ color: "var(--n-500)" }}>
            {filename} — {totalRows} baris
          </p>
        </div>
      </div>

      {/* Table */}
      <div
        className="overflow-x-auto rounded-[var(--radius-lg)] border"
        style={{ borderColor: "var(--n-200)" }}
      >
        <table className="w-full text-left text-sm">
          <thead>
            <tr style={{ backgroundColor: "var(--n-50)" }}>
              <th
                className="px-[var(--space-3)] py-[var(--space-2)] text-xs font-medium uppercase tracking-wider"
                style={{ color: "var(--n-500)" }}
              >
                Baris
              </th>
              <th
                className="px-[var(--space-3)] py-[var(--space-2)] text-xs font-medium uppercase tracking-wider"
                style={{ color: "var(--n-500)" }}
              >
                Terminal ID
              </th>
              <th
                className="px-[var(--space-3)] py-[var(--space-2)] text-right text-xs font-medium uppercase tracking-wider"
                style={{ color: "var(--n-500)" }}
              >
                Saldo Vault
              </th>
              <th
                className="px-[var(--space-3)] py-[var(--space-2)] text-right text-xs font-medium uppercase tracking-wider"
                style={{ color: "var(--n-500)" }}
              >
                Rencana Isi Vendor
              </th>
              <th
                className="px-[var(--space-3)] py-[var(--space-2)] text-xs font-medium uppercase tracking-wider"
                style={{ color: "var(--n-500)" }}
              >
                Hasil Rekonsiliasi
              </th>
              <th
                className="px-[var(--space-3)] py-[var(--space-2)] text-right text-xs font-medium uppercase tracking-wider"
                style={{ color: "var(--n-500)" }}
              >
                Shortage Claim
              </th>
            </tr>
          </thead>
          <tbody>
            {previewRows.map((row) => (
              <tr key={row.rowNumber} className="border-t" style={{ borderColor: "var(--n-100)" }}>
                <td
                  className="px-[var(--space-3)] py-[var(--space-2)] tabular-nums"
                  style={{ color: "var(--n-500)" }}
                >
                  {row.rowNumber}
                </td>
                <td
                  className="px-[var(--space-3)] py-[var(--space-2)]"
                  style={{ color: "var(--n-800)" }}
                >
                  {row.terminalId}
                </td>
                <td
                  className="px-[var(--space-3)] py-[var(--space-2)] text-right tabular-nums"
                  style={{ color: "var(--n-800)" }}
                >
                  {row.vaultBalance.toLocaleString("id-ID")}
                </td>
                <td
                  className="px-[var(--space-3)] py-[var(--space-2)] text-right tabular-nums"
                  style={{ color: "var(--n-800)" }}
                >
                  {row.vendorFillPlan.toLocaleString("id-ID")}
                </td>
                <td
                  className="px-[var(--space-3)] py-[var(--space-2)]"
                  style={{ color: "var(--n-800)" }}
                >
                  {row.reconciliationResult}
                </td>
                <td
                  className="px-[var(--space-3)] py-[var(--space-2)] text-right tabular-nums"
                  style={{ color: "var(--n-800)" }}
                >
                  {row.shortageClaim.toLocaleString("id-ID")}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {totalRows > 20 && (
        <p className="text-xs" style={{ color: "var(--n-500)" }}>
          Menampilkan 20 dari {totalRows} baris.
        </p>
      )}

      {/* Validation Error Summary */}
      {hasErrors && (
        <div
          className="flex flex-col gap-[var(--space-3)] rounded-[var(--radius-md)] p-[var(--space-4)]"
          style={{ backgroundColor: "var(--danger-bg)", color: "var(--danger-fg)" }}
          role="alert"
        >
          <div className="flex items-center gap-[var(--space-2)]">
            <AlertCircle size={18} className="shrink-0" aria-hidden="true" />
            <p className="text-sm font-medium">Kesalahan validasi</p>
          </div>
          <div
            className="overflow-x-auto rounded-[var(--radius-md)] border"
            style={{ borderColor: "var(--danger-solid)" }}
          >
            <table className="w-full text-left text-xs">
              <thead>
                <tr style={{ backgroundColor: "var(--danger-bg)" }}>
                  <th className="px-[var(--space-3)] py-[var(--space-2)] font-medium">Baris</th>
                  <th className="px-[var(--space-3)] py-[var(--space-2)] font-medium">Kolom</th>
                  <th className="px-[var(--space-3)] py-[var(--space-2)] font-medium">Pesan</th>
                </tr>
              </thead>
              <tbody>
                {validationErrors.map((err, idx) => (
                  <tr
                    key={`${err.row}-${err.field}-${idx}`}
                    className="border-t"
                    style={{ borderColor: "var(--danger-solid)", opacity: 0.9 }}
                  >
                    <td className="px-[var(--space-3)] py-[var(--space-2)] tabular-nums">
                      {err.row}
                    </td>
                    <td className="px-[var(--space-3)] py-[var(--space-2)]">{err.field}</td>
                    <td className="px-[var(--space-3)] py-[var(--space-2)]">{err.message}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Action Buttons */}
      <div className="flex items-center gap-[var(--space-3)]">
        <button
          type="button"
          onClick={onConfirm}
          disabled={isSubmitting}
          className="inline-flex items-center gap-[var(--space-2)] rounded-[var(--radius-md)] px-[var(--space-4)] py-[var(--space-2)] text-sm font-medium transition-colors duration-150 disabled:cursor-not-allowed"
          style={{
            backgroundColor: isSubmitting ? "var(--n-200)" : "var(--red-500)",
            color: isSubmitting ? "var(--n-400)" : "var(--n-0)",
          }}
        >
          {isSubmitting ? (
            <>
              <Loader2 size={16} className="animate-spin" aria-hidden="true" />
              Mengunggah...
            </>
          ) : (
            <>
              <CheckCircle2 size={16} aria-hidden="true" />
              Konfirmasi Upload
            </>
          )}
        </button>

        <button
          type="button"
          onClick={onCancel}
          disabled={isSubmitting}
          className="inline-flex items-center rounded-[var(--radius-md)] px-[var(--space-4)] py-[var(--space-2)] text-sm font-medium transition-colors duration-150 disabled:cursor-not-allowed disabled:opacity-50"
          style={{ color: "var(--n-600)", backgroundColor: "var(--n-100)" }}
        >
          Batal
        </button>
      </div>
    </div>
  );
}
