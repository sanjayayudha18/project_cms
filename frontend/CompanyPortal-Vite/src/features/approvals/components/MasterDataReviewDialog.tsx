import { Badge, type BadgeVariant } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { useToast } from "@/lib/hooks/useToast";
import { Ban, CheckCircle2, Pencil, Plus } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import type { MasterDataChange, MasterDataOp } from "../api";
import { useApprove, useMasterDataDetail, useReject } from "../hooks";
import { OP_LABELS, changeSubject, diffFields, entityLabel } from "../lib/changeDiff";

const OP_BADGE: Record<MasterDataOp, { variant: BadgeVariant; icon: LucideIcon }> = {
  create: { variant: "success", icon: Plus },
  update: { variant: "info", icon: Pencil },
  disable: { variant: "warning", icon: Ban },
  enable: { variant: "success", icon: CheckCircle2 },
};

const OP_ORDER: MasterDataOp[] = ["create", "update", "disable", "enable"];

function OpBadge({ op }: { op: MasterDataOp }) {
  const { variant, icon } = OP_BADGE[op];
  return <Badge variant={variant} icon={icon} label={OP_LABELS[op]} />;
}

/** One change as Bidang / Sebelum / Sesudah. "Berubah" is a text label, not just a colour. */
function ChangeDiffTable({ change }: { change: MasterDataChange }) {
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
  const detailQuery = useMasterDataDetail(requestId);
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
          <>
            <div className="flex flex-wrap items-center gap-2">
              <span className="font-medium text-[var(--n-800)]">
                Impor CSV · {detail.total} perubahan
              </span>
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
                  <th className="py-2 pr-4 font-medium">Entitas</th>
                  <th className="py-2 pr-4 font-medium">Data</th>
                  <th className="py-2 font-medium">Operasi</th>
                </tr>
              </thead>
              <tbody>
                {detail.changes.map((c) => (
                  <tr key={c.id} className="border-t border-[var(--n-200)]">
                    <td className="py-2 pr-4">{entityLabel(c.entity_type)}</td>
                    <td className="py-2 pr-4 tabular-nums">{changeSubject(c)}</td>
                    <td className="py-2">
                      <OpBadge op={c.op} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {detail.truncated && (
              <p className="text-xs text-[var(--n-500)]">
                Menampilkan {detail.changes.length} dari {detail.total} perubahan.
              </p>
            )}
          </>
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
