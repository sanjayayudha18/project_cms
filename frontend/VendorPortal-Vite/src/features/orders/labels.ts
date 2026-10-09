import type {
  DenomAmount,
  PartyRole,
  PartyStatus,
  VendorOrderSummary,
} from "@/features/orders/api";
import { formatIDR } from "@/lib/formatters";

type BadgeVariant = "info" | "warning" | "success" | "danger" | "neutral";

export const ROLE_LABEL: Record<PartyRole, string> = {
  replenish: "Replenish (pelaksana)",
  vault: "Vault (penyedia uang)",
};

const PARTY_STATUS: Record<PartyStatus, { label: string; variant: BadgeVariant }> = {
  pending: { label: "Menunggu keputusan", variant: "warning" },
  accepted: { label: "Diterima", variant: "success" },
  rejected: { label: "Ditolak", variant: "danger" },
  withdrawn: { label: "Ditarik", variant: "neutral" },
};

/** Status shown to the vendor: a cancelled request overrides the party status (FR6.4). */
export function orderStatus(
  o: Pick<VendorOrderSummary, "status" | "is_canceled" | "request_status">,
): {
  label: string;
  variant: BadgeVariant;
} {
  if (o.is_canceled) return { label: "Dibatalkan", variant: "neutral" };
  if (o.status === "pending" && o.request_status !== "sent_to_vendor") {
    return { label: "Sedang direvisi CIMB", variant: "info" };
  }
  return PARTY_STATUS[o.status];
}

export function denomLabel(denom: number): string {
  return denom >= 1000 ? `${denom / 1000}K` : String(denom);
}

/** "50K IDR 1.000.000 · 100K IDR 2.000.000" */
export function totalsText(totals: readonly DenomAmount[] | null): string {
  if (!totals || totals.length === 0) return "-";
  return totals.map((t) => `${denomLabel(t.denom)} ${formatIDR(t.amount)}`).join(" · ");
}

export function formatDate(iso: string | null): string {
  if (!iso) return "-";
  return new Date(iso).toLocaleDateString("id-ID", {
    day: "2-digit",
    month: "short",
    year: "numeric",
    timeZone: "UTC",
  });
}
