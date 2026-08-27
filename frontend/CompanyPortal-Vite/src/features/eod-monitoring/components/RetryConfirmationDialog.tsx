import { Button } from "@/components/ui/Button";
import { useToast } from "@/lib/hooks/useToast";
import { Loader } from "lucide-react";
import { useEffect, useRef } from "react";
import { getErrorMessage, useRetryFile } from "../hooks/useEodQueries";
import { FILE_TYPE_LABELS, type FileStatusRow } from "../types";

interface RetryConfirmationDialogProps {
  file: FileStatusRow | null;
  onClose: () => void;
}

/** Modal requiring explicit confirmation before a manual retry POST is sent. */
export function RetryConfirmationDialog({ file, onClose }: RetryConfirmationDialogProps) {
  const dialogRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLElement | null>(null);
  const isOpen = file !== null;
  const { toast } = useToast();
  const retryMutation = useRetryFile();

  useEffect(() => {
    if (isOpen) {
      triggerRef.current = document.activeElement as HTMLElement;
      requestAnimationFrame(() => {
        dialogRef.current?.focus();
      });
    }
    return () => {
      triggerRef.current?.focus();
    };
  }, [isOpen]);

  useEffect(() => {
    if (!isOpen) return;
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape" && !retryMutation.isPending) onClose();
    }
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [isOpen, onClose, retryMutation.isPending]);

  if (!isOpen || !file) return null;

  function handleConfirm() {
    if (!file) return;
    retryMutation.mutate(file.file_id, {
      onSuccess: () => {
        toast({ type: "success", message: `Retry berhasil dipicu untuk ${file.filename}` });
        onClose();
      },
      onError: (error) => {
        toast({ type: "error", message: getErrorMessage(error) });
        onClose();
      },
    });
  }

  return (
    <div className="fixed inset-0 z-[200] flex items-center justify-center p-4" role="presentation">
      <button
        type="button"
        aria-label="Batal"
        onClick={() => !retryMutation.isPending && onClose()}
        className="absolute inset-0 bg-black/40"
        style={{ animation: "eod-dialog-backdrop-enter 150ms ease-out forwards" }}
      />

      <div
        ref={dialogRef}
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="retry-dialog-title"
        tabIndex={-1}
        className="relative w-full max-w-md rounded-[var(--radius-lg)] bg-[var(--n-0)] p-6 shadow-[var(--shadow-md)] outline-none"
        style={{ animation: "eod-dialog-enter 200ms cubic-bezier(0.22,1,0.36,1) forwards" }}
      >
        <h2 id="retry-dialog-title" className="mb-3 text-lg font-semibold text-[var(--n-900)]">
          Konfirmasi Retry Manual
        </h2>

        <div className="mb-4 flex flex-col gap-1 rounded-[var(--radius-md)] bg-[var(--n-50)] p-3 text-sm">
          <p>
            <span className="text-[var(--n-600)]">Filename:</span> {file.filename}
          </p>
          <p>
            <span className="text-[var(--n-600)]">Tipe File:</span>{" "}
            {FILE_TYPE_LABELS[file.file_type]}
          </p>
          <p>
            <span className="text-[var(--n-600)]">Jumlah Retry:</span> {file.retry_count}
          </p>
        </div>

        <p className="mb-6 text-sm text-[var(--n-700)]">
          Retry manual akan menjalankan ulang script ETL untuk file ini. Tindakan ini tidak dapat
          dibatalkan.
        </p>

        <div className="flex justify-end gap-3">
          <Button variant="secondary" disabled={retryMutation.isPending} onClick={onClose}>
            Batal
          </Button>
          <Button variant="primary" disabled={retryMutation.isPending} onClick={handleConfirm}>
            {retryMutation.isPending && (
              <Loader className="mr-2 h-4 w-4 animate-spin" aria-hidden="true" />
            )}
            Konfirmasi Retry
          </Button>
        </div>
      </div>

      <style>
        {`
          @keyframes eod-dialog-enter {
            from { transform: scale(0.95); opacity: 0; }
            to { transform: scale(1); opacity: 1; }
          }
          @keyframes eod-dialog-backdrop-enter {
            from { opacity: 0; }
            to { opacity: 1; }
          }
        `}
      </style>
    </div>
  );
}
