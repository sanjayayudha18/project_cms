import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { DataTable } from "@/components/ui/DataTable";
import { EmptyState } from "@/components/ui/EmptyState";
import { NoticeBanner } from "@/components/ui/NoticeBanner";
import type { ColumnDef } from "@tanstack/react-table";
import { CircleAlert, CircleCheck, CircleOff, Loader2, TableProperties } from "lucide-react";
import { useMemo } from "react";
import type { MasterDataset } from "../datasets";
import type { ExportPreview } from "../types";

type Row = string[];

interface PreviewTableProps {
  dataset: MasterDataset;
  /** Last loaded page; kept while the next one loads. */
  data: ExportPreview | undefined;
  isLoading: boolean;
  isFetching: boolean;
  isError: boolean;
  onRetry: () => void;
  pageNumber: number;
  canPrev: boolean;
  canNext: boolean;
  onPrev: () => void;
  onNext: () => void;
}

function buildColumns(dataset: MasterDataset, columns: string[]): ColumnDef<Row, string>[] {
  return columns.map((name, i) => {
    const numeric = dataset.numericColumns.includes(name);
    const money = dataset.moneyColumns.includes(name);
    return {
      id: name,
      header: name,
      accessorFn: (row) => row[i] ?? "",
      enableSorting: false,
      meta: numeric ? { align: "right" } : undefined,
      cell: ({ getValue }) => {
        const v = getValue();
        if (name === "is_active") {
          return v === "true" ? (
            <Badge variant="success" icon={CircleCheck} label="Aktif" />
          ) : (
            <Badge variant="neutral" icon={CircleOff} label="Nonaktif" />
          );
        }
        return money ? <span className="font-mono">{v}</span> : v;
      },
    };
  });
}

/** One page of the table behind an export; column names are the CSV headers as-is (Req 2.1). */
export function PreviewTable({
  dataset,
  data,
  isLoading,
  isFetching,
  isError,
  onRetry,
  pageNumber,
  canPrev,
  canNext,
  onPrev,
  onNext,
}: PreviewTableProps) {
  const columns = useMemo(() => buildColumns(dataset, data?.columns ?? []), [dataset, data]);

  if (isLoading) {
    return (
      <output className="flex items-center gap-2 py-8 text-sm text-[var(--n-600)]">
        <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
        Memuat pratinjau…
      </output>
    );
  }

  return (
    <section aria-label={`Pratinjau ${dataset.label}`} className="flex flex-col gap-3">
      {isError && (
        <div className="flex flex-col items-start gap-2">
          <NoticeBanner
            variant="danger"
            icon={CircleAlert}
            title="Pratinjau gagal dimuat"
            description="Data tidak bisa diambil saat ini. Periksa koneksi lalu coba lagi."
          />
          <Button variant="secondary" onClick={onRetry}>
            Coba lagi
          </Button>
        </div>
      )}
      {data && data.rows.length === 0 && !isError && (
        <EmptyState icon={TableProperties} message="Tidak ada data yang cocok dengan filter ini." />
      )}
      {data && data.rows.length > 0 && (
        <>
          <div className="flex items-center justify-between gap-3">
            <h2 className="text-sm font-semibold text-[var(--n-800)]">Pratinjau {dataset.label}</h2>
            {isFetching && (
              <output className="flex items-center gap-2 text-sm text-[var(--n-600)]">
                <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
                Memuat…
              </output>
            )}
          </div>
          <div aria-busy={isFetching} className={isFetching ? "opacity-60" : undefined}>
            <DataTable data={data.rows} columns={columns} />
          </div>
          <nav aria-label="Halaman pratinjau" className="flex items-center justify-between gap-3">
            <Button variant="secondary" disabled={!canPrev || isFetching} onClick={onPrev}>
              Sebelumnya
            </Button>
            <span className="text-sm tabular-nums text-[var(--n-600)]">Halaman {pageNumber}</span>
            <Button variant="secondary" disabled={!canNext || isFetching} onClick={onNext}>
              Berikutnya
            </Button>
          </nav>
        </>
      )}
    </section>
  );
}
