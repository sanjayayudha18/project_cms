import { Badge, type BadgeVariant } from "@/components/ui/Badge";
import { formatAtmDateTime } from "@/features/atm-portal/lib/formatters";
import { type VendorPartyEvent, type VendorPartyStatus, useVendorParties } from "./vendorSide";

const ROLE_LABEL = { replenish: "Replenish (pelaksana)", vault: "Vault (penyedia uang)" } as const;

const STATUS: Record<VendorPartyStatus, { variant: BadgeVariant; label: string }> = {
  pending: { variant: "warning", label: "Menunggu keputusan" },
  accepted: { variant: "success", label: "Diterima" },
  rejected: { variant: "danger", label: "Ditolak" },
  withdrawn: { variant: "neutral", label: "Ditarik" },
};

const EVENT_LABEL: Record<VendorPartyEvent["event"], string> = {
  sent: "Dikirim",
  resent: "Dikirim ulang",
  accepted: "Diterima",
  rejected: "Ditolak",
  withdrawn: "Ditarik",
  cancelled: "Request dibatalkan",
};

function when(iso: string | null): string {
  return iso ? formatAtmDateTime(new Date(iso)) : "-";
}

/**
 * "Status Vendor" on the Vendor Request detail (cit-send-vendor FR6.6): one row
 * per vendor party (replenish / vault branch) with its decision, plus the
 * decision history. Renders nothing for requests never sent to vendors.
 */
export function VendorPartiesPanel({ requestId }: { requestId: number }) {
  const { data } = useVendorParties(requestId);
  if (!data || data.parties.length === 0) return null;
  const branchOf = new Map(
    data.parties.map((p) => [p.id, `${p.branch_name} (${ROLE_LABEL[p.role]})`]),
  );

  return (
    <section
      aria-labelledby="vendor-parties-title"
      className="flex flex-col gap-3 rounded-[var(--radius-lg)] border border-[var(--n-200)] p-4"
    >
      <h2 id="vendor-parties-title" className="text-base font-semibold text-[var(--n-900)]">
        Status Vendor
      </h2>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead className="text-left text-xs uppercase tracking-wide text-[var(--n-500)]">
            <tr>
              <th scope="col" className="py-2 pr-3">
                Peran
              </th>
              <th scope="col" className="py-2 pr-3">
                Branch
              </th>
              <th scope="col" className="py-2 pr-3">
                Vendor
              </th>
              <th scope="col" className="py-2 pr-3">
                Status
              </th>
              <th scope="col" className="py-2 pr-3">
                Diputuskan
              </th>
              <th scope="col" className="py-2">
                Alasan tolak
              </th>
            </tr>
          </thead>
          <tbody>
            {data.parties.map((p) => (
              <tr key={p.id} className="border-t border-[var(--n-100)]">
                <td className="py-2 pr-3">{ROLE_LABEL[p.role]}</td>
                <td className="py-2 pr-3">
                  {p.branch_name} <span className="text-[var(--n-500)]">({p.branch_code})</span>
                </td>
                <td className="py-2 pr-3">{p.vendor_name}</td>
                <td className="py-2 pr-3">
                  <Badge variant={STATUS[p.status].variant} label={STATUS[p.status].label} />
                </td>
                <td className="py-2 pr-3">
                  {p.decided_at ? `${when(p.decided_at)} · ${p.decided_by_name ?? "-"}` : "-"}
                </td>
                <td className="py-2">{p.rejection_reason ?? "-"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {data.events.length > 0 && (
        <details className="text-sm">
          <summary className="cursor-pointer font-medium text-[var(--n-700)]">
            Riwayat keputusan vendor ({data.events.length})
          </summary>
          <ol className="mt-2 flex flex-col gap-1">
            {data.events.map((e) => (
              <li key={e.id}>
                <span className="tabular-nums text-[var(--n-500)]">{when(e.created_at)}</span> ·{" "}
                {EVENT_LABEL[e.event]} · {branchOf.get(e.party_id) ?? "-"} · {e.actor_name}
                {e.reason ? ` — ${e.reason}` : ""}
              </li>
            ))}
          </ol>
        </details>
      )}
    </section>
  );
}
