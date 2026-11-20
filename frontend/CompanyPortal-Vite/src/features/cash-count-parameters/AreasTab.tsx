import { Button } from "@/components/ui/Button";
import { DataTable } from "@/components/ui/DataTable";
import { Dialog } from "@/components/ui/Dialog";
import type { ColumnDef } from "@tanstack/react-table";
import { useState } from "react";
import {
  type AreaCreate,
  type AreaUpdate,
  type CashCountArea,
  useCashCountAreas,
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

/** Parse "JKT1, JKT2" -> unique upper-case codes; the API validates they exist and are free. */
function parseRegions(text: string): string[] {
  return [
    ...new Set(
      text
        .split(/[\s,;]+/)
        .map((s) => s.trim().toUpperCase())
        .filter(Boolean),
    ),
  ];
}

/** Area cash count (AC-5.1-08): code immutable; update replaces the whole region set. */
export function AreasTab({ canWrite }: { canWrite: boolean }) {
  const query = useCashCountAreas();
  const { run, pendingIds, isPending } = useSubmit(
    useCashCountMutation<AreaCreate, AreaUpdate>("areas"),
  );
  const [editing, setEditing] = useState<CashCountArea | null | undefined>(undefined);

  const columns: ColumnDef<CashCountArea, unknown>[] = [
    { accessorKey: "code", header: "Kode" },
    { accessorKey: "name", header: "Nama" },
    {
      id: "regions",
      header: "Region",
      cell: ({ row }) => row.original.region_codes.join(", ") || "-",
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
            cell: ({ row }: { row: { original: CashCountArea } }) => (
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
          <Button onClick={() => setEditing(null)}>Tambah Area</Button>
        </div>
      )}
      <LoadState isLoading={query.isLoading} isError={query.isError} label="area cash count" />
      {query.data && (
        <DataTable data={query.data} columns={columns} emptyMessage="Belum ada area cash count" />
      )}
      {editing !== undefined && (
        <AreaDialog
          editing={editing}
          isPending={isPending}
          onClose={() => setEditing(undefined)}
          onSubmit={(m) => run(m, () => setEditing(undefined))}
        />
      )}
    </div>
  );
}

function AreaDialog({
  editing,
  isPending,
  onClose,
  onSubmit,
}: {
  editing: CashCountArea | null;
  isPending: boolean;
  onClose: () => void;
  onSubmit: (
    m: { op: "create"; body: AreaCreate } | { op: "update"; id: number; body: AreaUpdate },
  ) => void;
}) {
  const [code, setCode] = useState(editing?.code ?? "");
  const [name, setName] = useState(editing?.name ?? "");
  const [regions, setRegions] = useState(editing?.region_codes.join(", ") ?? "");
  const region_codes = parseRegions(regions);
  const canSubmit =
    (editing !== null || /^[A-Za-z0-9_]{2,20}$/.test(code)) &&
    name.trim() !== "" &&
    region_codes.length > 0 &&
    !isPending;

  function submit(e: React.FormEvent): void {
    e.preventDefault();
    if (!canSubmit) return;
    onSubmit(
      editing
        ? { op: "update", id: editing.id, body: { name: name.trim(), region_codes } }
        : { op: "create", body: { code: code.toUpperCase(), name: name.trim(), region_codes } },
    );
  }

  return (
    <Dialog
      open
      onClose={onClose}
      title={editing ? "Ubah Area Cash Count" : "Tambah Area Cash Count"}
    >
      <form onSubmit={submit} className="flex flex-col gap-4">
        <Field label="Kode (2–20 karakter)">
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
        <Field label="Region (pisahkan dengan koma)">
          <input
            value={regions}
            onChange={(e) => setRegions(e.target.value)}
            className={inputClass}
            placeholder="JKT1, JKT2"
          />
        </Field>
        <p className="text-xs text-[var(--n-500)]">
          Satu region hanya boleh milik satu area aktif dan harus dipakai cabang vendor aktif.
        </p>
        <FormActions onClose={onClose} canSubmit={canSubmit} />
      </form>
    </Dialog>
  );
}
