import { cn } from "@/lib/utils/cn";
import { WifiOff } from "lucide-react";

// ─── Types ────────────────────────────────────────────────────────────────────

export interface NetworkErrorProps {
  onRetry: () => void;
  className?: string;
}

// ─── Component ────────────────────────────────────────────────────────────────

export function NetworkError({ onRetry, className }: NetworkErrorProps) {
  return (
    <div
      role="alert"
      className={cn("flex flex-col items-center justify-center gap-4 p-8 text-center", className)}
    >
      <div
        className="flex items-center justify-center rounded-full p-3"
        style={{ backgroundColor: "var(--danger-bg)" }}
      >
        <WifiOff size={32} style={{ color: "var(--danger-fg)" }} aria-hidden="true" />
      </div>
      <h2 className="text-xl font-semibold" style={{ color: "var(--n-900)" }}>
        Koneksi Terputus
      </h2>
      <p className="max-w-md text-sm" style={{ color: "var(--n-600)" }}>
        Tidak dapat terhubung ke server. Periksa koneksi internet Anda dan coba lagi.
      </p>
      <button
        type="button"
        onClick={onRetry}
        className={cn(
          "mt-2 cursor-pointer rounded-md px-4 py-2 text-sm font-semibold text-white",
          "transition-colors duration-150 ease-out",
          "focus-visible:outline-none",
        )}
        style={{
          backgroundColor: "var(--red-500)",
          borderRadius: "var(--radius-md)",
        }}
        onMouseEnter={(e) => {
          e.currentTarget.style.backgroundColor = "var(--red-600)";
        }}
        onMouseLeave={(e) => {
          e.currentTarget.style.backgroundColor = "var(--red-500)";
        }}
      >
        Coba Lagi
      </button>
    </div>
  );
}
