import { Button } from "@/components/ui/Button";
import { EmptyState } from "@/components/ui/EmptyState";
import { PageHeader } from "@/components/ui/PageHeader";
import { AlertCircle } from "lucide-react";
import { useState } from "react";
import { useAuditLogList } from "../hooks/useAuditQueries";
import { useAuditLogUrlState } from "../useAuditLogUrlState";
import { AuditDetailDrawer } from "./AuditDetailDrawer";
import { AuditFilterBar } from "./AuditFilterBar";
import { AuditLogTable } from "./AuditLogTable";

const SKELETON_ROW_KEYS = ["row-1", "row-2", "row-3", "row-4", "row-5"];

/** Audit Log Viewer (Req 5-8): URL-synced filters + pagination, row -> detail. */
export function AuditLogPage() {
  const { params, setFilters, setPage, resetFilters } = useAuditLogUrlState();
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const query = useAuditLogList(params.filters, params.page, params.page_size);

  const rows = query.data?.data ?? [];
  const total = query.data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / params.page_size));

  return (
    <div className="flex flex-col gap-6 p-6">
      <PageHeader
        eyebrow="Admin"
        title="Log Audit"
        description="Riwayat aktivitas sistem: siapa melakukan apa, kapan, dan perubahan datanya."
      />

      <AuditFilterBar
        filters={params.filters}
        onFiltersChange={setFilters}
        onReset={resetFilters}
      />

      {query.isLoading && (
        <div className="flex flex-col gap-2">
          {SKELETON_ROW_KEYS.map((key) => (
            <div key={key} className="h-10 w-full animate-pulse rounded bg-[var(--n-200)]" />
          ))}
        </div>
      )}

      {!query.isLoading && query.isError && (
        <div className="flex flex-col items-center gap-3 py-12">
          <AlertCircle className="h-10 w-10 text-[var(--danger-fg)]" aria-hidden="true" />
          <p className="text-sm text-[var(--n-700)]">Gagal memuat log audit</p>
          <Button variant="secondary" onClick={() => query.refetch()}>
            Coba Lagi
          </Button>
        </div>
      )}

      {!query.isLoading && !query.isError && rows.length === 0 && (
        <EmptyState message="Tidak ada log audit" />
      )}

      {!query.isLoading && !query.isError && rows.length > 0 && (
        <AuditLogTable rows={rows} onSelect={setSelectedId} />
      )}

      <div className="flex flex-wrap items-center justify-between gap-4 text-sm text-[var(--n-600)]">
        <span className="tabular-nums">
          Halaman {params.page} dari {totalPages} ({total} entri)
        </span>
        <div className="flex items-center gap-2">
          <Button
            variant="secondary"
            disabled={params.page <= 1}
            onClick={() => setPage(params.page - 1)}
          >
            Sebelumnya
          </Button>
          <Button
            variant="secondary"
            disabled={params.page >= totalPages}
            onClick={() => setPage(params.page + 1)}
          >
            Berikutnya
          </Button>
        </div>
      </div>

      <AuditDetailDrawer selectedId={selectedId} onClose={() => setSelectedId(null)} />
    </div>
  );
}
