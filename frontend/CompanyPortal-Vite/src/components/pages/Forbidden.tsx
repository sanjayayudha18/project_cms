import { Link } from "@tanstack/react-router";
import { ShieldX } from "lucide-react";

/**
 * Forbidden (403) page — displayed when authenticated user lacks role access.
 *
 * Shows a clear message in Bahasa Indonesia with a link back to /dashboard.
 * Follows CIMB design system tokens.
 */
export function Forbidden() {
  return (
    <div
      className="flex min-h-[60vh] flex-col items-center justify-center gap-[var(--space-4)] text-center"
      role="alert"
      aria-live="polite"
    >
      {/* Icon */}
      <div
        className="flex h-16 w-16 items-center justify-center rounded-full"
        style={{ backgroundColor: "var(--danger-bg)" }}
      >
        <ShieldX
          size={32}
          style={{ color: "var(--danger-fg)" }}
          aria-hidden="true"
        />
      </div>

      {/* Heading */}
      <h1
        className="text-xl font-semibold"
        style={{ color: "var(--n-900)" }}
      >
        403 — Akses Ditolak
      </h1>

      {/* Description */}
      <p
        className="max-w-[400px] text-sm"
        style={{ color: "var(--n-600)" }}
      >
        Anda tidak memiliki akses ke halaman ini.
      </p>

      {/* Back to dashboard link */}
      <Link
        to="/"
        className="mt-[var(--space-2)] inline-flex items-center gap-[var(--space-2)] rounded-[var(--radius-md)] px-[var(--space-4)] py-[var(--space-2)] text-sm font-medium transition-colors duration-200"
        style={{
          color: "var(--red-600)",
          backgroundColor: "var(--red-50)",
        }}
      >
        Kembali ke Dashboard
      </Link>
    </div>
  );
}
