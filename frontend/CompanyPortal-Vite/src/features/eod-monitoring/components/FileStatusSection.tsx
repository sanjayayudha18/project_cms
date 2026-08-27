import { Badge } from "@/components/ui/Badge";
import { DataTable } from "@/components/ui/DataTable";
import { EmptyState } from "@/components/ui/EmptyState";
import type { ColumnDef } from "@tanstack/react-table";
import { useMemo } from "react";
import { getErrorMessage, useEodStatus } from "../hooks/useEodQueries";
import {
  FILE_TYPE_LABELS,
  type FileStatusRow,
  STATUS_BADGE_CONFIG,
  flattenFileStatus,
} from "../types";
import { formatWibDateTime } from "../utils";
import { SectionErrorState } from "./SectionErrorState";

interface FileStatusSectionProps {
  processingDate: string;
  refetchInterval: number | false;
  onFileSelect: (file: FileStatusRow) => void;
}

/** File status table: per-file EOD processing status, sortable, click-through to retry drawer. */
export function FileStatusSection({
  processingDate,
  refetchInterval,
  onFileSelect,
}: FileStatusSectionProps) {
  const { data, isLoading, isError, error, refetch } = useEodStatus(
    processingDate,
    refetchInterval,
  );

  const rows = useMemo(() => (data ? flattenFileStatus(data) : []), [data]);

  // biome-ignore lint/suspicious/noExplicitAny: TanStack Table requires any for column defs
  const columns = useMemo<ColumnDef<FileStatusRow, any>[]>(
    () => [
      {
        accessorKey: "file_type",
        header: "File Type",
        cell: ({ row }) => (
          <button
            type="button"
            onClick={() => onFileSelect(row.original)}
            className="rounded-[var(--radius-sm)] text-left font-medium text-[var(--red-600)] underline-offset-2 hover:underline focus-visible:ring-2 focus-visible:ring-[var(--red-100)] outline-none"
          >
            {FILE_TYPE_LABELS[row.original.file_type]}
          </button>
        ),
      },
      { accessorKey: "filename", header: "Filename" },
      {
        accessorKey: "processing_status",
        header: "Status",
        cell: ({ row }) => {
          const config = STATUS_BADGE_CONFIG[row.original.processing_status];
          return <Badge variant={config.variant} icon={config.icon} label={config.label} />;
        },
      },
      {
        accessorKey: "retry_count",
        header: "Retry Count",
        meta: { align: "right" },
      },
      {
        accessorKey: "detected_at",
        header: "Detected At",
        cell: ({ row }) => formatWibDateTime(row.original.detected_at),
      },
      {
        accessorKey: "last_retry_at",
        header: "Last Retry At",
        cell: ({ row }) =>
          row.original.last_retry_at ? formatWibDateTime(row.original.last_retry_at) : "-",
        enableSorting: false,
      },
      {
        accessorKey: "failure_reason",
        header: "Failure Reason",
        cell: ({ row }) => row.original.failure_reason ?? "-",
        enableSorting: false,
      },
    ],
    [onFileSelect],
  );

  return (
    <section aria-labelledby="file-status-title" aria-busy={isLoading}>
      <h2 id="file-status-title" className="mb-3 text-base font-semibold text-[var(--n-900)]">
        Status File
      </h2>

      {isLoading && (
        <div className="h-64 animate-pulse rounded-[var(--radius-lg)] bg-[var(--n-100)]" />
      )}

      {isError && !isLoading && (
        <SectionErrorState message={getErrorMessage(error)} onRetry={() => refetch()} />
      )}

      {!isLoading && !isError && rows.length === 0 && (
        <EmptyState message="Tidak ada file untuk tanggal proses ini" />
      )}

      {!isLoading && !isError && rows.length > 0 && (
        <DataTable
          data={rows}
          columns={columns}
          emptyMessage="Tidak ada file untuk tanggal proses ini"
        />
      )}
    </section>
  );
}
