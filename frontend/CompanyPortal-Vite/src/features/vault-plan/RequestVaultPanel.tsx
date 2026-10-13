import { Button } from "@/components/ui/Button";
import { ReasonModal } from "@/features/vendor-request/VendorRequestDetail";
import { useToast } from "@/lib/hooks/useToast";
import { useState } from "react";
import { AllocationTable, CoverageSummary } from "./AllocationRows";
import { PlanStatusBadge } from "./VaultPlanList";
import { areaLabel, coverageOf, rowsFromPlan } from "./allocation";
import { useRequestVaultDecision, useRequestVaultReview } from "./api";

const REASON_MAX = 500;

interface RequestVaultPanelProps {
  requestId: number;
  /** ATM-SPV review is open (request in vault_review and the viewer is a checker). */
  canReview: boolean;
}

/**
 * "Penetapan Vault" on the Vendor Request detail (cit-vault-select FR4.5): the
 * request's vaults, order vs allocation, and ATM-SPV approve / reject (-> back
 * to ACM). Renders nothing for requests outside the vault flow.
 */
export function RequestVaultPanel({ requestId, canReview }: RequestVaultPanelProps) {
  const { toast } = useToast();
  const review = useRequestVaultReview(requestId, true);
  const decision = useRequestVaultDecision();
  const [rejectOpen, setRejectOpen] = useState(false);
  const [reason, setReason] = useState("");

  const plan = review.data;
  if (!plan) return null;
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

      <p className="flex items-center gap-2 text-sm text-[var(--n-700)]">
        {areaLabel(plan.acm_area_name)} <PlanStatusBadge status={plan.status} />
      </p>

      {plan.allocations.length > 0 && (
        <>
          <AllocationTable allocations={plan.allocations} />
          <CoverageSummary lines={coverageOf(plan.order_totals, rowsFromPlan(plan.allocations))} />
          {plan.buffer_reason && (
            <p className="text-sm text-[var(--n-700)]">Alasan buffer: {plan.buffer_reason}</p>
          )}
        </>
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
