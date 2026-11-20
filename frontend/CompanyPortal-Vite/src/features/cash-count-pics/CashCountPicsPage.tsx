import { Button } from "@/components/ui/Button";
import { DataTable } from "@/components/ui/DataTable";
import { PageHeader } from "@/components/ui/PageHeader";
import { useAuthStore } from "@/lib/auth/store";
import { useToast } from "@/lib/hooks/useToast";
import type { ColumnDef } from "@tanstack/react-table";
import { useState } from "react";
import { useCashCountMutation } from "../cash-count-parameters/api";
import { LoadState, RowActions, StatusCell, useSubmit } from "../cash-count-parameters/shared";
import { ExportProgressDialog, isTooManyExports, useExportFlow } from "../exports";
import { PicDialog } from "./PicDialog";
import { PicImportDialog } from "./PicImportDialog";
import { type CashCountPic, PIC_ROLE_LABEL, type PicCreate, type PicUpdate, usePics } from "./api";

/** Mapping PIC Coordinator (per region) / Executor (per area). CC-CWM writes; ADMIN reads. */
export function CashCountPicsPage() {
  const canWrite = useAuthStore((s) => s.user?.role) === "CC-CWM";
  const { toast } = useToast();
  const query = usePics();
  const { run, pendingIds, isPending } = useSubmit(
    useCashCountMutation<PicCreate, PicUpdate>("pics"),
  );
  const [editing, setEditing] = useState<CashCountPic | null | undefined>(undefined);
  const [importing, setImporting] = useState(false);
  const exportFlow = useExportFlow();

  async function download(): Promise<void> {
    const res = await exportFlow.request({ kind: "cash_count_pics", status: "all" });
    if ("error" in res) {
      const e = res.error;
      toast({
        type: "error",
        message: isTooManyExports(e)
          ? "Masih ada ekspor yang berjalan. Tunggu selesai lalu coba lagi."
          : e instanceof Error
            ? e.message
            : "Ekspor gagal.",
      });
    }
  }

  const columns: ColumnDef<CashCountPic, unknown>[] = [
    { id: "role", header: "Peran", cell: ({ row }) => PIC_ROLE_LABEL[row.original.pic_role] },
    { accessorKey: "username", header: "Username" },
    { accessorKey: "full_name", header: "Nama" },
    {
      id: "target",
      header: "Region / Area",
      cell: ({ row }) =>
        row.original.pic_role === "COORDINATOR"
          ? (row.original.region_code ?? "-")
          : `${row.original.area_code ?? "-"} — ${row.original.area_name ?? ""}`,
    },
    {
      id: "status",
      header: "Status",
      cell: ({ row }) => (
        <StatusCell isActive={row.original.is_active} pending={pendingIds.has(row.original.id)} />
      ),
    },
    ...(canWrite
      ? [
          {
            id: "actions",
            header: "Aksi",
            meta: { align: "right" as const },
            cell: ({ row }: { row: { original: CashCountPic } }) => (
              <RowActions
                isActive={row.original.is_active}
                busy={pendingIds.has(row.original.id) || isPending}
                onEdit={() => setEditing(row.original)}
                onToggle={(op) => run({ op, id: row.original.id })}
              />
            ),
          },
        ]
      : []),
  ];

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Perhitungan Kas"
        title="Mapping PIC"
        description="PIC Coordinator per region dan PIC Executor per area. Setiap perubahan diajukan lewat maker-checker dan baru berlaku setelah disetujui."
      />
      <div className="flex flex-wrap justify-end gap-2">
        <Button
          variant="secondary"
          onClick={() => void download()}
          disabled={exportFlow.isRequesting}
        >
          Unduh CSV
        </Button>
        {canWrite && (
          <>
            <Button variant="secondary" onClick={() => setImporting(true)}>
              Unggah CSV
            </Button>
            <Button onClick={() => setEditing(null)}>Tambah Mapping</Button>
          </>
        )}
      </div>
      <LoadState isLoading={query.isLoading} isError={query.isError} label="mapping PIC" />
      {query.data && (
        <DataTable data={query.data} columns={columns} emptyMessage="Belum ada mapping PIC" />
      )}
      {editing !== undefined && (
        <PicDialog
          editing={editing}
          isPending={isPending}
          onClose={() => setEditing(undefined)}
          onSubmit={(m) => run(m, () => setEditing(undefined))}
        />
      )}
      {importing && <PicImportDialog onClose={() => setImporting(false)} />}
      <ExportProgressDialog {...exportFlow.dialogProps} />
    </div>
  );
}
