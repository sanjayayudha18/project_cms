import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { DataTable } from "@/components/ui/DataTable";
import { Dialog } from "@/components/ui/Dialog";
import { useToast } from "@/lib/hooks/useToast";
import type { ColumnDef } from "@tanstack/react-table";
import { CheckCircle, XCircle } from "lucide-react";
import { useState } from "react";
import { PendingApprovalBadge } from "../../master-data/PendingApprovalBadge";
import { pendingApprovalMessage } from "../../master-data/changeRequest";
import { usePendingCreates, usePendingEntityIds } from "../../master-data/pending";
import {
  type DsrLocationMap,
  type MapMutation,
  useDsrLocationMapMutation,
  useDsrLocationMaps,
  useUnmappedDsrLocations,
} from "../dsrLocationMaps";
import { useVendorVaults } from "../hooks";
import { PendingCreatesList } from "./PendingCreatesList";

const inputClass =
  "min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)] disabled:bg-[var(--n-100)] disabled:text-[var(--n-500)]";

/** Dialog state: a new label (optionally prefilled from "belum terpetakan") or an existing map. */
type Editing = { label: string; map: DsrLocationMap | null };

/**
 * "Mapping DSR" tab (cit-acm-plan FR2): which vault each DSR block label of
 * this vendor belongs to. Every change is maker-checker (202, pending approval).
 */
export function DsrLocationMapsPanel({ vendorId }: { vendorId: number }) {
  const { toast } = useToast();
  const mapsQuery = useDsrLocationMaps(vendorId);
  const unmappedQuery = useUnmappedDsrLocations(vendorId);
  const mutation = useDsrLocationMapMutation(vendorId);
  const pendingIds = usePendingEntityIds("dsr_location_map");
  const pendingCreates = usePendingCreates("dsr_location_map")
    .filter((c) => c.payload.vendor_id === vendorId)
    .map((c) => ({ id: c.id, label: String(c.payload.dsr_location) }));
  const [editing, setEditing] = useState<Editing | null>(null);

  function run(m: MapMutation, onDone?: () => void): void {
    mutation.mutate(m, {
      onSuccess: (res) => {
        toast({ type: "success", message: pendingApprovalMessage(res) });
        onDone?.();
      },
      onError: (err) => toast({ type: "error", message: err.message }),
    });
  }

  const columns: ColumnDef<DsrLocationMap, unknown>[] = [
    { accessorKey: "dsr_location", header: "Lokasi DSR" },
    { accessorKey: "vault_code", header: "Vault" },
    { accessorKey: "branch_name", header: "Cabang" },
    {
      id: "status",
      header: "Status",
      cell: ({ row }) => (
        <span className="inline-flex flex-wrap items-center gap-1">
          {row.original.is_active ? (
            <Badge variant="success" icon={CheckCircle} label="Aktif" />
          ) : (
            <Badge variant="danger" icon={XCircle} label="Nonaktif" />
          )}
          {pendingIds.has(row.original.id) && <PendingApprovalBadge />}
        </span>
      ),
    },
    {
      id: "actions",
      header: "Aksi",
      meta: { align: "right" },
      cell: ({ row }) => {
        const m = row.original;
        const busy = pendingIds.has(m.id) || mutation.isPending;
        return (
          <div className="flex justify-end gap-2">
            {m.is_active && (
              <Button
                variant="secondary"
                disabled={busy}
                onClick={() => setEditing({ label: m.dsr_location, map: m })}
              >
                Ubah Vault
              </Button>
            )}
            <Button
              variant={m.is_active ? "danger" : "secondary"}
              disabled={busy}
              onClick={() => run({ op: m.is_active ? "disable" : "enable", id: m.id })}
            >
              {m.is_active ? "Nonaktifkan" : "Aktifkan"}
            </Button>
          </div>
        );
      },
    },
  ];

  const unmapped = unmappedQuery.data ?? [];

  return (
    <div className="flex flex-col gap-4">
      {unmapped.length > 0 && (
        <section
          aria-label="Lokasi DSR belum terpetakan"
          className="rounded-[var(--radius-md)] border border-[var(--n-200)] p-3"
        >
          <h3 className="mb-2 text-sm font-semibold text-[var(--n-800)]">
            Lokasi DSR belum terpetakan ({unmapped.length}) — saldo vault-nya belum terbaca
          </h3>
          <ul className="flex flex-col gap-2">
            {unmapped.map((u) => (
              <li key={u.dsr_location} className="flex items-center justify-between gap-2 text-sm">
                <span>
                  {u.dsr_location}
                  {u.last_report_date && (
                    <span className="text-[var(--n-500)]">
                      {" "}
                      · DSR terakhir {u.last_report_date}
                    </span>
                  )}
                </span>
                <Button
                  variant="secondary"
                  onClick={() => setEditing({ label: u.dsr_location, map: null })}
                >
                  Petakan
                </Button>
              </li>
            ))}
          </ul>
        </section>
      )}

      <div className="flex justify-end">
        <Button onClick={() => setEditing({ label: "", map: null })}>Tambah Mapping</Button>
      </div>

      <PendingCreatesList items={pendingCreates} />

      {mapsQuery.isLoading && <p className="text-sm text-[var(--n-500)]">Memuat…</p>}
      {mapsQuery.isError && (
        <p role="alert" className="text-sm text-[var(--danger-fg)]">
          Gagal memuat mapping DSR
        </p>
      )}
      {mapsQuery.data && (
        <DataTable
          data={mapsQuery.data}
          columns={columns}
          emptyMessage="Belum ada mapping lokasi DSR"
        />
      )}

      {editing && (
        <MapDialog
          vendorId={vendorId}
          editing={editing}
          isPending={mutation.isPending}
          onClose={() => setEditing(null)}
          onSubmit={(m) => run(m, () => setEditing(null))}
        />
      )}
    </div>
  );
}

function MapDialog({
  vendorId,
  editing,
  isPending,
  onClose,
  onSubmit,
}: {
  vendorId: number;
  editing: Editing;
  isPending: boolean;
  onClose: () => void;
  onSubmit: (m: MapMutation) => void;
}) {
  // ponytail: first 100 active vaults of the vendor; add search if a vendor ever has more.
  const vaultsQuery = useVendorVaults(vendorId, { page: 1, page_size: 100, status: "active" });
  const [label, setLabel] = useState(editing.label);
  const [vaultId, setVaultId] = useState(editing.map ? String(editing.map.vendor_vault_id) : "");
  const isEdit = editing.map !== null;
  const canSubmit = label.trim() !== "" && vaultId !== "" && !isPending;

  function submit(e: React.FormEvent): void {
    e.preventDefault();
    if (!canSubmit) return;
    onSubmit(
      editing.map
        ? { op: "update", id: editing.map.id, vendor_vault_id: Number(vaultId) }
        : { op: "create", dsr_location: label.trim(), vendor_vault_id: Number(vaultId) },
    );
  }

  return (
    <Dialog open onClose={onClose} title={isEdit ? "Ubah Vault Mapping DSR" : "Tambah Mapping DSR"}>
      <form onSubmit={submit} className="flex flex-col gap-4">
        <label className="flex flex-col gap-1 text-sm font-medium text-[var(--n-700)]">
          Lokasi DSR (label blok di file DSR)
          <input
            value={label}
            onChange={(e) => setLabel(e.target.value)}
            disabled={isEdit}
            className={inputClass}
          />
        </label>
        <label className="flex flex-col gap-1 text-sm font-medium text-[var(--n-700)]">
          Vault
          <select
            value={vaultId}
            onChange={(e) => setVaultId(e.target.value)}
            className={inputClass}
          >
            <option value="">— pilih vault —</option>
            {(vaultsQuery.data?.vaults ?? []).map((v) => (
              <option key={v.id} value={v.id}>
                {v.vault_code} ({v.category})
              </option>
            ))}
          </select>
        </label>
        <div className="flex justify-end gap-2">
          <Button type="button" variant="secondary" onClick={onClose}>
            Batal
          </Button>
          <Button type="submit" disabled={!canSubmit}>
            Ajukan
          </Button>
        </div>
      </form>
    </Dialog>
  );
}
