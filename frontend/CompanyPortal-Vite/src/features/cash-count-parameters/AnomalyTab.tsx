import { Button } from "@/components/ui/Button";
import { DataTable } from "@/components/ui/DataTable";
import { Dialog } from "@/components/ui/Dialog";
import type { ColumnDef } from "@tanstack/react-table";
import { useState } from "react";
import {
  type AnomalyCategory,
  type AnomalyCreate,
  type AnomalyUpdate,
  useAnomalyCategories,
  useCashCountMutation,
} from "./api";
import {
  Field,
  FormActions,
  LoadState,
  RowActions,
  StatusCell,
  inputClass,
  useSubmit,
} from "./shared";

/** Kategori ATM Anomaly (AC-5.1-05): code is immutable after create. */
export function AnomalyTab({ canWrite }: { canWrite: boolean }) {
  const query = useAnomalyCategories();
  const { run, pendingIds, isPending } = useSubmit(
    useCashCountMutation<AnomalyCreate, AnomalyUpdate>("anomaly-categories"),
  );
  // undefined = closed, null = new, row = edit
  const [editing, setEditing] = useState<AnomalyCategory | null | undefined>(undefined);

  const columns: ColumnDef<AnomalyCategory, unknown>[] = [
    { accessorKey: "code", header: "Kode" },
    { accessorKey: "name", header: "Nama" },
    { id: "description", header: "Deskripsi", cell: ({ row }) => row.original.description ?? "-" },
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
            cell: ({ row }: { row: { original: AnomalyCategory } }) => (
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
    <div className="flex flex-col gap-4">
      {canWrite && (
        <div className="flex justify-end">
          <Button onClick={() => setEditing(null)}>Tambah Kategori</Button>
        </div>
      )}
      <LoadState isLoading={query.isLoading} isError={query.isError} label="kategori anomaly" />
      {query.data && (
        <DataTable
          data={query.data}
          columns={columns}
          emptyMessage="Belum ada kategori ATM Anomaly"
        />
      )}
      {editing !== undefined && (
        <AnomalyDialog
          editing={editing}
          isPending={isPending}
          onClose={() => setEditing(undefined)}
          onSubmit={(m) => run(m, () => setEditing(undefined))}
        />
      )}
    </div>
  );
}

function AnomalyDialog({
  editing,
  isPending,
  onClose,
  onSubmit,
}: {
  editing: AnomalyCategory | null;
  isPending: boolean;
  onClose: () => void;
  onSubmit: (
    m: { op: "create"; body: AnomalyCreate } | { op: "update"; id: number; body: AnomalyUpdate },
  ) => void;
}) {
  const [code, setCode] = useState(editing?.code ?? "");
  const [name, setName] = useState(editing?.name ?? "");
  const [description, setDescription] = useState(editing?.description ?? "");
  const canSubmit =
    (editing !== null || /^[A-Z0-9_]{2,20}$/.test(code)) && name.trim() !== "" && !isPending;

  function submit(e: React.FormEvent): void {
    e.preventDefault();
    if (!canSubmit) return;
    const desc = description.trim() || undefined;
    onSubmit(
      editing
        ? { op: "update", id: editing.id, body: { name: name.trim(), description: desc } }
        : { op: "create", body: { code, name: name.trim(), description: desc } },
    );
  }

  return (
    <Dialog
      open
      onClose={onClose}
      title={editing ? "Ubah Kategori Anomaly" : "Tambah Kategori Anomaly"}
    >
      <form onSubmit={submit} className="flex flex-col gap-4">
        <Field label="Kode (A–Z, 0–9, _; 2–20 karakter)">
          <input
            value={code}
            onChange={(e) => setCode(e.target.value)}
            disabled={editing !== null}
            className={inputClass}
          />
        </Field>
        <Field label="Nama">
          <input
            value={name}
            maxLength={100}
            onChange={(e) => setName(e.target.value)}
            className={inputClass}
          />
        </Field>
        <Field label="Deskripsi (opsional)">
          <textarea
            value={description}
            maxLength={500}
            rows={3}
            onChange={(e) => setDescription(e.target.value)}
            className={`${inputClass} py-2`}
          />
        </Field>
        <FormActions onClose={onClose} canSubmit={canSubmit} />
      </form>
    </Dialog>
  );
}
