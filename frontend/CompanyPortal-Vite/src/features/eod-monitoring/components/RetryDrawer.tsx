import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { X } from "lucide-react";
import { useEffect, useRef } from "react";
import { getErrorMessage, useFileHistory } from "../hooks/useEodQueries";
import {
  FILE_TYPE_LABELS,
  type FileStatusRow,
  STATUS_BADGE_CONFIG,
  TRIGGER_BADGE_MAP,
  TRIGGER_LABELS,
} from "../types";
import { formatDuration, formatWibDateTime } from "../utils";
import { SectionErrorState } from "./SectionErrorState";

interface RetryDrawerProps {
  file: FileStatusRow | null;
  onClose: () => void;
  onRetryClick: (file: FileStatusRow) => void;
}

const CHECKSUM_TRUNCATE_LENGTH = 12;

/** Right-side drawer showing retry history for a selected file. */
export function RetryDrawer({ file, onClose, onRetryClick }: RetryDrawerProps) {
  const drawerRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLElement | null>(null);
  const isOpen = file !== null;

  const { data, isLoading, isError, error, refetch } = useFileHistory(file?.file_id ?? null);

  useEffect(() => {
    if (isOpen) {
      triggerRef.current = document.activeElement as HTMLElement;
      requestAnimationFrame(() => {
        drawerRef.current?.querySelector<HTMLElement>("[data-autofocus]")?.focus();
      });
    }
    return () => {
      triggerRef.current?.focus();
    };
  }, [isOpen]);

  useEffect(() => {
    if (!isOpen) return;
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [isOpen, onClose]);

  if (!isOpen || !file) return null;

  const statusConfig = STATUS_BADGE_CONFIG[file.processing_status];
  const checksum =
    file.checksum.length >= CHECKSUM_TRUNCATE_LENGTH
      ? `${file.checksum.slice(0, CHECKSUM_TRUNCATE_LENGTH)}…`
      : file.checksum;

  const showRetryButton =
    file.processing_status === "failed" ||
    file.processing_status === "max_retries_exhausted" ||
    file.processing_status === "processing";
  const retryDisabled = file.processing_status === "processing";

  return (
    <div className="fixed inset-0 z-[100]" role="presentation">
      {/* Backdrop */}
      <button
        type="button"
        aria-label="Tutup panel"
        onClick={onClose}
        className="absolute inset-0 bg-black/30"
        style={{
          animation: "eod-drawer-backdrop-enter 300ms cubic-bezier(0.22,1,0.36,1) forwards",
        }}
      />

      <div
        ref={drawerRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby="retry-drawer-title"
        className="absolute right-0 top-0 h-full w-full bg-[var(--n-0)] shadow-[var(--shadow-md)] sm:w-[400px]"
        style={{
          animation: "eod-drawer-enter 300ms cubic-bezier(0.22,1,0.36,1) forwards",
        }}
      >
        <div className="flex h-full flex-col overflow-y-auto p-6">
          <div className="mb-4 flex items-start justify-between gap-4">
            <h2 id="retry-drawer-title" className="text-lg font-semibold text-[var(--n-900)]">
              Riwayat Retry
            </h2>
            <button
              type="button"
              onClick={onClose}
              data-autofocus
              aria-label="Tutup"
              className="flex min-h-[44px] min-w-[44px] items-center justify-center rounded-[var(--radius-md)] text-[var(--n-500)] hover:bg-[var(--n-100)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)] outline-none"
            >
              <X className="h-5 w-5" />
            </button>
          </div>

          <div className="mb-6 flex flex-col gap-2 rounded-[var(--radius-md)] bg-[var(--n-50)] p-4">
            <p className="text-sm font-medium text-[var(--n-900)]">{file.filename}</p>
            <p className="text-xs text-[var(--n-600)]">{FILE_TYPE_LABELS[file.file_type]}</p>
            <p className="text-xs text-[var(--n-500)]">Checksum: {checksum}</p>
            <div>
              <Badge
                variant={statusConfig.variant}
                icon={statusConfig.icon}
                label={statusConfig.label}
              />
            </div>
          </div>

          {isLoading && (
            <div className="h-40 animate-pulse rounded-[var(--radius-lg)] bg-[var(--n-100)]" />
          )}

          {isError && !isLoading && (
            <SectionErrorState message={getErrorMessage(error)} onRetry={() => refetch()} />
          )}

          {!isLoading && !isError && data && (
            <ul className="flex flex-col gap-3">
              {data.attempts.map((attempt) => (
                <li
                  key={attempt.attempt_number}
                  className="rounded-[var(--radius-md)] border border-[var(--n-200)] p-3"
                >
                  <div className="mb-1 flex items-center justify-between gap-2">
                    <span className="text-sm font-medium text-[var(--n-900)]">
                      Percobaan #{attempt.attempt_number}
                    </span>
                    <Badge
                      variant={TRIGGER_BADGE_MAP[attempt.trigger]}
                      label={TRIGGER_LABELS[attempt.trigger]}
                    />
                  </div>
                  <p className="text-xs text-[var(--n-600)]">
                    Mulai: {formatWibDateTime(attempt.started_at)}
                  </p>
                  <p className="text-xs text-[var(--n-600)]">
                    Durasi: {formatDuration(attempt.duration_ms)}
                  </p>
                  <div className="mt-1 flex items-center gap-2">
                    <Badge
                      variant={
                        attempt.outcome === "completed"
                          ? "success"
                          : attempt.outcome === "failed"
                            ? "danger"
                            : "info"
                      }
                      label={
                        attempt.outcome === "completed"
                          ? "Completed"
                          : attempt.outcome === "failed"
                            ? "Failed"
                            : "Berlangsung"
                      }
                    />
                  </div>
                  {attempt.outcome === "failed" && attempt.error_detail && (
                    <p className="mt-1 text-xs text-[var(--danger-fg)]">{attempt.error_detail}</p>
                  )}
                </li>
              ))}
            </ul>
          )}

          {showRetryButton && (
            <div className="mt-auto pt-6">
              <Button
                variant="primary"
                disabled={retryDisabled}
                onClick={() => onRetryClick(file)}
                className="w-full"
              >
                Retry Manual
              </Button>
            </div>
          )}
        </div>
      </div>

      <style>
        {`
          @keyframes eod-drawer-enter {
            from { transform: translateX(100%); opacity: 0; }
            to { transform: translateX(0); opacity: 1; }
          }
          @keyframes eod-drawer-backdrop-enter {
            from { opacity: 0; }
            to { opacity: 1; }
          }
        `}
      </style>
    </div>
  );
}
