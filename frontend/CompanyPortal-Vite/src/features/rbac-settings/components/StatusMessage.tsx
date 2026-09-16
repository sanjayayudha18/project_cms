/**
 * Shared loading/error/empty states for the RBAC settings menu's list pages
 * (Requirements 2.10, 2.11, 5.6, 6.4, 10.5: status conveyed by icon + text,
 * never color alone). Composes the existing NoticeBanner (error) and
 * EmptyState (empty) components instead of reinventing them.
 */

import { EmptyState } from "@/components/ui/EmptyState";
import { NoticeBanner } from "@/components/ui/NoticeBanner";
import { AlertTriangle, Loader2 } from "lucide-react";

interface StatusMessageProps {
  isLoading: boolean;
  isError: boolean;
  isEmpty: boolean;
  loadingLabel?: string;
  errorMessage?: string;
  emptyMessage?: string;
}

/**
 * Renders one of loading / error / empty, or nothing if none apply (the
 * caller renders its own content in that case). Checked in that priority
 * order: a request that failed while stale data is still shown should not
 * also read as "empty".
 */
export function StatusMessage({
  isLoading,
  isError,
  isEmpty,
  loadingLabel = "Memuat data…",
  errorMessage = "Gagal memuat data. Coba lagi.",
  emptyMessage = "Belum ada data.",
}: StatusMessageProps) {
  if (isLoading) {
    return (
      <div className="flex items-center gap-2 py-8 justify-center text-sm text-[var(--n-500)]">
        <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
        {loadingLabel}
      </div>
    );
  }

  if (isError) {
    return (
      <NoticeBanner
        icon={AlertTriangle}
        variant="danger"
        title="Terjadi kesalahan"
        description={errorMessage}
      />
    );
  }

  if (isEmpty) {
    return <EmptyState message={emptyMessage} />;
  }

  return null;
}
