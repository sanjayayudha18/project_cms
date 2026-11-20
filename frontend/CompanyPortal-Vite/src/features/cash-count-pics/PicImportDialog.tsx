import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { useToast } from "@/lib/hooks/useToast";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { CheckCircle2, TriangleAlert } from "lucide-react";
import { useState } from "react";
import {
  IMPORT_MAX_BYTES,
  ImportApiError,
  type ImportConfirmResult,
  type ImportPreview,
} from "../master-data-io/api";
import { ImportChangeTable } from "../master-data-io/components/ImportChangeTable";
import { ImportConfirmDialog } from "../master-data-io/components/ImportConfirmDialog";
import { ImportErrorTable } from "../master-data-io/components/ImportErrorTable";
import { ImportSummary } from "../master-data-io/components/ImportSummary";
import { confirmPicImport, dryRunPicImport } from "./api";

/**
 * CSV upload for the PIC mapping (AC-5.1-11). The file REPLACES the active mapping: any active row
 * missing from the file is deactivated in the same batch, so those rows are called out before submit.
 */
export function PicImportDialog({ onClose }: { onClose: () => void }) {
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const [file, setFile] = useState<File | null>(null);
  const [preview, setPreview] = useState<ImportPreview | null>(null);
  const [result, setResult] = useState<ImportConfirmResult | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);

  const onError = (err: Error) => toast({ type: "error", message: err.message });

  const previewMut = useMutation({
    mutationFn: dryRunPicImport,
    onSuccess: setPreview,
    onError,
  });
  const confirmMut = useMutation({
    mutationFn: confirmPicImport,
    onSuccess: (res) => {
      setConfirmOpen(false);
      setResult(res);
      void queryClient.invalidateQueries({ queryKey: ["cash-count", "pics"] });
    },
    onError: (err: Error) => {
      setConfirmOpen(false);
      // The file turned invalid between check and submit: show it like a dry-run.
      if (err instanceof ImportApiError && err.status === 422 && err.errors.length > 0) {
        setPreview((p) =>
          p ? { ...p, errors: err.errors, errors_truncated: false, changes: [] } : p,
        );
        return;
      }
      onError(err);
    },
  });

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
    file !== null && preview !== null && !hasErrors && !nothingToDo && preview.changes.length > 0;
  const implicitDisables = (preview?.changes ?? []).filter((c) => c.op === "disable");

  return (
    <Dialog open onClose={onClose} title="Unggah CSV Mapping PIC">
      <div className="flex max-h-[75vh] flex-col gap-4 overflow-y-auto">
        <ul className="list-disc pl-5 text-xs text-[var(--n-600)]">
          <li>
            Kolom: <code>pic_role,username,region_code,area_code</code>. Coordinator memakai{" "}
            <code>region_code</code>, Executor memakai <code>area_code</code>.
          </li>
          <li>
            <strong>Mengganti seluruh mapping aktif.</strong> Mapping aktif yang tidak ada di file
            akan dinonaktifkan dalam pengajuan yang sama.
          </li>
          <li>Satu file = satu pengajuan; semua baris disetujui atau ditolak bersama.</li>
        </ul>

        <div className="flex flex-wrap items-center gap-3">
          <input
            type="file"
            accept=".csv,text/csv"
            aria-label="File CSV mapping PIC"
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

        {preview && (
          <>
            <ImportSummary preview={preview} />
            {hasErrors ? (
              <ImportErrorTable errors={preview.errors} truncated={preview.errors_truncated} />
            ) : nothingToDo ? (
              <p className="text-sm text-[var(--n-700)]">Tidak ada perubahan untuk diimpor</p>
            ) : (
              <>
                {implicitDisables.length > 0 && (
                  <section
                    aria-label="Mapping yang akan dinonaktifkan"
                    className="flex flex-col gap-2 rounded border border-[var(--danger-fg)] p-3"
                  >
                    <p className="flex items-center gap-2 text-sm font-semibold text-[var(--danger-fg)]">
                      <TriangleAlert className="h-4 w-4" aria-hidden="true" />
                      {implicitDisables.length.toLocaleString("id-ID")} mapping aktif tidak ada di
                      file dan akan dinonaktifkan
                    </p>
                    <ul className="flex flex-wrap gap-x-4 gap-y-1 font-mono text-xs text-[var(--n-800)]">
                      {implicitDisables.slice(0, 20).map((c) => (
                        <li key={c.key}>{c.key}</li>
                      ))}
                      {implicitDisables.length > 20 && (
                        <li>… dan {implicitDisables.length - 20} lainnya (lihat tabel di bawah)</li>
                      )}
                    </ul>
                  </section>
                )}
                <p className="flex items-center gap-2 text-sm text-[var(--n-700)]">
                  <CheckCircle2 className="h-4 w-4" aria-hidden="true" />
                  File valid dan siap diajukan untuk persetujuan
                </p>
                <ImportChangeTable changes={preview.changes} />
              </>
            )}
          </>
        )}

        {result ? (
          <output className="flex flex-col gap-1 rounded border border-[var(--n-200)] p-4 text-sm">
            <span className="flex items-center gap-2 font-medium text-[var(--n-900)]">
              <CheckCircle2 className="h-4 w-4" aria-hidden="true" />
              {result.existing
                ? "File ini sudah diajukan sebelumnya. Tidak ada pengajuan baru."
                : "Impor diajukan dan menunggu persetujuan."}
            </span>
            <span className="tabular-nums text-[var(--n-600)]">
              Batch #{result.batch_id} · {result.rows.toLocaleString("id-ID")} baris
            </span>
          </output>
        ) : (
          <div className="flex justify-end">
            <Button onClick={() => setConfirmOpen(true)} disabled={!canConfirm}>
              Ajukan untuk persetujuan
            </Button>
          </div>
        )}

        <div className="flex justify-end">
          <Button variant="secondary" onClick={onClose}>
            Tutup
          </Button>
        </div>
      </div>

      {preview && file && (
        <ImportConfirmDialog
          open={confirmOpen}
          entityLabel="Mapping PIC cash count (ganti semua)"
          fileName={file.name}
          preview={preview}
          isPending={confirmMut.isPending}
          onConfirm={() => !confirmMut.isPending && confirmMut.mutate(file)}
          onClose={() => !confirmMut.isPending && setConfirmOpen(false)}
        />
      )}
    </Dialog>
  );
}
