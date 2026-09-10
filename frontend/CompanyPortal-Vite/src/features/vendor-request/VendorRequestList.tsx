/**
 * Vendor Request list page (Req 13): filterable, paginated table of
 * vendor_requests with status badges, request-number search, and row
 * click-through to detail.
 */

import { Button } from "@/components/ui/Button";
import { PageHeader } from "@/components/ui/PageHeader";
import { formatAtmDate, formatAtmDateTime } from "@/features/atm-portal/lib/formatters";
import { useAuthStore } from "@/lib/auth/store";
import { formatIDR } from "@/lib/utils/formatCurrency";
import { Link, useNavigate } from "@tanstack/react-router";
import { AlertCircle } from "lucide-react";
import { StatusBadge } from "./StatusBadge";
import { useVendorRequests } from "./hooks";
import { VENDOR_REQUEST_STATUSES, type VendorRequestStatus } from "./types";
import { useVendorRequestListUrlState } from "./useVendorRequestListUrlState";

const MAKER_ROLES = ["ADMIN", "ATM-USER", "BRANCH-ATM-USER"];
const PAGE_SIZE_OPTIONS = [10, 20, 50] as const;

const STATUS_LABELS: Record<VendorRequestStatus, string> = {
  draft: "Draft",
  pending_approval: "Menunggu Approval",
  approved: "Disetujui",
  rejected: "Ditolak",
  processing: "Diproses",
  completed: "Selesai",
  failed: "Gagal",
  cancelled: "Dibatalkan",
};

export function VendorRequestList() {
  const navigate = useNavigate();
  const role = useAuthStore((s) => s.user?.role);
  const { params, searchInput, setSearchInput, setParams } = useVendorRequestListUrlState();

  const { data, isLoading, isError, refetch } = useVendorRequests({
    status: params.status,
    forecastDate: params.forecastDate || undefined,
    requestNumber: params.search || undefined,
    page: params.page,
    pageSize: params.pageSize,
  });

  const rows = data?.data ?? [];
  const totalPages = Math.max(1, data?.pagination.total_pages ?? 1);
  const isMaker = role ? MAKER_ROLES.includes(role) : false;

  function toggleStatus(status: VendorRequestStatus): void {
    const next = params.status.includes(status)
      ? params.status.filter((s) => s !== status)
      : [...params.status, status];
    setParams({ status: next, page: 1 });
  }

  function goToDetail(id: number): void {
    navigate({ to: "/replenishment/vendor-requests/$id", params: { id: String(id) } });
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Replenishment"
        title="Vendor Request"
        description="Pantau status pengajuan replenishment ke vendor CIT"
        actions={
          isMaker ? (
            <Button onClick={() => navigate({ to: "/replenishment/forecast-browser" })}>
              Vendor Request Baru
            </Button>
          ) : undefined
        }
      />

      <div className="flex flex-wrap items-end gap-4">
        <details className="relative flex flex-col gap-1">
          <summary className="min-h-[44px] cursor-pointer list-none rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 py-2 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]">
            Status {params.status.length > 0 ? `(${params.status.length})` : ""}
          </summary>
          <div className="absolute top-full z-10 mt-1 flex w-56 flex-col gap-1 rounded-[var(--radius-md)] border border-[var(--n-200)] bg-[var(--n-0)] p-2 shadow-[var(--shadow-md)]">
            {VENDOR_REQUEST_STATUSES.map((status) => (
              <label
                key={status}
                className="flex min-h-[36px] items-center gap-2 rounded-[var(--radius-sm)] px-2 text-sm text-[var(--n-800)] hover:bg-[var(--n-50)]"
              >
                <input
                  type="checkbox"
                  checked={params.status.includes(status)}
                  onChange={() => toggleStatus(status)}
                  className="h-4 w-4 accent-[var(--red-500)]"
                />
                {STATUS_LABELS[status]}
              </label>
            ))}
          </div>
        </details>

        <div className="flex flex-col gap-1">
          <label
            htmlFor="vr-list-forecast-date"
            className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]"
          >
            Tanggal Forecast
          </label>
          <input
            id="vr-list-forecast-date"
            type="date"
            value={params.forecastDate}
            onChange={(e) => setParams({ forecastDate: e.target.value, page: 1 })}
            className="min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
          />
        </div>

        <div className="flex flex-col gap-1">
          <label
            htmlFor="vr-list-search"
            className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]"
          >
            Cari No. Request
          </label>
          <input
            id="vr-list-search"
            type="text"
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value.slice(0, 50))}
            maxLength={50}
            placeholder="VR-20260912-0001"
            className="min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
          />
        </div>
      </div>

      <div className="overflow-x-auto rounded-[var(--radius-lg)] border border-[var(--n-200)]">
        <table className="w-full min-w-[900px] border-collapse text-sm">
          <thead>
            <tr className="border-[var(--n-200)] border-b bg-[var(--n-50)]">
              <th className="px-3 py-2 text-left font-medium text-[var(--n-600)]">No. Request</th>
              <th className="px-3 py-2 text-left font-medium text-[var(--n-600)]">
                Tanggal Forecast
              </th>
              <th className="px-3 py-2 text-left font-medium text-[var(--n-600)]">Status</th>
              <th className="px-3 py-2 text-right font-medium text-[var(--n-600)]">Jumlah Item</th>
              <th className="px-3 py-2 text-right font-medium text-[var(--n-600)]">Total Amount</th>
              <th className="px-3 py-2 text-left font-medium text-[var(--n-600)]">Dibuat Oleh</th>
              <th className="px-3 py-2 text-left font-medium text-[var(--n-600)]">Dibuat Pada</th>
              <th className="px-3 py-2" />
            </tr>
          </thead>
          <tbody>
            {isLoading && <SkeletonRows />}
            {!isLoading && isError && <ErrorRow onRetry={() => refetch()} />}
            {!isLoading && !isError && rows.length === 0 && <EmptyRow />}
            {!isLoading &&
              !isError &&
              rows.map((row) => (
                // Mouse-only row click (Req 13.5); role="button" on <tr> isn't valid
                // ARIA and would fight the table's row semantics, so keyboard/screen-
                // reader users reach the same destination via the "Lihat Detail" link
                // cell below instead, a real focusable control.
                // biome-ignore lint/a11y/useKeyWithClickEvents: see comment above
                <tr
                  key={row.id}
                  onClick={() => goToDetail(row.id)}
                  className="cursor-pointer border-[var(--n-100)] border-b last:border-0 hover:bg-[var(--red-50)]"
                >
                  <td className="px-3 py-2 font-mono">{row.request_number}</td>
                  <td className="px-3 py-2">{formatAtmDate(new Date(row.forecast_date))}</td>
                  <td className="px-3 py-2">
                    <StatusBadge status={row.status} />
                  </td>
                  <td className="px-3 py-2 text-right tabular-nums">{row.item_count}</td>
                  <td className="px-3 py-2 text-right tabular-nums">
                    {formatIDR(row.total_amount)}
                  </td>
                  <td className="px-3 py-2">{row.created_by.full_name}</td>
                  <td className="px-3 py-2">{formatAtmDateTime(new Date(row.created_at))}</td>
                  <td className="px-3 py-2 text-right">
                    <Link
                      to="/replenishment/vendor-requests/$id"
                      params={{ id: String(row.id) }}
                      onClick={(e) => e.stopPropagation()}
                      className="text-sm font-medium text-[var(--red-600)] hover:underline"
                    >
                      Lihat Detail
                    </Link>
                  </td>
                </tr>
              ))}
          </tbody>
        </table>
      </div>

      <div className="flex flex-wrap items-center justify-between gap-4 text-sm text-[var(--n-600)]">
        <div className="flex items-center gap-2">
          <label htmlFor="vr-list-page-size">Baris per halaman</label>
          <select
            id="vr-list-page-size"
            value={params.pageSize}
            onChange={(e) => setParams({ pageSize: Number(e.target.value), page: 1 })}
            className="min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
          >
            {PAGE_SIZE_OPTIONS.map((size) => (
              <option key={size} value={size}>
                {size}
              </option>
            ))}
          </select>
        </div>
        <div className="flex items-center gap-2">
          <span>
            Halaman {params.page} dari {totalPages}
          </span>
          <Button
            variant="secondary"
            disabled={params.page <= 1}
            onClick={() => setParams({ page: params.page - 1 })}
          >
            Sebelumnya
          </Button>
          <Button
            variant="secondary"
            disabled={params.page >= totalPages}
            onClick={() => setParams({ page: params.page + 1 })}
          >
            Berikutnya
          </Button>
        </div>
      </div>
    </div>
  );
}

const COLUMN_COUNT = 8;

function SkeletonRows() {
  return (
    <>
      {Array.from({ length: 5 }).map((_, rowIndex) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: generic loading placeholder rows have no stable identity
        <tr key={`skeleton-row-${rowIndex}`}>
          {Array.from({ length: COLUMN_COUNT }).map((_, colIndex) => (
            // biome-ignore lint/suspicious/noArrayIndexKey: generic loading placeholder cells have no stable identity
            <td key={`skeleton-cell-${rowIndex}-${colIndex}`} className="px-3 py-3">
              <div className="h-3.5 w-full animate-pulse rounded bg-[var(--n-200)]" />
            </td>
          ))}
        </tr>
      ))}
    </>
  );
}

function ErrorRow({ onRetry }: { onRetry: () => void }) {
  return (
    <tr>
      <td colSpan={COLUMN_COUNT} className="px-3 py-12 text-center">
        <div className="flex flex-col items-center gap-3">
          <AlertCircle className="h-10 w-10 text-[var(--danger-fg)]" aria-hidden="true" />
          <p className="text-sm text-[var(--n-700)]">Gagal memuat daftar vendor request</p>
          <Button variant="secondary" onClick={onRetry}>
            Coba Lagi
          </Button>
        </div>
      </td>
    </tr>
  );
}

function EmptyRow() {
  return (
    <tr>
      <td colSpan={COLUMN_COUNT} className="px-3 py-12 text-center">
        <p className="text-sm text-[var(--n-700)]">
          Tidak ada vendor request yang sesuai dengan filter saat ini
        </p>
      </td>
    </tr>
  );
}
