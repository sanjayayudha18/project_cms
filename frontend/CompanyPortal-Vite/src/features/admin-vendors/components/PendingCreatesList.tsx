import { PendingApprovalBadge } from "../../master-data/PendingApprovalBadge";

interface PendingCreatesListProps {
  /** Change-request id + a human label for each staged create in this scope. */
  items: { id: number; label: string }[];
}

/**
 * Staged creates have no master row yet, so they are listed above the table
 * instead of as rows -- the maker sees the request exists and does not
 * submit it twice.
 */
export function PendingCreatesList({ items }: PendingCreatesListProps) {
  if (items.length === 0) return null;
  return (
    <section
      aria-label="Pengajuan baru menunggu approval"
      className="rounded-[var(--radius-md)] border border-[var(--n-200)] bg-[var(--n-50)] p-3 text-sm"
    >
      <p className="mb-2 font-medium text-[var(--n-700)]">
        Pengajuan baru menunggu approval ({items.length})
      </p>
      <ul className="flex flex-col gap-1">
        {items.map((item) => (
          <li key={item.id} className="flex flex-wrap items-center gap-2 text-[var(--n-800)]">
            <PendingApprovalBadge />
            <span>{item.label}</span>
            <span className="text-[var(--n-500)]">#{item.id}</span>
          </li>
        ))}
      </ul>
    </section>
  );
}
