import { Button } from "@/components/ui/Button";
import { PageHeader } from "@/components/ui/PageHeader";
import { useToast } from "@/lib/hooks/useToast";
import { useMutation } from "@tanstack/react-query";
import { AlertCircle, CheckCircle2 } from "lucide-react";
import { useState } from "react";
import {
  type ExportStatus,
  IMPORT_MAX_BYTES,
  IMPORT_MAX_ROWS,
  IO_ENTITIES,
  type ImportPreview,
  type IoEntity,
  confirmImport,
  downloadExport,
  downloadTemplate,
  dryRunImport,
} from "../api";

const STATUS_OPTIONS: { id: ExportStatus; label: string }[] = [
  { id: "active", label: "Aktif" },
  { id: "disabled", label: "Nonaktif" },
  { id: "all", label: "Semua" },
];

const SELECT_CLASS =
  "rounded border border-[var(--n-300)] bg-[var(--n-0)] px-3 py-2 text-sm text-[var(--n-900)]";

/**
 * Ekspor / impor CSV data master. Impor selalu lewat pratinjau (dry-run): file
 * hanya bisa diajukan bila tanpa kesalahan, lalu seluruh file menunggu SATU
 * persetujuan dan diterapkan sekaligus (semua atau tidak sama sekali).
 */
export function MasterDataIOPage() {
  const { toast } = useToast();
  const [entity, setEntity] = useState<IoEntity>("vendors");
  const [status, setStatus] = useState<ExportStatus>("active");
  const [file, setFile] = useState<File | null>(null);
  const [preview, setPreview] = useState<ImportPreview | null>(null);

  const onError = (err: Error) => toast({ type: "error", message: err.message });

  const exportMut = useMutation({ mutationFn: () => downloadExport(entity, status), onError });
  const templateMut = useMutation({ mutationFn: () => downloadTemplate(entity), onError });
  const previewMut = useMutation({
    mutationFn: (f: File) => dryRunImport(entity, f),
    onSuccess: setPreview,
    onError,
  });
  const confirmMut = useMutation({
    mutationFn: (f: File) => confirmImport(entity, f),
    onSuccess: (res) => {
      toast({
        type: "success",
        message: res.existing
          ? `File yang sama sudah diajukan sebelumnya (batch #${res.batch_id}, ${res.rows} baris)`
          : `Impor diajukan dan menunggu persetujuan (batch #${res.batch_id}, ${res.rows} baris)`,
      });
      reset();
    },
    onError,
  });

  function reset(): void {
    setFile(null);
    setPreview(null);
  }

  function pickFile(f: File | null): void {
    setPreview(null);
    if (f && f.size > IMPORT_MAX_BYTES) {
      toast({ type: "error", message: "Ukuran file melebihi 5 MiB" });
      setFile(null);
      return;
    }
    setFile(f);
  }

  const hasErrors = (preview?.errors.length ?? 0) > 0;
  const nothingToDo = preview !== null && preview.creates + preview.updates === 0;
  const canConfirm = file !== null && preview !== null && !hasErrors && !nothingToDo;

  return (
    <div className="flex flex-col gap-6 p-6">
      <PageHeader
        eyebrow="Data master"
        title="Ekspor & Impor CSV"
        description="Unduh data atau templat, ubah di spreadsheet, lalu impor kembali. Perubahan baru diterapkan setelah disetujui."
      />

      <div className="flex flex-wrap items-end gap-4">
        <label className="flex flex-col gap-1 text-sm font-medium text-[var(--n-700)]">
          Entitas
          <select
            className={SELECT_CLASS}
            value={entity}
            onChange={(e) => {
              setEntity(e.target.value as IoEntity);
              reset();
            }}
          >
            {IO_ENTITIES.map((o) => (
              <option key={o.id} value={o.id}>
                {o.label}
              </option>
            ))}
          </select>
        </label>
        <label className="flex flex-col gap-1 text-sm font-medium text-[var(--n-700)]">
          Status data ekspor
          <select
            className={SELECT_CLASS}
            value={status}
            onChange={(e) => setStatus(e.target.value as ExportStatus)}
          >
            {STATUS_OPTIONS.map((o) => (
              <option key={o.id} value={o.id}>
                {o.label}
              </option>
            ))}
          </select>
        </label>
        <Button
          variant="secondary"
          onClick={() => exportMut.mutate()}
          disabled={exportMut.isPending}
        >
          Unduh data (CSV)
        </Button>
        <Button
          variant="secondary"
          onClick={() => templateMut.mutate()}
          disabled={templateMut.isPending}
        >
          Unduh templat
        </Button>
      </div>

      <section className="flex flex-col gap-3 rounded-lg border border-[var(--n-200)] p-4">
        <h2 className="text-base font-semibold text-[var(--n-900)]">Impor</h2>
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
        </ul>
        <div className="flex flex-wrap items-center gap-3">
          <input
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

        {preview && (
          <div className="flex flex-col gap-3">
            <div className="flex flex-wrap items-center gap-4 text-sm tabular-nums">
              <span>Total baris: {preview.total_rows}</span>
              <span>Baru: {preview.creates}</span>
              <span>Ubah: {preview.updates}</span>
              <span>Tanpa perubahan: {preview.unchanged}</span>
              <span>Valid: {preview.valid_rows}</span>
            </div>

            {hasErrors ? (
              <div role="alert" className="flex flex-col gap-2">
                <p className="flex items-center gap-2 text-sm font-medium text-[var(--danger-fg)]">
                  <AlertCircle className="h-4 w-4" aria-hidden="true" />
                  {preview.errors.length} kesalahan, perbaiki file lalu periksa ulang
                </p>
                <table className="w-full border-collapse text-sm">
                  <thead>
                    <tr className="text-left text-[var(--n-500)]">
                      <th className="py-2 pr-4 font-medium">Baris</th>
                      <th className="py-2 pr-4 font-medium">Kolom</th>
                      <th className="py-2 font-medium">Kesalahan</th>
                    </tr>
                  </thead>
                  <tbody>
                    {preview.errors.map((e) => (
                      <tr
                        key={`${e.row}-${e.field}-${e.message}`}
                        className="border-t border-[var(--n-200)]"
                      >
                        <td className="py-2 pr-4 tabular-nums">{e.row || "—"}</td>
                        <td className="py-2 pr-4">{e.field || "—"}</td>
                        <td className="py-2">{e.message}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                {preview.errors_truncated && (
                  <p className="text-xs text-[var(--n-500)]">
                    Hanya sebagian kesalahan yang ditampilkan; perbaiki lalu periksa ulang.
                  </p>
                )}
              </div>
            ) : (
              <p className="flex items-center gap-2 text-sm text-[var(--n-700)]">
                <CheckCircle2 className="h-4 w-4" aria-hidden="true" />
                {nothingToDo
                  ? "Tidak ada perubahan untuk diimpor"
                  : "File valid dan siap diajukan untuk persetujuan"}
              </p>
            )}

            <div>
              <Button
                onClick={() => file && confirmMut.mutate(file)}
                disabled={!canConfirm || confirmMut.isPending}
              >
                Ajukan untuk persetujuan
              </Button>
            </div>
          </div>
        )}
      </section>
    </div>
  );
}
