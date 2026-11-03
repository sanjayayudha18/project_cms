import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Link } from "@tanstack/react-router";
import { CheckCircle2, Info } from "lucide-react";
import type { ImportConfirmResult } from "../api";
import { BATCH_STATUS } from "../batchStatus";

interface ImportResultPanelProps {
  result: ImportConfirmResult;
  onReset: () => void;
}

/**
 * Req 4.3 / 4.4. A new batch is always pending. An `existing` batch may have been decided
 * since (the confirm response carries no status), so only the history shows its real status.
 */
export function ImportResultPanel({ result, onReset }: ImportResultPanelProps) {
  const pending = BATCH_STATUS.pending;
  const Icon = result.existing ? Info : CheckCircle2;
  return (
    <output className="flex flex-col gap-3 rounded border border-[var(--n-200)] p-4">
      <p className="flex items-center gap-2 text-sm font-medium text-[var(--n-900)]">
        <Icon className="h-4 w-4" aria-hidden="true" />
        {result.existing
          ? "File ini sudah diajukan sebelumnya. Tidak ada batch baru yang dibuat."
          : "Impor diajukan dan menunggu persetujuan."}
      </p>
      <dl className="flex flex-wrap items-center gap-x-6 gap-y-2 text-sm">
        <div>
          <dt className="text-xs text-[var(--n-600)]">Nomor batch</dt>
          <dd className="font-semibold tabular-nums">#{result.batch_id}</dd>
        </div>
        <div>
          <dt className="text-xs text-[var(--n-600)]">Jumlah baris</dt>
          <dd className="font-semibold tabular-nums">{result.rows.toLocaleString("id-ID")}</dd>
        </div>
        {!result.existing && (
          <div>
            <dt className="text-xs text-[var(--n-600)]">Status</dt>
            <dd>
              <Badge variant={pending.variant} icon={pending.icon} label={pending.label} />
            </dd>
          </div>
        )}
      </dl>
      {result.existing && (
        <p className="text-xs text-[var(--n-600)]">
          Status terkini batch ini ada di riwayat impor.
        </p>
      )}
      <div className="flex flex-wrap gap-3">
        <Link
          to="/settings/admin/master-data-io"
          search={{ tab: "history", batch: result.batch_id }}
          className="text-sm font-medium text-[var(--red-600)] underline"
        >
          Lihat di riwayat
        </Link>
        <Button variant="secondary" onClick={onReset}>
          Impor file lain
        </Button>
      </div>
    </output>
  );
}
