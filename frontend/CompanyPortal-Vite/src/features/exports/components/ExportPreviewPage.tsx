import { PageHeader } from "@/components/ui/PageHeader";
import { Link } from "@tanstack/react-router";
import { ArrowLeft } from "lucide-react";
import { useState } from "react";
import { type MasterDataset, type PreviewFilter, toParams, toSearch } from "../datasets";
import { usePreviewPaging } from "../usePreviewPaging";
import { PreviewExportBar } from "./PreviewExportBar";
import { PreviewFilterBar, isSameFilter } from "./PreviewFilterBar";
import { PreviewTable } from "./PreviewTable";

const formatCount = new Intl.NumberFormat("id-ID");

export interface ExportPreviewPageProps {
  dataset: MasterDataset;
  /** Applied_Filter, i.e. `parseFilterSearch(dataset.id, routeSearch)`. */
  applied: PreviewFilter;
  /** Called with the URL search for the draft; the route writes it via `navigate({ search })`. */
  onApplySearch: (search: Record<string, string | number>) => void;
}

/** Preview of one master-data export: filter → paged table → "Ekspor N baris" (Req 2, 3, 5). */
export function ExportPreviewPage({ dataset, applied, onApplySearch }: ExportPreviewPageProps) {
  const [draft, setDraft] = useState<PreviewFilter>(applied);
  const params = toParams(dataset.id, applied);
  const paging = usePreviewPaging(params);
  const { query } = paging;

  const total = query.data?.total;
  const dirty = !isSameFilter(draft, applied);
  const loading = params !== null && (query.isLoading || query.isFetching);

  return (
    <div className="flex flex-col gap-6">
      <div>
        <Link
          to="/exports"
          className="mb-4 inline-flex min-h-[44px] items-center gap-2 text-sm font-medium text-[var(--red-600)] outline-none hover:underline focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
        >
          <ArrowLeft className="h-4 w-4" aria-hidden="true" />
          Kembali ke Export Data
        </Link>
        <PageHeader eyebrow="Export Data" title={dataset.label} description={dataset.description} />
        {total !== undefined && (
          <p className="-mt-4 text-sm text-[var(--n-600)]">
            Total <span className="font-semibold tabular-nums">{formatCount.format(total)}</span>{" "}
            baris
          </p>
        )}
      </div>

      <PreviewFilterBar
        dataset={dataset}
        applied={applied}
        onApply={(f) => onApplySearch(toSearch(f))}
        onDraftChange={setDraft}
      />

      {params !== null && (
        <PreviewTable
          dataset={dataset}
          data={query.data}
          isLoading={query.isLoading}
          isFetching={query.isFetching}
          isError={query.isError}
          onRetry={() => void query.refetch()}
          pageNumber={paging.pageNumber}
          canPrev={paging.canPrev}
          canNext={paging.canNext}
          onPrev={paging.prev}
          onNext={paging.next}
        />
      )}

      <PreviewExportBar params={params} total={total ?? 0} dirty={dirty} loading={loading} />
    </div>
  );
}
