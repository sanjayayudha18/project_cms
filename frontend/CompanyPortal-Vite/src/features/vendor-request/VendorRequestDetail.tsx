/**
 * Vendor Request detail page (Req 14): header (nulls omitted), items table
 * + total, status badge, and state×role×creator-gated actions mirroring the
 * backend's checkActor (vendor_request.go). Gates + handlers live in
 * useVendorRequestActions; buttons and modals in VendorRequestActions.
 */

import { PageHeader } from "@/components/ui/PageHeader";
import { RequestVaultPanel } from "@/features/vault-plan/RequestVaultPanel";
import { useAuthStore } from "@/lib/auth/store";
import { formatThousands } from "@/lib/utils/formatCurrency";
import { Link, useParams } from "@tanstack/react-router";
import { AlertCircle } from "lucide-react";
import { CompletionAtmTable } from "./CompletionPanels";
import { StatusBadge } from "./StatusBadge";
import { VendorPartiesPanel } from "./VendorPartiesPanel";
import { VendorRequestActionBar, VendorRequestModals } from "./VendorRequestActions";
import { DetailFields, DetailSkeleton, NotFoundState } from "./VendorRequestDetailParts";
import { useVendorRequest } from "./hooks";
import type { VendorRequestDetail as VendorRequestDetailType } from "./types";
import { getRequestPermissions, useVendorRequestActions } from "./useVendorRequestActions";

// cit-acm-plan FR5: the Penetapan Vault panel (renders nothing for legacy, non-vault requests).
const VAULT_PANEL_STATUSES = [
  "vault_assignment",
  "vault_review",
  "ready",
  "sent_to_vendor",
  "vendor_accepted",
  "vendor_rejected",
  "completion_pending",
  "completed",
];

export function VendorRequestDetail() {
  const routeParams = useParams({ strict: false }) as { id?: string };
  const id = routeParams.id ? Number(routeParams.id) : null;
  const { data, isLoading, isError, error } = useVendorRequest(id);

  if (id === null || Number.isNaN(id)) {
    return <NotFoundState />;
  }

  if (isLoading) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader eyebrow="Replenishment" title="Vendor Request" />
        <DetailSkeleton />
      </div>
    );
  }

  if (isError) {
    if (error?.status === 404) return <NotFoundState />;
    return (
      <div className="flex flex-col gap-6">
        <PageHeader eyebrow="Replenishment" title="Vendor Request" />
        <div className="flex flex-col items-center gap-3 rounded-[var(--radius-lg)] border border-[var(--n-200)] py-12">
          <AlertCircle className="h-10 w-10 text-[var(--danger-fg)]" aria-hidden="true" />
          <p className="text-sm text-[var(--n-700)]">Gagal memuat detail vendor request</p>
        </div>
      </div>
    );
  }

  if (!data) return <NotFoundState />;

  return <VendorRequestBody data={data} />;
}

function VendorRequestBody({ data }: { data: VendorRequestDetailType }) {
  const user = useAuthStore((s) => s.user);
  const a = useVendorRequestActions(data);
  const perms = getRequestPermissions(data, user);
  const items = a.editingItems ?? data.items;

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Replenishment"
        title={data.request_number}
        description="Detail pengajuan vendor request"
        actions={
          <div className="flex flex-wrap gap-2">
            <StatusBadge status={data.status} />
            {data.is_canceled && <StatusBadge status={data.status} isCanceled />}
          </div>
        }
      />

      <Link
        to="/replenishment/vendor-requests"
        className="text-sm font-medium text-[var(--red-600)] hover:underline"
      >
        ← Kembali ke Daftar Vendor Request
      </Link>

      <DetailFields detail={data} />

      <div className="overflow-x-auto rounded-[var(--radius-lg)] border border-[var(--n-200)]">
        <table className="w-full min-w-[640px] border-collapse text-sm">
          <thead>
            <tr className="border-[var(--n-200)] border-b bg-[var(--n-50)]">
              <th className="px-3 py-2 text-left font-medium text-[var(--n-600)]">ATM ID</th>
              <th className="px-3 py-2 text-left font-medium text-[var(--n-600)]">No. Tiket</th>
              <th className="px-3 py-2 text-right font-medium text-[var(--n-600)]">Denominasi</th>
              <th className="px-3 py-2 text-right font-medium text-[var(--n-600)]">
                Amount Replenish
              </th>
            </tr>
          </thead>
          <tbody>
            {items.map((item) => (
              <tr key={item.id} className="border-[var(--n-100)] border-b last:border-0">
                <td className="px-3 py-2">{item.terminal_id}</td>
                <td className="px-3 py-2 font-mono">{item.ticket_number || "-"}</td>
                <td className="px-3 py-2 text-right tabular-nums">{formatThousands(item.denom)}</td>
                <td className="px-3 py-2 text-right tabular-nums">
                  {a.editingItems ? (
                    <input
                      type="number"
                      aria-label={`Amount replenish untuk ${item.terminal_id}`}
                      value={item.amount_replenish}
                      onChange={(e) => a.updateEditedAmount(item.id, Number(e.target.value))}
                      className="w-36 rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-2 py-1 text-right text-sm tabular-nums outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
                    />
                  ) : (
                    formatThousands(item.amount_replenish)
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="flex flex-wrap items-center justify-between gap-4 rounded-[var(--radius-lg)] border border-[var(--n-200)] bg-[var(--n-50)] px-4 py-3">
        <div className="text-sm text-[var(--n-700)]">
          <span className="font-semibold text-[var(--n-900)]">{items.length}</span> item — Total:{" "}
          <span className="font-semibold tabular-nums text-[var(--n-900)]">
            Rp {formatThousands(data.total_amount)}
          </span>
        </div>

        <div className="flex flex-wrap gap-3">
          <VendorRequestActionBar a={a} perms={perms} />
        </div>
      </div>

      {VAULT_PANEL_STATUSES.includes(data.status) && (
        <RequestVaultPanel
          requestId={data.id}
          canReview={data.status === "vault_review" && perms.isChecker && !data.is_canceled}
        />
      )}

      <VendorPartiesPanel requestId={data.id} />

      {perms.awaitsVendorReport && (
        <p className="rounded-[var(--radius-md)] bg-[var(--info-bg)] px-4 py-3 text-sm text-[var(--info-fg)]">
          Menunggu laporan realisasi dari vendor replenish (diisi di VendorPortal). Setelah masuk,
          ATM-SPV menyetujui atau menolak laporan di halaman ini.
        </p>
      )}

      {perms.showCompletionTable && <CompletionAtmTable atms={data.atms} />}

      <VendorRequestModals a={a} data={data} />
    </div>
  );
}
