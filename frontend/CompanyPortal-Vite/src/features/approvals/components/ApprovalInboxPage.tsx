import { Button } from "@/components/ui/Button";
import { EmptyState } from "@/components/ui/EmptyState";
import { PageHeader } from "@/components/ui/PageHeader";
import { AlertCircle } from "lucide-react";
import { useState } from "react";
import { useInbox } from "../hooks";
import { MasterDataReviewDialog } from "./MasterDataReviewDialog";

const SKELETON_ROW_KEYS = ["row-1", "row-2", "row-3"];

const DOCUMENT_LABELS: Record<string, string> = {
  master_data: "Perubahan data master",
};

/**
 * Kotak masuk persetujuan: the caller's pending approval steps. Approver
 * authority comes from the approval hierarchy (not from a role), so this screen
 * is open to every signed-in user and simply shows nothing for non-approvers.
 * Only master-data requests can be reviewed here (the checker sees the exact
 * change before deciding); other document types keep their own screens.
 */
export function ApprovalInboxPage() {
  const inboxQuery = useInbox();
  const [reviewId, setReviewId] = useState<number | null>(null);
  const items = inboxQuery.data ?? [];

  return (
    <div className="flex flex-col gap-6 p-6">
      <PageHeader
        eyebrow="Persetujuan"
        title="Kotak Masuk Persetujuan"
        description="Permintaan yang menunggu keputusan Anda. Perubahan baru diterapkan setelah disetujui."
      />

      {inboxQuery.isLoading && (
        <div className="flex flex-col gap-2">
          {SKELETON_ROW_KEYS.map((key) => (
            <div key={key} className="h-10 w-full animate-pulse rounded bg-[var(--n-200)]" />
          ))}
        </div>
      )}

      {!inboxQuery.isLoading && inboxQuery.isError && (
        <div className="flex flex-col items-center gap-3 py-12">
          <AlertCircle className="h-10 w-10 text-[var(--danger-fg)]" aria-hidden="true" />
          <p className="text-sm text-[var(--n-700)]">Gagal memuat kotak masuk</p>
          <Button variant="secondary" onClick={() => inboxQuery.refetch()}>
            Coba Lagi
          </Button>
        </div>
      )}

      {!inboxQuery.isLoading && !inboxQuery.isError && items.length === 0 && (
        <EmptyState message="Tidak ada permintaan yang menunggu persetujuan Anda" />
      )}

      {!inboxQuery.isLoading && !inboxQuery.isError && items.length > 0 && (
        <div className="overflow-x-auto rounded-lg border border-[var(--n-200)]">
          <table className="w-full border-collapse text-sm">
            <thead>
              <tr className="bg-[var(--n-50)] text-left text-[var(--n-600)]">
                <th className="px-4 py-3 font-medium">Permintaan</th>
                <th className="px-4 py-3 font-medium">Jenis</th>
                <th className="px-4 py-3 font-medium">Diajukan oleh</th>
                <th className="px-4 py-3 font-medium">Level</th>
                <th className="px-4 py-3 font-medium">
                  <span className="sr-only">Aksi</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {items.map((item) => (
                <tr key={item.step_id} className="border-t border-[var(--n-200)]">
                  <td className="px-4 py-3 tabular-nums">#{item.request_id}</td>
                  <td className="px-4 py-3">
                    {DOCUMENT_LABELS[item.document_type] ?? item.document_type}
                  </td>
                  <td className="px-4 py-3 tabular-nums">Pengguna #{item.maker_id}</td>
                  <td className="px-4 py-3 tabular-nums">{item.step_level}</td>
                  <td className="px-4 py-3 text-right">
                    {item.document_type === "master_data" ? (
                      <Button variant="secondary" onClick={() => setReviewId(item.request_id)}>
                        Tinjau
                      </Button>
                    ) : (
                      <span className="text-xs text-[var(--n-500)]">
                        Ditinjau di layar jenis dokumennya
                      </span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <MasterDataReviewDialog requestId={reviewId} onClose={() => setReviewId(null)} />
    </div>
  );
}
