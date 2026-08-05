import { api } from "@/lib/api/client";
import { useToast } from "@/lib/hooks/useToast";
import { AlertCircle, FileSpreadsheet, Upload } from "lucide-react";
import { useRef, useState } from "react";
import type {
  ColumnValidationResult,
  DSRUploadResponse,
  DSRUploadStep,
  ParsedDSR,
  ValidationError,
} from "../types";
import { parseDSRExcel } from "../utils/dsr-parser";
import { validateDSRColumns, validateDSRFile } from "../utils/dsr-validation";
import { DSRPreviewTable } from "./DSRPreviewTable";

// ─── DSR Upload Form ──────────────────────────────────────────────────────────

export function DSRUploadForm() {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const { toast } = useToast();

  const [step, setStep] = useState<DSRUploadStep>("idle");
  const [parsedData, setParsedData] = useState<ParsedDSR | null>(null);
  const [fileError, setFileError] = useState<string | null>(null);
  const [columnError, setColumnError] = useState<ColumnValidationResult | null>(null);
  const [validationErrors, setValidationErrors] = useState<ValidationError[]>([]);

  // ─── File Selection Handler ───────────────────────────────────────────────

  async function handleFileSelect(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    if (!file) return;

    // Reset state
    setFileError(null);
    setColumnError(null);
    setParsedData(null);
    setValidationErrors([]);

    // Validate file type and size
    const fileValidation = validateDSRFile(file);
    if (!fileValidation.valid) {
      setFileError(fileValidation.error ?? "File tidak valid.");
      resetFileInput();
      return;
    }

    // Parse Excel file
    try {
      const buffer = await file.arrayBuffer();
      const parsed = parseDSRExcel(buffer, file.name, file.size);

      // Validate columns
      const colValidation = validateDSRColumns(parsed.headers);
      if (!colValidation.valid) {
        setColumnError(colValidation);
        setStep("idle");
        resetFileInput();
        return;
      }

      setParsedData(parsed);
      setStep("preview");
    } catch {
      setFileError("Gagal membaca file Excel. Pastikan file tidak rusak dan coba lagi.");
      resetFileInput();
    }
  }

  // ─── Submission Handler ───────────────────────────────────────────────────

  async function handleConfirm() {
    if (!parsedData) return;

    setStep("submitting");
    setValidationErrors([]);

    try {
      const response = await api.post<DSRUploadResponse>("/forecasting/dsr", {
        rows: parsedData.rows,
        filename: parsedData.filename,
        uploadDate: new Date().toISOString(),
      });

      const data = response.data;

      if (data.status === "rejected" && data.errors && data.errors.length > 0) {
        setValidationErrors(data.errors);
        setStep("error");
        return;
      }

      // Success
      const uploadTime = new Date(data.timestamp).toLocaleString("id-ID", {
        dateStyle: "medium",
        timeStyle: "short",
      });

      toast({
        type: "success",
        message: `Upload berhasil — ${data.rowCount} baris diterima pada ${uploadTime}`,
      });

      setStep("success");
      handleReset();
    } catch {
      toast({
        type: "error",
        message: "Gagal mengunggah DSR. Silakan coba lagi.",
      });
      setStep("error");
    }
  }

  // ─── Reset ────────────────────────────────────────────────────────────────

  function handleCancel() {
    handleReset();
  }

  function handleReset() {
    setStep("idle");
    setParsedData(null);
    setFileError(null);
    setColumnError(null);
    setValidationErrors([]);
    resetFileInput();
  }

  function resetFileInput() {
    if (fileInputRef.current) {
      fileInputRef.current.value = "";
    }
  }

  // ─── Render ───────────────────────────────────────────────────────────────

  return (
    <div className="flex flex-col gap-[var(--space-6)]">
      {/* Upload Area — visible only in idle and error states */}
      {(step === "idle" || step === "error") && (
        <div
          className="rounded-[var(--radius-lg)] border-2 border-dashed p-[var(--space-8)] text-center transition-colors duration-200"
          style={{
            borderColor: fileError || columnError ? "var(--danger-solid)" : "var(--n-300)",
            backgroundColor: "var(--n-0)",
          }}
        >
          <div className="flex flex-col items-center gap-[var(--space-3)]">
            <div
              className="flex h-12 w-12 items-center justify-center rounded-full"
              style={{ backgroundColor: "var(--red-50)" }}
            >
              <Upload size={24} style={{ color: "var(--red-500)" }} aria-hidden="true" />
            </div>

            <div className="flex flex-col gap-[var(--space-1)]">
              <p className="text-sm font-medium" style={{ color: "var(--n-800)" }}>
                Pilih file DSR untuk diunggah
              </p>
              <p className="text-xs" style={{ color: "var(--n-500)" }}>
                Format yang diterima: .xlsx, .xls (maks. 10 MB)
              </p>
            </div>

            <label
              className="inline-flex cursor-pointer items-center gap-[var(--space-2)] rounded-[var(--radius-md)] px-[var(--space-4)] py-[var(--space-2)] text-sm font-medium transition-colors duration-150"
              style={{
                backgroundColor: "var(--red-500)",
                color: "var(--n-0)",
              }}
            >
              <FileSpreadsheet size={16} aria-hidden="true" />
              Pilih File
              <input
                ref={fileInputRef}
                type="file"
                accept=".xlsx,.xls"
                onChange={handleFileSelect}
                className="sr-only"
                aria-label="Pilih file DSR Excel"
              />
            </label>
          </div>
        </div>
      )}

      {/* File Validation Error */}
      {fileError && (
        <div
          className="flex items-start gap-[var(--space-3)] rounded-[var(--radius-md)] p-[var(--space-4)]"
          style={{ backgroundColor: "var(--danger-bg)", color: "var(--danger-fg)" }}
          role="alert"
        >
          <AlertCircle size={20} className="mt-0.5 shrink-0" aria-hidden="true" />
          <p className="text-sm">{fileError}</p>
        </div>
      )}

      {/* Column Validation Error */}
      {columnError && !columnError.valid && (
        <div
          className="flex flex-col gap-[var(--space-3)] rounded-[var(--radius-md)] p-[var(--space-4)]"
          style={{ backgroundColor: "var(--danger-bg)", color: "var(--danger-fg)" }}
          role="alert"
        >
          <div className="flex items-start gap-[var(--space-3)]">
            <AlertCircle size={20} className="mt-0.5 shrink-0" aria-hidden="true" />
            <div className="flex flex-col gap-[var(--space-2)]">
              <p className="text-sm font-medium">Struktur kolom file tidak sesuai template DSR</p>

              {columnError.missingColumns.length > 0 && (
                <div>
                  <p className="text-xs font-medium">Kolom yang hilang:</p>
                  <ul className="mt-1 list-inside list-disc text-xs">
                    {columnError.missingColumns.map((col) => (
                      <li key={col}>{col}</li>
                    ))}
                  </ul>
                </div>
              )}

              {columnError.unexpectedColumns.length > 0 && (
                <div>
                  <p className="text-xs font-medium">Kolom tidak dikenal:</p>
                  <ul className="mt-1 list-inside list-disc text-xs">
                    {columnError.unexpectedColumns.map((col) => (
                      <li key={col}>{col}</li>
                    ))}
                  </ul>
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Preview Table with submission flow */}
      {(step === "preview" || step === "submitting" || step === "error") && parsedData && (
        <DSRPreviewTable
          rows={parsedData.rows}
          totalRows={parsedData.totalRows}
          filename={parsedData.filename}
          isSubmitting={step === "submitting"}
          validationErrors={validationErrors}
          onConfirm={handleConfirm}
          onCancel={handleCancel}
        />
      )}
    </div>
  );
}
