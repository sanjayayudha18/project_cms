import { Button } from "@/components/ui/Button";
import { DataTable } from "@/components/ui/DataTable";
import { Dialog } from "@/components/ui/Dialog";
import type { ColumnDef } from "@tanstack/react-table";
import { useState } from "react";
import {
  type ChecklistCreate,
  type ChecklistItem,
  type ChecklistUpdate,
  useCashCountMutation,
  useChecklistItems,
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

const CATEGORY_LABEL = { SELECTED_MACHINE: "Selected machine", NON_MACHINE: "Non-mesin" } as const;
const DAY_LABEL = { DAY1: "Hari 1 (mesin)", DAY2: "Hari 2 (vault)" } as const;

/**
 * Checklist Berita Acara (AC-5.1-06/07): empty until the user fills it (no seed).
 * Category cannot change after create; day is required only for SELECTED_MACHINE.
 */
export function ChecklistTab({ canWrite }: { canWrite: boolean }) {
  const query = useChecklistItems();
  const { run, pendingIds, isPending } = useSubmit(
    useCashCountMutation<ChecklistCreate, ChecklistUpdate>("checklist-items"),
  );
  const [editing, setEditing] = useState<ChecklistItem | null | undefined>(undefined);

  const columns: ColumnDef<ChecklistItem, unknown>[] = [
    {
      id: "category",
      header: "Kategori",
      cell: ({ row }) => CATEGORY_LABEL[row.original.category],
    },
    {
      id: "day",
      header: "Hari",
      cell: ({ row }) => (row.original.day ? DAY_LABEL[row.original.day] : "-"),
    },
    { accessorKey: "point_no", header: "No. poin", meta: { align: "right" } },
    { accessorKey: "item_text", header: "Item" },
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
            cell: ({ row }: { row: { original: ChecklistItem } }) => (
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
          <Button onClick={() => setEditing(null)}>Tambah Item</Button>
        </div>
      )}
      <LoadState isLoading={query.isLoading} isError={query.isError} label="checklist BA" />
      {query.data && (
        <DataTable
          data={query.data}
          columns={columns}
          pageSize={25}
          emptyMessage="Checklist BA masih kosong — isi item pertama lewat Tambah Item"
        />
      )}
      {editing !== undefined && (
        <ChecklistDialog
          editing={editing}
          isPending={isPending}
          onClose={() => setEditing(undefined)}
          onSubmit={(m) => run(m, () => setEditing(undefined))}
        />
      )}
    </div>
  );
}

function ChecklistDialog({
  editing,
  isPending,
  onClose,
  onSubmit,
}: {
  editing: ChecklistItem | null;
  isPending: boolean;
  onClose: () => void;
  onSubmit: (
    m:
      | { op: "create"; body: ChecklistCreate }
      | { op: "update"; id: number; body: ChecklistUpdate },
  ) => void;
}) {
  const [category, setCategory] = useState<ChecklistItem["category"]>(
    editing?.category ?? "NON_MACHINE",
  );
  const [day, setDay] = useState<"DAY1" | "DAY2" | "">(editing?.day ?? "");
  const [pointNo, setPointNo] = useState(editing ? String(editing.point_no) : "");
  const [text, setText] = useState(editing?.item_text ?? "");
  const isMachine = category === "SELECTED_MACHINE";
  const point = Number(pointNo);
  const canSubmit =
    Number.isInteger(point) &&
    point > 0 &&
    text.trim() !== "" &&
    (!isMachine || day !== "") &&
    !isPending;

  function submit(e: React.FormEvent): void {
    e.preventDefault();
    if (!canSubmit) return;
    const dayValue = isMachine && day !== "" ? day : null;
    const body = { day: dayValue, point_no: point, item_text: text.trim() };
    onSubmit(
      editing
        ? { op: "update", id: editing.id, body }
        : { op: "create", body: { category, ...body } },
    );
  }

  return (
    <Dialog
      open
      onClose={onClose}
      title={editing ? "Ubah Item Checklist" : "Tambah Item Checklist"}
    >
      <form onSubmit={submit} className="flex flex-col gap-4">
        <Field label="Kategori">
          <select
            value={category}
            disabled={editing !== null}
            onChange={(e) => setCategory(e.target.value as ChecklistItem["category"])}
            className={inputClass}
          >
            <option value="NON_MACHINE">{CATEGORY_LABEL.NON_MACHINE}</option>
            <option value="SELECTED_MACHINE">{CATEGORY_LABEL.SELECTED_MACHINE}</option>
          </select>
        </Field>
        {isMachine && (
          <Field label="Hari">
            <select
              value={day}
              onChange={(e) => setDay(e.target.value as "DAY1" | "DAY2" | "")}
              className={inputClass}
            >
              <option value="">— pilih hari —</option>
              <option value="DAY1">{DAY_LABEL.DAY1}</option>
              <option value="DAY2">{DAY_LABEL.DAY2}</option>
            </select>
          </Field>
        )}
        <Field label="Nomor poin (unik di antara item aktif dengan kategori + hari sama)">
          <input
            type="number"
            min={1}
            value={pointNo}
            onChange={(e) => setPointNo(e.target.value)}
            className={`${inputClass} text-right tabular-nums`}
          />
        </Field>
        <Field label="Teks item">
          <textarea
            value={text}
            maxLength={500}
            rows={3}
            onChange={(e) => setText(e.target.value)}
            className={`${inputClass} py-2`}
          />
        </Field>
        <FormActions onClose={onClose} canSubmit={canSubmit} />
      </form>
    </Dialog>
  );
}
