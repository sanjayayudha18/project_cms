import { cn } from "@/lib/utils/cn";
import { ServerCrash } from "lucide-react";

// ─── Server Error Message Sanitization ────────────────────────────────────────
// CRITICAL: Never expose technical details (stack traces, SQL errors,
// internal messages, file paths) to the user. Always return a fixed
// generic message regardless of what the server responded with.

const GENERIC_SERVER_ERROR_MESSAGE = "Terjadi kesalahan pada server. Silakan coba lagi nanti.";

/**
 * Patterns that indicate sensitive technical details in error messages.
 * Used internally for detection — the output is ALWAYS the generic message.
 */
const SENSITIVE_PATTERNS = [
  /stack\s*trace/i,
  /at\s+\w+\s*\(/i, // stack frame: "at functionName ("
  /\.go:\d+/i, // Go file paths
  /\.ts:\d+/i, // TypeScript file paths
  /\.js:\d+/i, // JavaScript file paths
  /sql/i,
  /postgres/i,
  /internal\s+server/i,
  /panic/i,
  /segfault/i,
  /null\s*pointer/i,
  /undefined\s+reference/i,
  /ECONNREFUSED/i,
  /errno/i,
] as const;

/**
 * Sanitizes a server error response to prevent leaking technical details.
 * Always returns a safe, user-friendly message in Bahasa Indonesia.
 *
 * @param _errorMessage - The raw error message (intentionally unused — always returns generic)
 * @returns A safe, generic error message
 */
export function sanitizeServerError(_errorMessage?: string | null): string {
  return GENERIC_SERVER_ERROR_MESSAGE;
}

/**
 * Checks whether a message contains sensitive technical details.
 * Useful for logging/alerting purposes only — never show the raw message to users.
 */
export function containsSensitiveDetails(message: string): boolean {
  return SENSITIVE_PATTERNS.some((pattern) => pattern.test(message));
}

// ─── Types ────────────────────────────────────────────────────────────────────

export interface ServerErrorProps {
  onRetry?: () => void;
  className?: string;
}

// ─── Component ────────────────────────────────────────────────────────────────

export function ServerError({ onRetry, className }: ServerErrorProps) {
  return (
    <div
      role="alert"
      className={cn("flex flex-col items-center justify-center gap-4 p-8 text-center", className)}
    >
      <div
        className="flex items-center justify-center rounded-full p-3"
        style={{ backgroundColor: "var(--danger-bg)" }}
      >
        <ServerCrash size={32} style={{ color: "var(--danger-fg)" }} aria-hidden="true" />
      </div>
      <h2 className="text-xl font-semibold" style={{ color: "var(--n-900)" }}>
        Kesalahan Server
      </h2>
      <p className="max-w-md text-sm" style={{ color: "var(--n-600)" }}>
        {GENERIC_SERVER_ERROR_MESSAGE}
      </p>
      {onRetry && (
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
      )}
    </div>
  );
}
