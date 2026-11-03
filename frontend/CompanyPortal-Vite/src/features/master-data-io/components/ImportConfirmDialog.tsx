import { ConfirmActionDialog } from "@/components/ui/ConfirmActionDialog";
import type { ImportPreview } from "../api";

interface ImportConfirmDialogProps {
  open: boolean;
  entityLabel: string;
  fileName: string;
  preview: ImportPreview;
  isPending: boolean;
  onConfirm: () => void;
  onClose: () => void;
}

/** Req 4.2: nothing is sent until the maker confirms this summary. */
export function ImportConfirmDialog({
  open,
  entityLabel,
  fileName,
  preview,
  isPending,
  onConfirm,
  onClose,
}: ImportConfirmDialogProps) {
  const counts = [
    `${preview.creates} baru`,
    `${preview.data_updates} ubah`,
    `${preview.enables} aktifkan`,
    `${preview.disables} nonaktifkan`,
  ].join(", ");
  const message = `Entitas: ${entityLabel}. File: ${fileName}. Perubahan: ${counts}. Data belum berubah sampai pengajuan disetujui.`;

  return (
    <ConfirmActionDialog
      open={open}
      title="Ajukan impor untuk persetujuan?"
      message={message}
      confirmLabel="Ajukan"
      isPending={isPending}
      onConfirm={onConfirm}
      onClose={onClose}
    />
  );
}
