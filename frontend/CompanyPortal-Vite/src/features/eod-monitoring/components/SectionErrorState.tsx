import { Button } from "@/components/ui/Button";
import { AlertTriangle } from "lucide-react";

interface SectionErrorStateProps {
  message: string;
  onRetry: () => void;
}

/** Inline error state with a "Coba Lagi" retry button, used within a failed section. */
export function SectionErrorState({ message, onRetry }: SectionErrorStateProps) {
  return (
    <div className="flex flex-col items-center justify-center gap-2 py-8 text-center">
      <AlertTriangle className="h-8 w-8 text-[var(--danger-fg)]" aria-hidden="true" />
      <p className="text-sm text-[var(--n-700)]">{message}</p>
      <Button variant="secondary" onClick={onRetry}>
        Coba Lagi
      </Button>
    </div>
  );
}
