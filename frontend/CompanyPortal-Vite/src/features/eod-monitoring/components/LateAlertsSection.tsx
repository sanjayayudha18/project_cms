import { Badge } from "@/components/ui/Badge";
import { EmptyState } from "@/components/ui/EmptyState";
import { AlertTriangle, CheckCircle } from "lucide-react";
import { getErrorMessage, useEodLate } from "../hooks/useEodQueries";
import { FILE_TYPE_LABELS, type LateDetectionItem } from "../types";
import { formatSlaTime, formatWibDateTime } from "../utils";
import { SectionErrorState } from "./SectionErrorState";

interface LateAlertsSectionProps {
  processingDate: string;
  refetchInterval: number | false;
}

/** Late SLA alerts section: shows file types that breached their SLA deadline. */
export function LateAlertsSection({ processingDate, refetchInterval }: LateAlertsSectionProps) {
  const { data, isLoading, isError, error, refetch } = useEodLate(processingDate, refetchInterval);

  return (
    <section aria-labelledby="late-alerts-title" aria-busy={isLoading}>
      <h2 id="late-alerts-title" className="mb-3 text-base font-semibold text-[var(--n-900)]">
        Peringatan Keterlambatan
      </h2>

      {isLoading && (
        <div className="h-32 animate-pulse rounded-[var(--radius-lg)] bg-[var(--n-100)]" />
      )}

      {isError && !isLoading && (
        <SectionErrorState message={getErrorMessage(error)} onRetry={() => refetch()} />
      )}

      {!isLoading && !isError && data && data.length === 0 && (
        <EmptyState message="Tidak ada file terlambat" />
      )}

      {!isLoading && !isError && data && data.length > 0 && (
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2 lg:grid-cols-3">
          {data.map((item) => (
            <LateAlertCard key={item.id} item={item} />
          ))}
        </div>
      )}
    </section>
  );
}

function LateAlertCard({ item }: { item: LateDetectionItem }) {
  return (
    <div className="rounded-[var(--radius-lg)] border border-[var(--n-200)] bg-[var(--n-0)] p-4 shadow-sm">
      <div className="mb-2 flex items-center justify-between gap-2">
        <p className="text-sm font-medium text-[var(--n-900)]">
          {FILE_TYPE_LABELS[item.file_type]}
        </p>
        {item.is_resolved ? (
          <Badge variant="success" icon={CheckCircle} label="Resolved" />
        ) : (
          <Badge variant="warning" icon={AlertTriangle} label="Late" />
        )}
      </div>
      <p className="text-xs text-[var(--n-600)]">SLA: {formatSlaTime(item.sla_deadline)}</p>
      <p className="text-xs text-[var(--n-600)]">
        Terdeteksi: {formatWibDateTime(item.detected_at)}
      </p>
    </div>
  );
}
