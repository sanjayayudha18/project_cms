import { Button } from "@/components/ui/Button";
import { DataTable } from "@/components/ui/DataTable";
import { formatWibDateTime } from "@/features/eod-monitoring/utils";
import { useQueryClient } from "@tanstack/react-query";
import type { ColumnDef } from "@tanstack/react-table";
import { CircleAlert } from "lucide-react";
import { useEffect, useState } from "react";
import { downloadExportFile, isApiError } from "../api";
import { exportKeys, useMyExports, useRequestExport } from "../hooks";
import type {
  BranchEntity,
  ExportJob,
  ExportRequestParams,
  MasterDataEntity,
  VendorEntity,
} from "../types";
import { ExportStatusBadge } from "./ExportStatusBadge";

const KIND_LABEL: Record<ExportJob["kind"], string> = {
  vendor: "Vendor",
  cabang: "Cabang",
  harga_paket: "Harga paket",
  master_data_global: "Data master",
};
const TICK_MS = 30_000;
const DASH = "—";

/** Request params that reproduce `job`, or null when the job lacks the ids to do so. */
export function jobToParams(job: ExportJob): ExportRequestParams | null {
  const status = job.status_filter;
  const { vendor_id: vendorId, branch_id: branchId } = job;
  switch (job.kind) {
    case "vendor":
      return vendorId === null
        ? null
        : { kind: "vendor", vendorId, entity: job.entity as VendorEntity, status };
    case "cabang":
      return vendorId === null || branchId === null
        ? null
        : { kind: "cabang", vendorId, branchId, entity: job.entity as BranchEntity, status };
    case "harga_paket":
      return vendorId === null
        ? null
        : {
            kind: "harga_paket",
            vendorId,
            branchId: branchId ?? undefined,
            priceLevel: job.price_level ?? undefined,
            status,
          };
    case "master_data_global":
      return { kind: "master_data_global", entity: job.entity as MasterDataEntity, status };
  }
}

/** "9 menit lagi" / "kurang dari 1 menit" / "sudah lewat". */
export function remainingText(expiresAt: string | null, now: number): string {
  if (!expiresAt) return DASH;
  const ms = new Date(expiresAt).getTime() - now;
  if (ms <= 0) return "sudah lewat";
  const min = Math.ceil(ms / 60_000);
  return min <= 1 ? "kurang dari 1 menit" : `${min} menit lagi`;
}

const errText = (e: unknown, fallback: string) =>
  isApiError(e) || e instanceof Error ? e.message : fallback;

export function MyExportsTable() {
  const { data, isLoading, isError, refetch } = useMyExports();
  const qc = useQueryClient();
  const { mutateAsync, isPending } = useRequestExport();
  const [now, setNow] = useState(() => Date.now());
  const [busyId, setBusyId] = useState<number | null>(null);
  const [rowErrors, setRowErrors] = useState<Record<number, string>>({});

  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), TICK_MS);
    return () => clearInterval(t);
  }, []);

  const setError = (id: number, msg: string | null) =>
    setRowErrors((prev) => {
      const { [id]: _drop, ...rest } = prev;
      return msg === null ? rest : { ...rest, [id]: msg };
    });

  const download = async (job: ExportJob) => {
    setError(job.id, null);
    setBusyId(job.id);
    try {
      await downloadExportFile(job.id, job.file_name);
    } catch (e) {
      setError(job.id, errText(e, "Unduhan gagal."));
    } finally {
      setBusyId(null);
      await qc.invalidateQueries({ queryKey: [...exportKeys.all, "mine"] });
    }
  };

  const requestAgain = async (job: ExportJob) => {
    const params = jobToParams(job);
    if (!params) return;
    setError(job.id, null);
    setBusyId(job.id);
    try {
      await mutateAsync(params);
    } catch (e) {
      setError(job.id, errText(e, "Permintaan ekspor gagal."));
    } finally {
      setBusyId(null);
    }
  };

  const disabled = isPending || busyId !== null;
  const columns: ColumnDef<ExportJob, unknown>[] = [
    {
      id: "file_name",
      header: "Nama file",
      cell: ({ row }) => <span className="break-all">{row.original.file_name}</span>,
    },
    {
      id: "kind",
      header: "Jenis",
      cell: ({ row }) => `${KIND_LABEL[row.original.kind]} · ${row.original.entity}`,
    },
    {
      id: "created_at",
      header: "Diminta",
      cell: ({ row }) => (
        <span className="whitespace-nowrap">{formatWibDateTime(row.original.created_at)} WIB</span>
      ),
    },
    {
      id: "status",
      header: "Status",
      cell: ({ row }) => <ExportStatusBadge status={row.original.status} />,
    },
    {
      id: "row_count",
      header: "Baris",
      meta: { align: "right" },
      cell: ({ row }) => (
        <span className="tabular-nums">{row.original.row_count.toLocaleString("id-ID")}</span>
      ),
    },
    {
      id: "expires_at",
      header: "Batas unduh",
      cell: ({ row }) =>
        row.original.status === "succeeded" ? remainingText(row.original.expires_at, now) : DASH,
    },
    {
      id: "downloaded_at",
      header: "Diunduh",
      cell: ({ row }) =>
        row.original.downloaded_at ? `${formatWibDateTime(row.original.downloaded_at)} WIB` : DASH,
    },
    {
      id: "error_message",
      header: "Keterangan",
      cell: ({ row }) => {
        const j = row.original;
        const err = rowErrors[j.id];
        const text = j.status === "failed" ? j.error_message : null;
        if (!text && !err) return DASH;
        return (
          <div className="flex flex-col gap-1">
            {text ? <span>{text}</span> : null}
            {err ? (
              <span role="alert" className="inline-flex items-center gap-1 text-[var(--danger-fg)]">
                <CircleAlert className="h-4 w-4 shrink-0" aria-hidden="true" />
                {err}
              </span>
            ) : null}
          </div>
        );
      },
    },
    {
      id: "actions",
      header: "Aksi",
      cell: ({ row }) => {
        const j = row.original;
        if (j.status === "succeeded") {
          return (
            <div className="flex flex-col items-start gap-1">
              <Button variant="secondary" disabled={disabled} onClick={() => download(j)}>
                Unduh<span className="sr-only"> {j.file_name}</span>
              </Button>
              <span className="text-xs text-[var(--n-500)]">
                Hanya bisa diunduh satu kali, lalu dihapus dari server.
              </span>
            </div>
          );
        }
        if ((j.status === "downloaded" || j.status === "expired") && jobToParams(j)) {
          return (
            <Button variant="secondary" disabled={disabled} onClick={() => requestAgain(j)}>
              Minta ekspor lagi<span className="sr-only"> {j.file_name}</span>
            </Button>
          );
        }
        return DASH;
      },
    },
  ];

  if (isLoading) {
    return (
      // biome-ignore lint/a11y/useSemanticElements: matches other loading states in the app
      <p role="status" className="py-8 text-center text-sm text-[var(--n-500)]">
        Memuat daftar ekspor…
      </p>
    );
  }
  if (isError) {
    return (
      <div role="alert" className="flex flex-col items-center gap-3 py-8 text-center text-sm">
        <span className="inline-flex items-center gap-1 text-[var(--danger-fg)]">
          <CircleAlert className="h-4 w-4" aria-hidden="true" />
          Gagal memuat daftar ekspor.
        </span>
        <Button variant="secondary" onClick={() => refetch()}>
          Coba lagi
        </Button>
      </div>
    );
  }
  return (
    <DataTable
      data={data ?? []}
      columns={columns}
      emptyMessage="Belum ada ekspor. Permintaan ekspor Anda akan muncul di sini."
    />
  );
}
