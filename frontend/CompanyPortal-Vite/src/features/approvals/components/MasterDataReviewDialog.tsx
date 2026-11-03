import { Badge, type BadgeVariant } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { PageNumbers } from "@/components/ui/PageNumbers";
import { useToast } from "@/lib/hooks/useToast";
import { Ban, CheckCircle2, ChevronDown, ChevronUp, Pencil, Plus } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { Fragment, useState } from "react";
import {
  MASTER_DATA_PAGE_SIZE,
  type MasterDataChange,
  type MasterDataDetail,
  type MasterDataOp,
} from "../api";
import { useApprove, useMasterDataDetail, useReject } from "../hooks";
import { OP_LABELS, changeSubject, diffFields, entityLabel } from "../lib/changeDiff";

const OP_BADGE: Record<MasterDataOp, { variant: BadgeVariant; icon: LucideIcon }> = {
  create: { variant: "success", icon: Plus },
  update: { variant: "info", icon: Pencil },
  disable: { variant: "warning", icon: Ban },
  enable: { variant: "success", icon: CheckCircle2 },
};

const OP_ORDER: MasterDataOp[] = ["create", "update", "disable", "enable"];

export function OpBadge({ op }: { op: MasterDataOp }) {
  const { variant, icon } = OP_BADGE[op];
  return <Badge variant={variant} icon={icon} label={OP_LABELS[op]} />;
}

/** One change as Bidang / Sebelum / Sesudah. "Berubah" is a text label, not just a colour. */
export function ChangeDiffTable({
  change,
}: {
  change: Pick<MasterDataChange, "op" | "payload" | "before">;
}) {
  const fields = diffFields(change);
  return (
    <table className="w-full border-collapse text-sm">
      <thead>
        <tr className="text-left text-[var(--n-500)]">
          <th className="py-2 pr-4 font-medium">Bidang</th>
          <th className="py-2 pr-4 font-medium">Sebelum</th>
          <th className="py-2 font-medium">Sesudah</th>
        </tr>
      </thead>
      <tbody>
        {fields.map((f) => (
          <tr key={f.field} className="border-t border-[var(--n-200)] align-top">
            <th scope="row" className="py-2 pr-4 text-left font-medium text-[var(--n-700)]">
              {f.label}
            </th>
            <td className="py-2 pr-4 text-[var(--n-600)]">{f.before}</td>
            <td className="py-2 text-[var(--n-900)]">
              <span className="break-words">{f.after}</span>
              {f.changed && change.op === "update" && (
                <span className="ml-2">
                  <Badge variant="info" icon={Pencil} label="Berubah" />
                </span>
              )}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

interface BatchReviewProps {
  detail: MasterDataDetail;
  page: number;
  onPageChange: (page: number) => void;
}

/** CSV import batch: header, per-operation counts, rows with an inline diff, 50 per page. */
function BatchReview({ detail, page, onPageChange }: BatchReviewProps) {
  const [openIds, setOpenIds] = useState<ReadonlySet<number>>(new Set());
  const { batch } = detail;
  const totalPages = Math.max(1, Math.ceil(detail.total / MASTER_DATA_PAGE_SIZE));
  const first = detail.total === 0 ? 0 : (page - 1) * MASTER_DATA_PAGE_SIZE + 1;
  const last = (page - 1) * MASTER_DATA_PAGE_SIZE + detail.changes.length;

  function toggle(id: number): void {
    setOpenIds((prev) => {
      const next = new Set(prev);
      if (!next.delete(id)) next.add(id);
      return next;
    });
  }

  return (
    <>
      <div>
        <div className="font-semibold text-[var(--n-900)]">
          Impor CSV
          {batch ? ` · ${entityLabel(batch.entity)}` : ""}
          {batch?.file_name ? ` · ${batch.file_name}` : ""}
        </div>
        <div className="text-sm text-[var(--n-500)]">
          {batch ? `Diajukan oleh ${batch.maker_name} · ` : ""}
          <span className="tabular-nums">{detail.total}</span> perubahan
        </div>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        {OP_ORDER.filter((op) => (detail.counts[op] ?? 0) > 0).map((op) => (
          <span key={op} className="inline-flex items-center gap-1 text-sm">
            <OpBadge op={op} />
            <span className="tabular-nums">{detail.counts[op]}</span>
          </span>
        ))}
      </div>
      <p className="text-xs text-[var(--n-500)]">
        Semua baris diterapkan sekaligus saat disetujui; jika satu baris gagal, tidak ada yang
        diterapkan.
      </p>
      <table className="w-full border-collapse text-sm">
        <thead>
          <tr className="text-left text-[var(--n-500)]">
            <th className="py-2 pr-4 font-medium">Data</th>
            <th className="py-2 pr-4 font-medium">Operasi</th>
            <th className="py-2 font-medium">Perubahan</th>
          </tr>
        </thead>
        <tbody>
          {detail.changes.map((c) => {
            const isOpen = openIds.has(c.id);
            const panelId = `change-diff-${c.id}`;
            return (
              <Fragment key={c.id}>
                <tr className="border-t border-[var(--n-200)]">
                  <td className="py-2 pr-4 tabular-nums">{changeSubject(c)}</td>
                  <td className="py-2 pr-4">
                    <OpBadge op={c.op} />
                  </td>
                  <td className="py-2">
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
                    <td colSpan={3} className="bg-[var(--n-50)] px-3 pb-3">
                      <ChangeDiffTable change={c} />
                    </td>
                  </tr>
                )}
              </Fragment>
            );
          })}
        </tbody>
      </table>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="text-sm tabular-nums text-[var(--n-600)]">
          Menampilkan {first}-{last} dari {detail.total} perubahan
        </span>
        {totalPages > 1 && (
          <nav aria-label="Halaman perubahan" className="flex items-center gap-1">
            <Button variant="secondary" disabled={page <= 1} onClick={() => onPageChange(page - 1)}>
              Sebelumnya
            </Button>
            <PageNumbers current={page} total={totalPages} onChange={onPageChange} />
            <Button
              variant="secondary"
              disabled={page >= totalPages}
              onClick={() => onPageChange(page + 1)}
            >
              Berikutnya
            </Button>
          </nav>
        )}
      </div>
    </>
  );
}

interface MasterDataReviewDialogProps {
  /** approval_requests id (the inbox item's request_id); null = closed. */
  requestId: number | null;
  onClose: () => void;
}

/**
 * Review of a master-data approval request: what exactly changes (a field diff,
 * or the rows + per-operation counts of a CSV import batch), then Setujui / Tolak.
 * The checker decides here; nothing is applied until Setujui.
 */
export function MasterDataReviewDialog({ requestId, onClose }: MasterDataReviewDialogProps) {
  const { toast } = useToast();
  // The page belongs to one request; opening another request starts at page 1.
  const [pageState, setPageState] = useState<{ requestId: number | null; page: number }>({
    requestId,
    page: 1,
  });
  const page = pageState.requestId === requestId ? pageState.page : 1;
  const detailQuery = useMasterDataDetail(requestId, page);
  const approve = useApprove();
  const reject = useReject();
  const detail = detailQuery.data;
  const isBusy = approve.isPending || reject.isPending;

  function decide(kind: "approve" | "reject"): void {
    if (requestId === null) return;
    const mutation = kind === "approve" ? approve : reject;
    mutation.mutate(requestId, {
      onSuccess: () => {
        toast({
          type: "success",
          message: kind === "approve" ? "Perubahan disetujui dan diterapkan" : "Perubahan ditolak",
        });
        onClose();
      },
      onError: (err) => toast({ type: "error", message: err.message }),
    });
  }

  const isBatch = detail?.batch_id != null;

  return (
    <Dialog open={requestId !== null} onClose={onClose} title="Tinjau perubahan master data">
      <div className="flex max-h-[70vh] flex-col gap-4 overflow-y-auto">
        {detailQuery.isLoading && <p className="text-sm text-[var(--n-500)]">Memuat perubahan…</p>}

        {detailQuery.isError && (
          <p role="alert" className="text-sm text-[var(--danger-fg)]">
            Gagal memuat perubahan: {detailQuery.error.message}
          </p>
        )}

        {detail && !isBatch && detail.changes[0] && (
          <>
            <div className="flex flex-wrap items-center gap-2">
              <span className="font-medium text-[var(--n-800)]">
                {entityLabel(detail.changes[0].entity_type)} · {changeSubject(detail.changes[0])}
              </span>
              <OpBadge op={detail.changes[0].op} />
            </div>
            <ChangeDiffTable change={detail.changes[0]} />
          </>
        )}

        {detail && isBatch && (
          <BatchReview
            detail={detail}
            page={page}
            onPageChange={(p) => setPageState({ requestId, page: p })}
          />
        )}

        <div className="flex justify-end gap-2 pt-2">
          <Button variant="secondary" onClick={() => decide("reject")} disabled={isBusy || !detail}>
            Tolak
          </Button>
          <Button onClick={() => decide("approve")} disabled={isBusy || !detail}>
            Setujui
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
