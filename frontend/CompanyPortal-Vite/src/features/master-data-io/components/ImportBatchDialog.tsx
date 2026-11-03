import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { NoticeBanner } from "@/components/ui/NoticeBanner";
import { PageNumbers } from "@/components/ui/PageNumbers";
import { ChangeDiffTable, OpBadge } from "@/features/approvals/components/MasterDataReviewDialog";
import { changeSubject } from "@/features/approvals/lib/changeDiff";
import { formatWibDateTime } from "@/features/eod-monitoring/utils";
import { useToast } from "@/lib/hooks/useToast";
import { useNavigate, useSearch } from "@tanstack/react-router";
import { AlertTriangle, Ban, ChevronDown, ChevronUp, Info, Loader2, RotateCw } from "lucide-react";
import { Fragment, useCallback, useEffect, useState } from "react";
import {
  IO_ENTITIES,
  type ImportBatchChange,
  type ImportBatchDetail,
  type ImportChangeOp,
} from "../api";
import { BATCH_STATUS, type BatchStatus, canRetry } from "../batchStatus";
import { useImportBatch, useRetryApply } from "../hooks";
import type { MasterDataIoSearchParams } from "../searchSchema";

/** Change_Rows per page; the server pages the rows (Req 6.2). */
export const BATCH_ROWS_PAGE_SIZE = 50;

const OP_ORDER: ImportChangeOp[] = ["create", "update", "disable", "enable"];

const entityLabel = (id: string) => IO_ENTITIES.find((e) => e.id === id)?.label ?? id;

/** "Gagal" for the row that rolled the batch back, "Dibatalkan" for rows cancelled with it. */
function RowStatus({ change }: { change: ImportBatchChange }) {
  if (change.failed) {
    return (
      <>
        <Badge variant="danger" icon={AlertTriangle} label="Gagal" />
        {change.error ? (
          <div className="mt-1 break-words text-xs text-[var(--danger-fg)]">{change.error}</div>
        ) : null}
      </>
    );
  }
  if (change.status === "stale") {
    return <Badge variant="neutral" icon={Ban} label="Dibatalkan" />;
  }
  const st = BATCH_STATUS[change.status as BatchStatus];
  return st ? (
    <Badge variant={st.variant} icon={st.icon} label={st.label} />
  ) : (
    <span className="text-[var(--n-600)]">{change.status}</span>
  );
}

/**
 * Cause of a stale batch. The server picks the failed row across all pages
 * (`failure_reason`); the row's data is named only when it is on this page.
 * Old batches record no cause and never fall back to the technical batch `error`.
 */
function staleCause(detail: ImportBatchDetail): string {
  if (!detail.failure_reason) return "Penyebab tidak tercatat";
  const failed = detail.changes.find((c) => c.failed);
  return failed
    ? `Baris ${changeSubject(failed)} gagal: ${detail.failure_reason}`
    : detail.failure_reason;
}

interface BodyProps {
  detail: ImportBatchDetail;
  page: number;
  onPageChange: (page: number) => void;
}

function BatchBody({ detail, page, onPageChange }: BodyProps) {
  const { toast } = useToast();
  const retry = useRetryApply();
  const [openIds, setOpenIds] = useState<ReadonlySet<number>>(new Set());
  const st = BATCH_STATUS[detail.status];
  const totalPages = Math.max(1, Math.ceil(detail.total / BATCH_ROWS_PAGE_SIZE));
  const first = detail.total === 0 ? 0 : (page - 1) * BATCH_ROWS_PAGE_SIZE + 1;
  const last = (page - 1) * BATCH_ROWS_PAGE_SIZE + detail.changes.length;
  const approvalId = detail.approval_request_id;
  const canRetryNow = canRetry(detail.status) && approvalId !== null;

  function toggle(id: number): void {
    setOpenIds((prev) => {
      const next = new Set(prev);
      if (!next.delete(id)) next.add(id);
      return next;
    });
  }

  function onRetry(): void {
    if (approvalId === null || retry.isPending) return;
    // useRetryApply refreshes the batch detail and list on success and on error.
    retry.mutate(approvalId, {
      onSuccess: () => toast({ type: "success", message: "Batch berhasil diterapkan" }),
      onError: (err) => toast({ type: "error", message: err.message }),
    });
  }

  return (
    <>
      <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm sm:grid-cols-[auto_1fr_auto_1fr]">
        <dt className="text-[var(--n-500)]">File</dt>
        <dd className="break-words text-[var(--n-900)]">{detail.file_name ?? "—"}</dd>
        <dt className="text-[var(--n-500)]">Status</dt>
        <dd>
          <Badge variant={st.variant} icon={st.icon} label={st.label} />
        </dd>
        <dt className="text-[var(--n-500)]">Diajukan</dt>
        <dd className="tabular-nums text-[var(--n-900)]">
          {formatWibDateTime(detail.created_at)} · {detail.maker_name}
        </dd>
        <dt className="text-[var(--n-500)]">Diputuskan</dt>
        <dd className="tabular-nums text-[var(--n-900)]">
          {detail.decided_at
            ? `${formatWibDateTime(detail.decided_at)}${detail.decided_by_name ? ` · ${detail.decided_by_name}` : ""}`
            : "—"}
        </dd>
      </dl>

      <div className="flex flex-wrap items-center gap-2">
        {OP_ORDER.filter((op) => (detail.counts[op] ?? 0) > 0).map((op) => (
          <span key={op} className="inline-flex items-center gap-1 text-sm">
            <OpBadge op={op} />
            <span className="tabular-nums">{detail.counts[op]}</span>
          </span>
        ))}
      </div>

      {detail.status === "stale" && (
        <NoticeBanner
          variant="danger"
          icon={AlertTriangle}
          title="Tidak ada data yang diterapkan"
          description={`${staleCause(detail)}. File yang sama boleh diajukan ulang setelah diperbaiki.`}
        />
      )}

      {canRetryNow && (
        <NoticeBanner
          variant="warning"
          icon={AlertTriangle}
          title="Sudah disetujui, tetapi penyimpanan gagal"
          description={`${detail.error ? `Kesalahan: ${detail.error}. ` : ""}Data belum berubah. Coba terapkan lagi tanpa persetujuan ulang.`}
        />
      )}

      {approvalId === null && (
        <NoticeBanner
          variant="info"
          icon={Info}
          title="Pengajuan belum lengkap"
          description="Pengajuan belum lengkap. Ajukan ulang file yang sama untuk menyelesaikannya."
        />
      )}

      <div className="overflow-x-auto rounded-lg border border-[var(--n-200)]">
        <table className="w-full border-collapse text-sm">
          <thead className="border-b border-[var(--n-200)] bg-[var(--n-50)]">
            <tr className="text-left text-xs font-semibold text-[var(--n-600)]">
              <th className="px-3 py-2">Data</th>
              <th className="px-3 py-2">Operasi</th>
              <th className="px-3 py-2">Status</th>
              <th className="px-3 py-2">Perubahan</th>
            </tr>
          </thead>
          <tbody>
            {detail.changes.map((c) => {
              const isOpen = openIds.has(c.id);
              const panelId = `batch-change-diff-${c.id}`;
              return (
                <Fragment key={c.id}>
                  <tr className="border-t border-[var(--n-100)] align-top">
                    <td className="break-words px-3 py-2 tabular-nums">{changeSubject(c)}</td>
                    <td className="px-3 py-2">
                      <OpBadge op={c.op} />
                    </td>
                    <td className="px-3 py-2">
                      <RowStatus change={c} />
                    </td>
                    <td className="px-3 py-2">
                      <button
                        type="button"
                        aria-expanded={isOpen}
                        aria-controls={panelId}
                        onClick={() => toggle(c.id)}
                        className="inline-flex min-h-[44px] items-center gap-1 rounded-[var(--radius-md)] px-2 text-[var(--n-700)] underline-offset-2 outline-none hover:underline focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
                      >
                        {isOpen ? "Sembunyikan" : "Lihat perubahan"}
                        {isOpen ? (
                          <ChevronUp aria-hidden="true" className="size-4" />
                        ) : (
                          <ChevronDown aria-hidden="true" className="size-4" />
                        )}
                      </button>
                    </td>
                  </tr>
                  {isOpen && (
                    <tr id={panelId}>
                      <td colSpan={4} className="bg-[var(--n-50)] px-3 pb-3">
                        <ChangeDiffTable change={c} />
                      </td>
                    </tr>
                  )}
                </Fragment>
              );
            })}
          </tbody>
        </table>
      </div>

      <nav
        aria-label="Halaman perubahan batch"
        className="flex flex-wrap items-center justify-between gap-2 text-sm text-[var(--n-600)]"
      >
        <span className="tabular-nums">
          Menampilkan {first}-{last} dari {detail.total} baris
        </span>
        {totalPages > 1 && (
          <div className="flex items-center gap-1">
            <PageNumbers current={page} total={totalPages} onChange={onPageChange} />
          </div>
        )}
      </nav>

      {canRetryNow && (
        <div className="flex justify-end">
          <Button onClick={onRetry} disabled={retry.isPending}>
            {retry.isPending ? (
              <Loader2 className="mr-2 size-4 animate-spin" aria-hidden="true" />
            ) : (
              <RotateCw className="mr-2 size-4" aria-hidden="true" />
            )}
            Coba terapkan lagi
          </Button>
        </div>
      )}
    </>
  );
}

/**
 * Detail of one import batch, opened from `?batch=ID` (master-data-import-revamp Req 6).
 * Closing, Esc, and a 404 all drop `batch` from the URL and keep the other params.
 */
export function ImportBatchDialog() {
  const search = useSearch({ strict: false }) as MasterDataIoSearchParams;
  const navigate = useNavigate();
  const { toast } = useToast();
  const batchId = Number.isInteger(search.batch) ? (search.batch as number) : null;
  // The page belongs to one batch; opening another starts at page 1.
  const [pageState, setPageState] = useState({ batchId, page: 1 });
  const page = pageState.batchId === batchId ? pageState.page : 1;
  const query = useImportBatch(batchId, page, BATCH_ROWS_PAGE_SIZE);

  const close = useCallback(() => {
    const { batch: _batch, ...rest } = search;
    const clean = Object.fromEntries(
      Object.entries(rest).filter(([, v]) => v !== undefined && v !== false),
    );
    navigate({ to: ".", search: clean });
  }, [search, navigate]);

  const notFound = query.isError && query.error.status === 404;
  useEffect(() => {
    if (!notFound) return;
    toast({ type: "error", message: "Batch tidak ditemukan" });
    close();
  }, [notFound, toast, close]);

  if (batchId === null) return null;
  const detail = query.data;

  return (
    <Dialog
      open
      onClose={close}
      title={detail ? `Batch #${detail.id} · ${entityLabel(detail.entity)}` : `Batch #${batchId}`}
      className="max-w-3xl"
    >
      <div className="flex max-h-[70vh] flex-col gap-4 overflow-y-auto">
        {query.isLoading && (
          <output className="flex items-center gap-2 text-sm text-[var(--n-600)]">
            <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
            Memuat batch...
          </output>
        )}
        {query.isError && !notFound && (
          <div role="alert" className="flex items-center gap-3 text-sm text-[var(--danger-fg)]">
            <AlertTriangle className="h-4 w-4 shrink-0" aria-hidden="true" />
            Batch gagal dimuat.
            <Button variant="secondary" onClick={() => query.refetch()}>
              Coba lagi
            </Button>
          </div>
        )}
        {detail && (
          <BatchBody
            detail={detail}
            page={page}
            onPageChange={(p) => setPageState({ batchId, page: p })}
          />
        )}
        <div className="flex justify-end">
          <Button variant="secondary" onClick={close}>
            Tutup
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
