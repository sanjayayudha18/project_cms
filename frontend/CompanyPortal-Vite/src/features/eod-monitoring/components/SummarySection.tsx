import { SummaryCard } from "@/components/ui/SummaryCard";
import { getErrorMessage, useEodSummary } from "../hooks/useEodQueries";
import { SectionErrorState } from "./SectionErrorState";

interface SummarySectionProps {
  processingDate: string;
  refetchInterval: number | false;
}

const SKELETON_KEYS = ["pending", "processing", "completed", "failed", "max-retries", "late"];

/** Top summary section: 6 aggregate counts of EOD file processing states. */
export function SummarySection({ processingDate, refetchInterval }: SummarySectionProps) {
  const { data, isLoading, isError, error, refetch } = useEodSummary(
    processingDate,
    refetchInterval,
  );

  return (
    <section aria-labelledby="summary-title" aria-busy={isLoading}>
      <h2 id="summary-title" className="mb-3 text-base font-semibold text-[var(--n-900)]">
        Ringkasan
      </h2>

      {isLoading && (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
          {SKELETON_KEYS.map((key) => (
            <div
              key={key}
              className="h-[76px] animate-pulse rounded-[var(--radius-lg)] bg-[var(--n-100)]"
            />
          ))}
        </div>
      )}

      {isError && !isLoading && (
        <SectionErrorState message={getErrorMessage(error)} onRetry={() => refetch()} />
      )}

      {!isLoading && !isError && data && (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
          <SummaryCard label="Pending" value={data.counts.pending} format="number" />
          <SummaryCard label="Processing" value={data.counts.processing} format="number" />
          <SummaryCard label="Completed" value={data.counts.completed} format="number" />
          <SummaryCard label="Failed" value={data.counts.failed} format="number" />
          <SummaryCard
            label="Max Retries Exhausted"
            value={data.counts.max_retries_exhausted}
            format="number"
          />
          <SummaryCard label="Late" value={data.counts.late} format="number" />
        </div>
      )}
    </section>
  );
}
