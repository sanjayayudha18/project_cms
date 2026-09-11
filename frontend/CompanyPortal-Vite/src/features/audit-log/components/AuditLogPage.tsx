/**
 * Audit Log Viewer page — Task 6 scope only: wires the route to the list
 * query with fixed default params so the route/hook/guard chain is real and
 * testable end to end. Task 7 replaces this body with AuditFilterBar +
 * AuditLogTable (URL-synced filters/pagination); Task 8 adds the detail
 * drawer. Intentionally minimal until then.
 */

import { PageHeader } from "@/components/ui/PageHeader";
import { useAuditLogList } from "../hooks/useAuditQueries";
import { EMPTY_AUDIT_LOG_FILTERS } from "../types";

const DEFAULT_PAGE = 1;
const DEFAULT_PAGE_SIZE = 25;

export function AuditLogPage() {
  const { data, isLoading, isError } = useAuditLogList(
    EMPTY_AUDIT_LOG_FILTERS,
    DEFAULT_PAGE,
    DEFAULT_PAGE_SIZE,
  );

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Admin"
        title="Log Audit"
        description="Riwayat aktivitas sistem: siapa melakukan apa, kapan, dan perubahan datanya."
      />

      {isLoading && <p className="text-sm text-[var(--n-500)]">Memuat log audit…</p>}
      {isError && (
        <p className="text-sm text-[var(--red-600)]">Gagal memuat log audit. Coba lagi.</p>
      )}
      {data && (
        <p className="text-sm text-[var(--n-500)]">
          {data.total} entri log audit ditemukan (halaman {data.page}).
        </p>
      )}
    </div>
  );
}
