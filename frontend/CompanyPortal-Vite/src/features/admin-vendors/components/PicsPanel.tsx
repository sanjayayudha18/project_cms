import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { ConfirmActionDialog } from "@/components/ui/ConfirmActionDialog";
import { DataTable } from "@/components/ui/DataTable";
import { useToast } from "@/lib/hooks/useToast";
import type { ColumnDef } from "@tanstack/react-table";
import { BellRing, CheckCircle, XCircle } from "lucide-react";
import { useState } from "react";
import { PendingApprovalBadge } from "../../master-data/PendingApprovalBadge";
import { pendingApprovalMessage } from "../../master-data/changeRequest";
import { usePendingEntityIds } from "../../master-data/pending";
import { useDisableVendorPic, useEnableVendorPic, useVendorPics } from "../hooks";
import type { AdminVendorPic } from "../types";
import { VendorPicFormDialog } from "./VendorPicFormDialog";

interface PicsPanelProps {
  vendorId: number;
  /** A specific branch's PICs, or null for vendor-wide PICs (vendor_branch_id IS NULL). */
  branchId: number | null;
}

/** PIC list scoped to one branch (branchId set) or vendor-wide (branchId null). */
export function PicsPanel({ vendorId, branchId }: PicsPanelProps) {
  const { toast } = useToast();
  const query = useVendorPics(vendorId, {
    page: 1,
    page_size: 100,
    status: "all",
    ...(branchId === null ? { vendor_wide_only: true } : { branch_id: branchId }),
  });
  const pendingIds = usePendingEntityIds("vendor_pic");
  const disableMutation = useDisableVendorPic(vendorId);
  const enableMutation = useEnableVendorPic(vendorId);

  const [formPic, setFormPic] = useState<AdminVendorPic | null>(null);
  const [isFormOpen, setFormOpen] = useState(false);
  const [pendingAction, setPendingAction] = useState<{
    type: "disable" | "enable";
    pic: AdminVendorPic;
  } | null>(null);

  const pics = query.data?.pics ?? [];
  const warnings = query.data?.warnings ?? [];

  function confirmPendingAction(): void {
    if (!pendingAction) return;
    const mutation = pendingAction.type === "disable" ? disableMutation : enableMutation;
    mutation.mutate(pendingAction.pic.id, {
      onSuccess: (res) => {
        toast({ type: "success", message: pendingApprovalMessage(res) });
        setPendingAction(null);
      },
      onError: (err) => {
        toast({ type: "error", message: err.message });
        setPendingAction(null);
      },
    });
  }

  const columns: ColumnDef<AdminVendorPic, unknown>[] = [
    { accessorKey: "name", header: "Nama" },
    {
      accessorKey: "position",
      header: "Jabatan",
      cell: ({ getValue }) => String(getValue() ?? "—"),
    },
    { accessorKey: "phone", header: "Telepon", cell: ({ getValue }) => String(getValue() ?? "—") },
    { accessorKey: "email", header: "Email", cell: ({ getValue }) => String(getValue() ?? "—") },
    {
      accessorKey: "is_notification_recipient",
      header: "Penerima Notifikasi",
      cell: ({ getValue }) =>
        getValue() ? (
          <Badge variant="info" icon={BellRing} label="Penerima notifikasi" />
        ) : (
          <span className="text-[var(--n-500)]">—</span>
        ),
    },
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
        const pic = row.original;
        const isPending = pendingIds.has(pic.id);
        return (
          <div className="flex justify-end gap-2">
            <Button
              variant="secondary"
              disabled={isPending}
              onClick={() => {
                setFormPic(pic);
                setFormOpen(true);
              }}
            >
              Ubah
            </Button>
            {pic.is_active ? (
              <Button
                variant="danger"
                disabled={isPending}
                onClick={() => setPendingAction({ type: "disable", pic })}
              >
                Nonaktifkan
              </Button>
            ) : (
              <Button
                variant="secondary"
                disabled={isPending}
                onClick={() => setPendingAction({ type: "enable", pic })}
              >
                Aktifkan
              </Button>
            )}
          </div>
        );
      },
    },
  ];

  return (
    <div className="flex flex-col gap-3 rounded-lg border border-[var(--n-200)] bg-[var(--n-0)] p-4">
      <div className="flex justify-end">
        <Button
          onClick={() => {
            setFormPic(null);
            setFormOpen(true);
          }}
        >
          Tambah PIC
        </Button>
      </div>

      {warnings.map((warning) => (
        <output key={warning} className="text-sm text-[var(--warning-fg)]">
          {warning}
        </output>
      ))}

      {query.isLoading && <p className="text-sm text-[var(--n-500)]">Memuat…</p>}
      {query.isError && (
        <p role="alert" className="text-sm text-[var(--danger-fg)]">
          Gagal memuat data PIC
        </p>
      )}
      {!query.isLoading && !query.isError && (
        <DataTable
          data={pics}
          columns={columns}
          emptyMessage={
            branchId === null ? "Belum ada PIC vendor-wide" : "Cabang ini belum punya PIC"
          }
        />
      )}

      <VendorPicFormDialog
        open={isFormOpen}
        onClose={() => setFormOpen(false)}
        vendorId={vendorId}
        branchId={branchId}
        pic={formPic}
      />

      <ConfirmActionDialog
        open={pendingAction !== null}
        title={pendingAction?.type === "disable" ? "Nonaktifkan PIC" : "Aktifkan PIC"}
        message={
          pendingAction?.type === "disable"
            ? `Nonaktifkan PIC "${pendingAction.pic.name}"?`
            : `Aktifkan kembali PIC "${pendingAction?.pic.name}"?`
        }
        confirmLabel={pendingAction?.type === "disable" ? "Nonaktifkan" : "Aktifkan"}
        isPending={disableMutation.isPending || enableMutation.isPending}
        onConfirm={confirmPendingAction}
        onClose={() => setPendingAction(null)}
      />
    </div>
  );
}
