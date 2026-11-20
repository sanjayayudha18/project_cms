import { Button } from "@/components/ui/Button";
import type { PksListItem } from "../types";

export type PksRowAction = "replace" | "terminate" | "unterminate" | "reminder";

/** Real PKS record that is still the vendor's current one for its type (not replaced, not a pending create). */
function isCurrent(item: PksListItem): boolean {
  return item.id !== null && item.status !== "diganti" && item.status !== "menunggu_persetujuan";
}

/** Per-row write actions (ADMIN/ADMIN_PARAM only; the parent passes `actions` only for them). */
export function PksRowActions({
  item,
  onAction,
}: {
  item: PksListItem;
  onAction: (action: PksRowAction, item: PksListItem) => void;
}) {
  const hasPending = item.pending !== null;
  const current = isCurrent(item);
  const terminated = item.status === "terminated";
  const pendingNote = hasPending ? "Menunggu persetujuan perubahan lain" : undefined;

  if (item.id === null) return null;
  return (
    <div className="flex flex-wrap justify-end gap-2" title={pendingNote}>
      {current && (
        <Button variant="secondary" disabled={hasPending} onClick={() => onAction("replace", item)}>
          Ganti
        </Button>
      )}
      {current && (
        <Button
          variant="secondary"
          disabled={hasPending}
          onClick={() => onAction(terminated ? "unterminate" : "terminate", item)}
        >
          {terminated ? "Batalkan terminasi" : "Terminasi"}
        </Button>
      )}
      <Button variant="secondary" onClick={() => onAction("reminder", item)}>
        Ubah reminder
      </Button>
      {hasPending && <span className="basis-full text-xs text-[var(--n-600)]">{pendingNote}</span>}
    </div>
  );
}
