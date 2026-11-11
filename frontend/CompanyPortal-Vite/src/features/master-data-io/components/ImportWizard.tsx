import { Button } from "@/components/ui/Button";
import { downloadMasterDataTemplate } from "@/features/exports";
import { useToast } from "@/lib/hooks/useToast";
import { useMutation } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { CheckCircle2 } from "lucide-react";
import { useState } from "react";
import {
  IMPORT_MAX_BYTES,
  IMPORT_MAX_ROWS,
  IO_ENTITIES,
  ImportApiError,
  type ImportConfirmResult,
  type ImportPreview,
  type IoEntity,
  confirmImport,
  dryRunImport,
} from "../api";
import { ImportChangeTable } from "./ImportChangeTable";
import { ImportConfirmDialog } from "./ImportConfirmDialog";
import { ImportErrorTable } from "./ImportErrorTable";
import { ImportResultPanel } from "./ImportResultPanel";
import { ImportSteps } from "./ImportSteps";
import { ImportSummary } from "./ImportSummary";

const SELECT_CLASS =
  "rounded border border-[var(--n-300)] bg-[var(--n-0)] px-3 py-2 text-sm text-[var(--n-900)]";
const CARD_CLASS = "flex flex-col gap-3 rounded-lg border border-[var(--n-200)] p-4";

/**
 * Tab "Impor baru": wizard tiga langkah (pilih data, unggah + periksa, tinjau +
 * ajukan). Mengganti entitas atau file menghapus hasil pemeriksaan dan kembali ke
 * langkah 2. Langkah 3 masih tampilan hasil lama (diganti task 11.3/11.4).
 */
export function ImportWizard() {
  const { toast } = useToast();
  const [entity, setEntity] = useState<IoEntity>("vendors");
  const [file, setFile] = useState<File | null>(null);
  const [preview, setPreview] = useState<ImportPreview | null>(null);
  const [result, setResult] = useState<ImportConfirmResult | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  // Remounts the uncontrolled file input so "Impor file lain" clears the chosen file.
  const [inputKey, setInputKey] = useState(0);

  const onError = (err: Error) => toast({ type: "error", message: err.message });

  const templateMut = useMutation({
    mutationFn: () => downloadMasterDataTemplate(entity),
    onError,
  });
  const previewMut = useMutation({
    mutationFn: (f: File) => dryRunImport(entity, f),
    onSuccess: setPreview,
    onError,
  });
  const confirmMut = useMutation({
    mutationFn: (f: File) => confirmImport(entity, f),
    onSuccess: (res) => {
      setConfirmOpen(false);
      setResult(res);
    },
    onError: (err: Error) => {
      setConfirmOpen(false);
      // Req 4.6: the file turned invalid between check and submit; show it like a dry-run.
      if (err instanceof ImportApiError && err.status === 422 && err.errors.length > 0) {
        setPreview((p) =>
          p ? { ...p, errors: err.errors, errors_truncated: false, changes: [] } : p,
        );
        return;
      }
      onError(err);
    },
  });

  function reset(): void {
    setFile(null);
    setPreview(null);
    setResult(null);
    setConfirmOpen(false);
    setInputKey((k) => k + 1);
  }

  function pickEntity(next: IoEntity): void {
    setEntity(next);
    reset();
  }

  function pickFile(f: File | null): void {
    setPreview(null);
    setResult(null);
    if (f && f.size > IMPORT_MAX_BYTES) {
      toast({ type: "error", message: "Ukuran file melebihi 5 MiB" });
      setFile(null);
      return;
    }
    setFile(f);
  }

  const hasErrors = (preview?.errors.length ?? 0) > 0;
  const nothingToDo = preview !== null && preview.creates + preview.updates === 0;
  const canConfirm =
    file !== null && preview !== null && !hasErrors && preview.changes.length > 0 && !nothingToDo;
  const entityLabel = IO_ENTITIES.find((o) => o.id === entity)?.label;

  return (
    <div className="flex flex-col gap-6">
      <ImportSteps
        current={preview ? 3 : 2}
        details={[entityLabel, file?.name, preview ? `${preview.total_rows} baris` : undefined]}
      />

      <section className={CARD_CLASS}>
        <h2 className="text-base font-semibold text-[var(--n-900)]">
          <span className="mr-2 text-xs font-bold text-[var(--n-500)]">LANGKAH 1</span>
          Pilih data
        </h2>
        <div className="flex flex-wrap items-end gap-4">
          <label className="flex flex-col gap-1 text-sm font-medium text-[var(--n-700)]">
            Entitas
            <select
              className={SELECT_CLASS}
              value={entity}
              onChange={(e) => pickEntity(e.target.value as IoEntity)}
            >
              {IO_ENTITIES.map((o) => (
                <option key={o.id} value={o.id}>
                  {o.label}
                </option>
              ))}
            </select>
          </label>
          <Button
            variant="secondary"
            onClick={() => templateMut.mutate()}
            disabled={templateMut.isPending}
          >
            Unduh templat
          </Button>
          <Link
            to="/exports/$dataset"
            params={{ dataset: entity }}
            className="w-fit pb-2 text-sm font-medium text-[var(--red-600)] underline"
          >
            Ekspor data ini
          </Link>
        </div>
        <ul className="list-disc pl-5 text-xs text-[var(--n-600)]">
          <li>
            CSV dengan pemisah koma atau titik koma, maks. 5 MiB dan{" "}
            {IMPORT_MAX_ROWS.toLocaleString("id-ID")} baris. Kolom <code>id</code> kosong = data
            baru; terisi = ubah.
          </li>
          <li>
            Excel menghapus angka nol di depan (mis. NPWP). Buka CSV di Excel dengan kolom bertipe
            Teks (Data → Dari Teks/CSV), atau awali nilai dengan tanda petik satu (').
          </li>
          <li>Baris tidak bisa mengubah data dan status aktif sekaligus.</li>
          {entity === "vendors" && (
            <li>
              File ekspor vendor berisi kolom Direktur Utama dan PKS. Kolom itu hanya untuk dibaca
              dan diabaikan saat impor. Ubah PKS di tab PKS dan direktur di data PIC.
            </li>
          )}
        </ul>
      </section>

      <section className={CARD_CLASS}>
        <h2 className="text-base font-semibold text-[var(--n-900)]">
          <span className="mr-2 text-xs font-bold text-[var(--n-500)]">LANGKAH 2</span>
          Unggah dan periksa
        </h2>
        <div className="flex flex-wrap items-center gap-3">
          <input
            key={inputKey}
            type="file"
            accept=".csv,text/csv"
            aria-label="File CSV impor"
            className="text-sm"
            onChange={(e) => pickFile(e.target.files?.[0] ?? null)}
          />
          <Button
            onClick={() => file && previewMut.mutate(file)}
            disabled={!file || previewMut.isPending}
          >
            Periksa file
          </Button>
        </div>
      </section>

      {preview && (
        <section className={CARD_CLASS}>
          <h2 className="text-base font-semibold text-[var(--n-900)]">
            <span className="mr-2 text-xs font-bold text-[var(--n-500)]">LANGKAH 3</span>
            Tinjau dan ajukan
          </h2>
          <ImportSummary preview={preview} />

          {hasErrors ? (
            <ImportErrorTable errors={preview.errors} truncated={preview.errors_truncated} />
          ) : (
            <>
              <p className="flex items-center gap-2 text-sm text-[var(--n-700)]">
                <CheckCircle2 className="h-4 w-4" aria-hidden="true" />
                {nothingToDo
                  ? "Tidak ada perubahan untuk diimpor"
                  : "File valid dan siap diajukan untuk persetujuan"}
              </p>
              {!nothingToDo && <ImportChangeTable changes={preview.changes} />}
            </>
          )}

          {result ? (
            <ImportResultPanel result={result} onReset={reset} />
          ) : (
            <div>
              <Button onClick={() => setConfirmOpen(true)} disabled={!canConfirm}>
                Ajukan untuk persetujuan
              </Button>
            </div>
          )}
        </section>
      )}

      {preview && file && (
        <ImportConfirmDialog
          open={confirmOpen}
          entityLabel={entityLabel ?? entity}
          fileName={file.name}
          preview={preview}
          isPending={confirmMut.isPending}
          onConfirm={() => !confirmMut.isPending && confirmMut.mutate(file)}
          onClose={() => !confirmMut.isPending && setConfirmOpen(false)}
        />
      )}
    </div>
  );
}
