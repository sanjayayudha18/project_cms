import { Button } from "@/components/ui/Button";
import {
  confirmDsrUpload,
  uploadDsrFile,
  type DsrConfirmResponse,
  type DsrDryRunResponse,
} from "@/features/dsr/dsrUploadApi";
import type { ApiError } from "@/lib/api/client";
import { AlertTriangle, CheckCircle2, FileSpreadsheet, Loader2, X } from "lucide-react";
import { useRef, useState } from "react";

interface DsrUploadDialogProps {
  readonly onClose: () => void;
  readonly onConfirmed?: (result: DsrConfirmResponse) => void;
}

type Stage = "pick" | "uploading" | "preview" | "confirming" | "success" | "error";

function formatDecimalIDR(value: string | undefined | null): string {
  if (value === undefined || value === null) return "-";
  const n = Number(value);
  if (Number.isNaN(n)) return value;
  return n.toLocaleString("id-ID");
}

function errorMessage(err: unknown): string {
  if (err && typeof err === "object" && "message" in err) {
    return String((err as ApiError).message);
  }
  return "Terjadi kesalahan tidak terduga";
}

/**
 * Two-phase vendor DSR upload: pick a file -> dry-run preview (nothing saved
 * yet) -> vendor reviews the parsed Daily + Rencana Isi rows -> Confirm
 * persists it, Cancel discards it. Backend re-parses the staged file on
 * confirm rather than trusting anything echoed back from this dialog.
 */
export function DsrUploadDialog({ onClose, onConfirmed }: DsrUploadDialogProps) {
  const [stage, setStage] = useState<Stage>("pick");
  const [preview, setPreview] = useState<DsrDryRunResponse | null>(null);
  const [errorText, setErrorText] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);

  const handleFileChange = async (file: File) => {
    setStage("uploading");
    setErrorText("");
    try {
      const result = await uploadDsrFile(file);
      setPreview(result);
      setStage("preview");
    } catch (err) {
      setErrorText(errorMessage(err));
      setStage("error");
    }
  };

  const handleConfirm = async () => {
    if (!preview) return;
    setStage("confirming");
    try {
      const result = await confirmDsrUpload(preview.staged_filename, preview.checksum);
      setStage("success");
      onConfirmed?.(result);
    } catch (err) {
      setErrorText(errorMessage(err));
      setStage("error");
    }
  };

  const handleRetry = () => {
    setPreview(null);
    setErrorText("");
    setStage("pick");
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
      role="dialog"
      aria-modal="true"
      aria-labelledby="dsr-upload-title"
      onClick={(e) => {
        if (e.target === e.currentTarget && stage !== "uploading" && stage !== "confirming") {
          onClose();
        }
      }}
    >
      <div className="flex max-h-[90vh] w-full max-w-3xl flex-col overflow-hidden rounded-lg bg-white shadow-lg">
        <div className="flex items-center justify-between border-b border-neutral-200 px-5 py-4">
          <h2 id="dsr-upload-title" className="text-lg font-semibold text-surface-text">
            Upload Laporan DSR
          </h2>
          <button
            type="button"
            onClick={onClose}
            aria-label="Tutup"
            disabled={stage === "uploading" || stage === "confirming"}
            className="inline-flex min-h-[44px] min-w-[44px] items-center justify-center rounded-md text-neutral-500 hover:bg-neutral-100 disabled:opacity-50"
          >
            <X className="size-5" aria-hidden="true" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto px-5 py-4">
          {stage === "pick" && (
            <div className="flex flex-col items-center gap-3 py-8">
              <FileSpreadsheet className="size-10 text-neutral-400" aria-hidden="true" />
              <p className="text-center text-sm text-neutral-500">
                Pilih file DSR (.xls atau .xlsx, maks 10MB). Data akan diperiksa dulu sebelum
                disimpan.
              </p>
              <Button onClick={() => inputRef.current?.click()}>Pilih File</Button>
              <input
                ref={inputRef}
                type="file"
                accept=".xls,.xlsx"
                className="hidden"
                onChange={(e) => {
                  const file = e.target.files?.[0];
                  if (file) void handleFileChange(file);
                  e.target.value = "";
                }}
              />
            </div>
          )}

          {stage === "uploading" && (
            <div className="flex flex-col items-center gap-3 py-12">
              <Loader2 className="size-8 animate-spin text-sidebar-active" aria-hidden="true" />
              <p className="text-sm text-neutral-500">Memproses file, mohon tunggu...</p>
            </div>
          )}

          {stage === "error" && (
            <div className="flex flex-col items-center gap-3 py-8">
              <AlertTriangle className="size-8 text-danger-fg" aria-hidden="true" />
              <p className="text-center text-sm text-danger-fg">{errorText}</p>
              <Button variant="secondary" onClick={handleRetry}>
                Coba Lagi
              </Button>
            </div>
          )}

          {stage === "confirming" && (
            <div className="flex flex-col items-center gap-3 py-12">
              <Loader2 className="size-8 animate-spin text-sidebar-active" aria-hidden="true" />
              <p className="text-sm text-neutral-500">Menyimpan data...</p>
            </div>
          )}

          {stage === "success" && (
            <div className="flex flex-col items-center gap-3 py-12">
              <CheckCircle2 className="size-10 text-success-fg" aria-hidden="true" />
              <p className="text-sm font-medium text-surface-text">
                Data DSR berhasil disimpan.
              </p>
              <Button onClick={onClose}>Selesai</Button>
            </div>
          )}

          {stage === "preview" && preview && (
            <DsrPreviewReview preview={preview} />
          )}
        </div>

        {stage === "preview" && (
          <div className="flex items-center justify-end gap-3 border-t border-neutral-200 px-5 py-4">
            <Button variant="secondary" onClick={handleRetry}>
              Batal / Ganti File
            </Button>
            <Button onClick={() => void handleConfirm()}>Konfirmasi & Simpan</Button>
          </div>
        )}
      </div>
    </div>
  );
}

function DsrPreviewReview({ preview }: { readonly preview: DsrDryRunResponse }) {
  const { daily, rencana_isi: rencanaIsi } = preview;

  return (
    <div className="flex flex-col gap-6">
      <div className="rounded-md bg-neutral-50 p-3 text-sm text-neutral-600">
        <p>
          <span className="font-medium">Vendor:</span> {preview.vendor_name}
        </p>
        <p>
          <span className="font-medium">File:</span> {preview.original_filename}
        </p>
        <p className="mt-1 text-xs text-neutral-400">
          Periksa data di bawah ini sebelum menyimpan. Klik "Batal / Ganti File" jika ada yang
          perlu diperbaiki di file sumber.
        </p>
      </div>

      <section>
        <h3 className="mb-2 text-sm font-semibold text-surface-text">Saldo Harian (Daily)</h3>
        {daily.error ? (
          <p className="text-sm text-danger-fg">Gagal membaca sheet Daily: {daily.error}</p>
        ) : (
          <>
            {(daily.error_count ?? 0) > 0 && (
              <p className="mb-2 flex items-center gap-1 text-sm text-warning-fg">
                <AlertTriangle className="size-4" aria-hidden="true" />
                {daily.error_count} sel bermasalah (nilai tidak terbaca) — tetap bisa disimpan.
              </p>
            )}
            <div className="max-h-64 overflow-auto rounded-md border border-neutral-200">
              <table className="w-full text-sm">
                <thead className="sticky top-0 bg-neutral-50 text-left text-xs text-neutral-500">
                  <tr>
                    <th className="px-3 py-2">#</th>
                    <th className="px-3 py-2">Uraian</th>
                    <th className="px-3 py-2 text-right">Total (IDR)</th>
                  </tr>
                </thead>
                <tbody>
                  {daily.rows?.map((row) => (
                    <tr key={row.row_no} className="border-t border-neutral-100">
                      <td className="px-3 py-1.5 text-neutral-400">{row.row_no}</td>
                      <td className="px-3 py-1.5">{row.line_label}</td>
                      <td className="px-3 py-1.5 text-right tabular-nums">
                        {formatDecimalIDR(row.line_total_idr)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </>
        )}
      </section>

      <section>
        <h3 className="mb-2 text-sm font-semibold text-surface-text">Rencana Isi</h3>
        {rencanaIsi.error ? (
          <p className="text-sm text-danger-fg">Gagal membaca sheet Rencana Isi: {rencanaIsi.error}</p>
        ) : (
          <div className="max-h-64 overflow-auto rounded-md border border-neutral-200">
            <table className="w-full text-sm">
              <thead className="sticky top-0 bg-neutral-50 text-left text-xs text-neutral-500">
                <tr>
                  <th className="px-3 py-2">ATM ID</th>
                  <th className="px-3 py-2">Lokasi</th>
                  <th className="px-3 py-2 text-right">Isi 100rb</th>
                  <th className="px-3 py-2 text-right">Isi 50rb</th>
                </tr>
              </thead>
              <tbody>
                {rencanaIsi.rows?.map((row) => (
                  <tr key={row.row_no} className="border-t border-neutral-100">
                    <td className="px-3 py-1.5">{row.atm_terminal_id}</td>
                    <td className="px-3 py-1.5">{row.atm_location ?? "-"}</td>
                    <td className="px-3 py-1.5 text-right tabular-nums">
                      {formatDecimalIDR(row.fill_100k_idr)}
                    </td>
                    <td className="px-3 py-1.5 text-right tabular-nums">
                      {formatDecimalIDR(row.fill_50k_idr)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  );
}
