import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";

interface ConfirmActionDialogProps {
  open: boolean;
  title: string;
  message: string;
  confirmLabel: string;
  isPending: boolean;
  onConfirm: () => void;
  onClose: () => void;
}

/** Generic confirm-before-you-act dialog, built on the shared Dialog primitive (Task 7). */
export function ConfirmActionDialog({
  open,
  title,
  message,
  confirmLabel,
  isPending,
  onConfirm,
  onClose,
}: ConfirmActionDialogProps) {
  return (
    <Dialog open={open} onClose={onClose} title={title}>
      <p className="text-sm text-[var(--n-700)]">{message}</p>
      <div className="mt-6 flex justify-end gap-3">
        <Button variant="secondary" disabled={isPending} onClick={onClose}>
          Batal
        </Button>
        <Button variant="danger" disabled={isPending} onClick={onConfirm}>
          {confirmLabel}
        </Button>
      </div>
    </Dialog>
  );
}
