import { Badge } from "@/components/ui/Badge";
import { DataTable } from "@/components/ui/DataTable";
import { EmptyState } from "@/components/ui/EmptyState";
import { FilterSelect } from "@/components/ui/FilterSelect";
import type { ColumnDef } from "@tanstack/react-table";
import { useMemo, useState } from "react";
import { getErrorMessage, useEodAudit } from "../hooks/useEodQueries";
import {
  type AuditLogEntry,
  FILE_TYPE_LABELS,
  OUTCOME_BADGE_MAP,
  TRIGGER_BADGE_MAP,
  TRIGGER_LABELS,
} from "../types";
import { formatDuration, formatWibDateTimeSec } from "../utils";
import { SectionErrorState } from "./SectionErrorState";

interface AuditLogSectionProps {
  processingDate: string;
  refetchInterval: number | false;
}

const FILE_TYPE_OPTIONS = Object.entries(FILE_TYPE_LABELS).map(([value, label]) => ({
  value,
  label,
}));

const TRIGGER_OPTIONS = Object.entries(TRIGGER_LABELS).map(([value, label]) => ({
  value,
  label,
}));

const OUTCOME_LABELS: Record<string, string> = {
  completed: "Completed",
  failed: "Failed",
  in_progress: "Berlangsung",
};

/** Audit log section: filterable trail of retry actions for the selected date. */
export function AuditLogSection({ processingDate, refetchInterval }: AuditLogSectionProps) {
  const [fileType, setFileType] = useState<string | null>(null);
  const [trigger, setTrigger] = useState<string | null>(null);

  const { data, isLoading, isError, error, refetch } = useEodAudit(
    processingDate,
    fileType,
    trigger,
    refetchInterval,
  );

  // biome-ignore lint/suspicious/noExplicitAny: TanStack Table requires any for column defs
  const columns = useMemo<ColumnDef<AuditLogEntry, any>[]>(
    () => [
      {
        accessorKey: "created_at",
        header: "Timestamp",
        cell: ({ row }) => formatWibDateTimeSec(row.original.created_at),
      },
      { accessorKey: "event_type", header: "Event Type" },
      {
        accessorKey: "trigger",
        header: "Trigger",
        cell: ({ row }) => (
          <Badge
            variant={TRIGGER_BADGE_MAP[row.original.trigger]}
            label={TRIGGER_LABELS[row.original.trigger]}
          />
        ),
      },
      {
        accessorKey: "file_type",
        header: "File Type",
        cell: ({ row }) => FILE_TYPE_LABELS[row.original.file_type],
      },
      { accessorKey: "initiated_by", header: "Initiated By" },
      {
        accessorKey: "outcome",
        header: "Outcome",
        cell: ({ row }) => {
          const key = row.original.outcome ?? "in_progress";
          return (
            <Badge variant={OUTCOME_BADGE_MAP[key] ?? "info"} label={OUTCOME_LABELS[key] ?? key} />
          );
        },
      },
      {
        accessorKey: "duration_ms",
        header: "Duration",
        cell: ({ row }) => formatDuration(row.original.duration_ms),
        meta: { align: "right" },
      },
      {
        accessorKey: "error_detail",
        header: "Error Detail",
        cell: ({ row }) => row.original.error_detail ?? "-",
        enableSorting: false,
      },
    ],
    [],
  );

  return (
    <section aria-labelledby="audit-log-title" aria-busy={isLoading}>
      <h2 id="audit-log-title" className="mb-3 text-base font-semibold text-[var(--n-900)]">
        Log Audit
      </h2>

      <div className="mb-4 flex flex-wrap gap-4">
        <FilterSelect
          label="File Type"
          options={FILE_TYPE_OPTIONS}
          value={fileType}
          onChange={setFileType}
        />
        <FilterSelect
          label="Trigger"
          options={TRIGGER_OPTIONS}
          value={trigger}
          onChange={setTrigger}
        />
      </div>

      {isLoading && (
        <div className="h-64 animate-pulse rounded-[var(--radius-lg)] bg-[var(--n-100)]" />
      )}

      {isError && !isLoading && (
        <SectionErrorState message={getErrorMessage(error)} onRetry={() => refetch()} />
      )}

      {!isLoading && !isError && data && data.length === 0 && (
        <EmptyState message="Tidak ada log audit" />
      )}

      {!isLoading && !isError && data && data.length > 0 && (
        <DataTable data={data} columns={columns} emptyMessage="Tidak ada log audit" />
      )}
    </section>
  );
}
