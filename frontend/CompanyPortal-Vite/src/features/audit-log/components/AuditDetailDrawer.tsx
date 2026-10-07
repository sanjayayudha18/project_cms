import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { formatWibDateTimeSec } from "@/features/eod-monitoring/utils";
import { useAuditLogDetail } from "../hooks/useAuditQueries";
import { ActionBadge } from "./AuditLogTable";
import { BeforeAfterDiff } from "./BeforeAfterDiff";

interface AuditDetailDrawerProps {
  /** audit_logs id; null = closed. */
  selectedId: number | null;
  onClose: () => void;
}

/**
 * Detail view for one audit entry (Req 7). Built on Dialog for the focus trap,
 * Escape/outside close and focus return; it is a centered modal rather than a
 * slide-in panel. A failed fetch shows an inline error + retry without closing.
 */
export function AuditDetailDrawer({ selectedId, onClose }: AuditDetailDrawerProps) {
  const query = useAuditLogDetail(selectedId);
  const detail = query.data;

  return (
    <Dialog
      open={selectedId !== null}
      onClose={onClose}
      title="Detail log audit"
      className="max-w-2xl"
    >
      <div className="flex max-h-[70vh] flex-col gap-4 overflow-y-auto">
        {query.isLoading && (
          <div aria-label="Memuat detail" className="flex flex-col gap-2">
            <div className="h-6 w-2/3 animate-pulse rounded bg-[var(--n-200)]" />
            <div className="h-24 w-full animate-pulse rounded bg-[var(--n-200)]" />
          </div>
        )}

        {query.isError && (
          <div role="alert" className="flex flex-col items-start gap-2">
            <p className="text-sm text-[var(--danger-fg)]">
              Gagal memuat detail: {query.error.message}
            </p>
            <Button variant="secondary" onClick={() => query.refetch()}>
              Coba Lagi
            </Button>
          </div>
        )}

        {detail && (
          <>
            <dl className="grid grid-cols-[max-content_1fr] gap-x-6 gap-y-2 text-sm">
              <dt className="text-[var(--n-500)]">Waktu (WIB)</dt>
              <dd className="tabular-nums">{formatWibDateTimeSec(detail.created_at)}</dd>
              <dt className="text-[var(--n-500)]">Aktor</dt>
              <dd className="tabular-nums">#{detail.actor_id}</dd>
              <dt className="text-[var(--n-500)]">Aksi</dt>
              <dd>
                <ActionBadge action={detail.action} />
              </dd>
              <dt className="text-[var(--n-500)]">Entitas</dt>
              <dd>
                {detail.entity_type} <span className="tabular-nums">#{detail.entity_id}</span>
              </dd>
              <dt className="text-[var(--n-500)]">Alamat IP</dt>
              <dd className="tabular-nums">{detail.ip ?? "-"}</dd>
            </dl>
            <BeforeAfterDiff before={detail.before} after={detail.after} />
          </>
        )}
      </div>
    </Dialog>
  );
}
