import { Link } from "@tanstack/react-router";

interface BackLinkProps {
  to: string;
  label: string;
  search?: Record<string, unknown>;
}

/**
 * Tautan kembali ke halaman induk (bukan history.back, jadi aman saat halaman
 * dibuka langsung dari URL). Gaya = tombol ghost, target sentuh min 44×44 px.
 */
export function BackLink({ to, label, search }: BackLinkProps) {
  return (
    <Link
      to={to}
      search={search}
      className="inline-flex items-center justify-center min-h-[44px] min-w-[44px] px-4 rounded-[var(--radius-md)] font-medium text-sm transition-colors duration-150 bg-transparent text-[var(--red-600)] hover:bg-[var(--red-50)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)] focus-visible:border-[var(--red-400)] outline-none cursor-pointer"
    >
      <span aria-hidden="true">←&nbsp;</span>
      {label}
    </Link>
  );
}

export type { BackLinkProps };
