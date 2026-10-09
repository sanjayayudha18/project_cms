import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { ReasonModal } from "@/features/vendor-request/VendorRequestDetail";
import { useToast } from "@/lib/hooks/useToast";
import { AlertTriangle } from "lucide-react";
import { useState } from "react";
import { PlanStatusBadge } from "./VaultPlanList";
import { TIER_LABELS, formatDenoms, useRequestVaultDecision, useRequestVaultReview } from "./api";

const REASON_MAX = 500;

interface RequestVaultPanelProps {
  requestId: number;
  /** ATM-SPV review is open (request in vault_review and the viewer is a checker). */
  canReview: boolean;
}

/**
 * "Penetapan Vault" on the Vendor Request detail (cit-acm-plan FR5): the ACM
 * recommendation per area + ATM, and ATM-SPV approve (-> ready) / reject
 * (-> back to ACM). Renders nothing for requests outside the vault flow.
 */
export function RequestVaultPanel({ requestId, canReview }: RequestVaultPanelProps) {
  const { toast } = useToast();
  const review = useRequestVaultReview(requestId, true);
  const decision = useRequestVaultDecision();
  const [rejectOpen, setRejectOpen] = useState(false);
  const [reason, setReason] = useState("");

  if (!review.data || review.data.plans.length === 0) return null;
  const { plans, assignments } = review.data;
  const reasonValid = reason.trim().length >= 1 && reason.trim().length <= REASON_MAX;

  function decide(approve: boolean): void {
    decision.mutate(
      { id: requestId, approve, reason: approve ? undefined : reason.trim() },
      {
        onSuccess: () => {
          toast({
            type: "success",
            message: approve ? "Penetapan vault disetujui" : "Penetapan vault dikembalikan ke ACM",
          });
          setRejectOpen(false);
          setReason("");
        },
        onError: (err) => toast({ type: "error", message: err.message }),
      },
    );
  }

  return (
    <section
      aria-labelledby="vault-panel-title"
      className="flex flex-col gap-3 rounded-[var(--radius-lg)] border border-[var(--n-200)] p-4"
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 id="vault-panel-title" className="text-base font-semibold text-[var(--n-900)]">
          Penetapan Vault
        </h2>
        {canReview && (
          <div className="flex gap-2">
            <Button
              variant="danger"
              disabled={decision.isPending}
              onClick={() => setRejectOpen(true)}
            >
              Tolak Penetapan
            </Button>
            <Button disabled={decision.isPending} onClick={() => decide(true)}>
              Setujui Penetapan
            </Button>
          </div>
        )}
      </div>

      <ul className="flex flex-wrap gap-3 text-sm text-[var(--n-700)]">
        {plans.map((p) => (
          <li key={p.id} className="flex items-center gap-2">
            Area {p.acm_area_name} <PlanStatusBadge status={p.status} />
          </li>
        ))}
      </ul>

      {assignments.length > 0 && (
        <div className="overflow-x-auto">
          <table className="w-full min-w-[720px] border-collapse text-sm">
            <thead>
              <tr className="border-[var(--n-200)] border-b bg-[var(--n-50)] text-left text-[var(--n-600)]">
                <th className="px-3 py-2 font-medium">ATM</th>
                <th className="px-3 py-2 font-medium">Branch Vault</th>
                <th className="px-3 py-2 font-medium">Tier</th>
                <th className="px-3 py-2 text-right font-medium">Saldo Vault (IDR)</th>
                <th className="px-3 py-2 text-right font-medium">Kapasitas (IDR)</th>
              </tr>
            </thead>
            <tbody>
              {assignments.map((a) => (
                <tr
                  key={a.terminal_id}
                  className="border-[var(--n-100)] border-b align-top last:border-0"
                >
                  <td className="px-3 py-2 font-mono">{a.terminal_id}</td>
                  <td className="px-3 py-2">
                    {a.vault_branch.code} {a.vault_branch.name} · {a.vault_branch.vendor_name}
                    {a.is_urgent && (
                      <div className="text-xs text-[var(--n-600)]">Urgent: {a.urgent_reason}</div>
                    )}
                  </td>
                  <td className="px-3 py-2">{TIER_LABELS[a.tier]}</td>
                  <td className="px-3 py-2 text-right tabular-nums">
                    {formatDenoms(a.saldo_snapshot)}
                  </td>
                  <td className="px-3 py-2 text-right tabular-nums">
                    {formatDenoms(a.capacity_snapshot)}
                    {a.capacity_warning && (
                      <div className="mt-1 flex justify-end">
                        <Badge
                          variant="warning"
                          icon={AlertTriangle}
                          label="Peringatan kapasitas"
                        />
                      </div>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {rejectOpen && (
        <ReasonModal
          title="Tolak Penetapan Vault"
          reasonLabel="Alasan Penolakan"
          confirmLabel="Kembalikan ke ACM"
          busyLabel="Menolak..."
          reason={reason}
          onReasonChange={setReason}
          valid={reasonValid}
          busy={decision.isPending}
          onConfirm={() => decide(false)}
          onDismiss={() => setRejectOpen(false)}
        />
      )}
    </section>
  );
}
