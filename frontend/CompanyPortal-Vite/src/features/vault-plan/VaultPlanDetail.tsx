import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { NoticeBanner } from "@/components/ui/NoticeBanner";
import { PageHeader } from "@/components/ui/PageHeader";
import { ReasonModal } from "@/features/vendor-request/VendorRequestDetail";
import { useAuthStore } from "@/lib/auth/store";
import { useToast } from "@/lib/hooks/useToast";
import { Link, useParams } from "@tanstack/react-router";
import { AlertTriangle } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { PlanStatusBadge } from "./VaultPlanList";
import {
  TIER_LABELS,
  type VaultAssignmentView,
  type VaultCandidate,
  type VaultPlanAtm,
  type VaultPlanDetail as VaultPlanDetailType,
  type VaultPlanMutation,
  formatDenoms,
  useVaultCandidates,
  useVaultPlan,
  useVaultPlanMutation,
} from "./api";

const URGENT_REASON_MIN = 10;
const URGENT_REASON_MAX = 500;
const REJECT_REASON_MAX = 500;

interface DraftRow {
  vaultBranchId: number | null;
  isUrgent: boolean;
  urgentReason: string;
}

const EMPTY_DRAFT: DraftRow = { vaultBranchId: null, isUrgent: false, urgentReason: "" };

function initialDrafts(plan: VaultPlanDetailType): Record<string, DraftRow> {
  return Object.fromEntries(
    plan.atms.map((a) => [
      a.terminal_id,
      {
        vaultBranchId: a.assignment?.vault_branch.id ?? null,
        isUrgent: a.assignment?.is_urgent ?? false,
        urgentReason: a.assignment?.urgent_reason ?? "",
      },
    ]),
  );
}

function urgentReasonValid(d: DraftRow): boolean {
  const n = d.urgentReason.trim().length;
  return !d.isUrgent || (n >= URGENT_REASON_MIN && n <= URGENT_REASON_MAX);
}

/** Detail rencana vault (cit-acm-plan FR2-FR4): per-ATM penetapan, submit, approve/reject. */
export function VaultPlanDetail() {
  const { id } = useParams({ strict: false }) as { id?: string };
  const planId = id ? Number(id) : null;
  const query = useVaultPlan(planId);

  if (query.isLoading) return <p className="p-6 text-sm text-[var(--n-500)]">Memuat…</p>;
  if (query.isError || !query.data) {
    return (
      <div className="flex flex-col gap-4 p-6">
        <PageHeader eyebrow="CIT" title="Rencana Vault Tidak Ditemukan" />
        <Link to="/cit/vault-plans" className="text-sm text-[var(--red-600)] hover:underline">
          ← Kembali ke Penetapan Vault
        </Link>
      </div>
    );
  }
  // Re-seed the editor whenever the server state changes (submit/reject/approve).
  return <PlanEditor key={`${query.data.id}-${query.data.status}`} plan={query.data} />;
}

function PlanEditor({ plan }: { plan: VaultPlanDetailType }) {
  const user = useAuthStore((s) => s.user);
  const { toast } = useToast();
  const mutation = useVaultPlanMutation();
  const [drafts, setDrafts] = useState(() => initialDrafts(plan));
  const [rejectOpen, setRejectOpen] = useState(false);
  const [rejectReason, setRejectReason] = useState("");

  const canEdit = plan.status === "draft" && user?.role === "ACM-USER";
  const canDecide =
    plan.status === "pending_acm_approval" &&
    user?.role === "ACM-SPV" &&
    plan.submitted_by !== user.id;
  const allAssigned = plan.atms.every((a) => a.assignment !== null);
  const draftList = Object.entries(drafts);
  const saveValid = draftList.every(([, d]) => urgentReasonValid(d));
  const rejectValid =
    rejectReason.trim().length >= 1 && rejectReason.trim().length <= REJECT_REASON_MAX;

  function run(m: VaultPlanMutation, success: string, onDone?: () => void): void {
    mutation.mutate(m, {
      onSuccess: () => {
        toast({ type: "success", message: success });
        onDone?.();
      },
      onError: (err) => toast({ type: "error", message: err.message }),
    });
  }

  function save(): void {
    const assignments = draftList.flatMap(([terminalId, d]) =>
      d.vaultBranchId === null
        ? []
        : [
            {
              terminal_id: terminalId,
              vault_branch_id: d.vaultBranchId,
              is_urgent: d.isUrgent,
              urgent_reason: d.isUrgent ? d.urgentReason.trim() : null,
            },
          ],
    );
    run({ op: "save", id: plan.id, assignments }, "Penetapan vault disimpan");
  }

  return (
    <div className="flex flex-col gap-6 p-6">
      <PageHeader
        eyebrow="CIT · Penetapan Vault"
        title={plan.request_number}
        description={`Area ${plan.acm_area_name} · Tanggal replenish ${plan.replenish_date ?? "-"}`}
        actions={<PlanStatusBadge status={plan.status} />}
      />
      <Link to="/cit/vault-plans" className="text-sm text-[var(--red-600)] hover:underline">
        ← Kembali ke Penetapan Vault
      </Link>

      {plan.status === "draft" && plan.rejection_reason && (
        <NoticeBanner
          icon={AlertTriangle}
          variant="warning"
          title={
            plan.rejected_by_vendor
              ? "Ditolak vendor (branch vault) — pilih vault lain"
              : "Ditolak ACM-SPV"
          }
          description={plan.rejection_reason}
        />
      )}
      {plan.status === "draft" && plan.vault_rejection_reason && (
        <NoticeBanner
          icon={AlertTriangle}
          variant="warning"
          title="Ditolak ATM-SPV saat review"
          description={plan.vault_rejection_reason}
        />
      )}

      <div className="overflow-x-auto rounded-[var(--radius-lg)] border border-[var(--n-200)]">
        <table className="w-full min-w-[960px] border-collapse text-sm">
          <thead>
            <tr className="border-[var(--n-200)] border-b bg-[var(--n-50)] text-left text-[var(--n-600)]">
              <th className="px-3 py-2 font-medium">ATM</th>
              <th className="px-3 py-2 font-medium">Cabang Replenish</th>
              <th className="px-3 py-2 text-right font-medium">Order (IDR)</th>
              <th className="px-3 py-2 font-medium">Branch Vault</th>
            </tr>
          </thead>
          <tbody>
            {plan.atms.map((atm) => (
              <AtmRow
                key={atm.terminal_id}
                planId={plan.id}
                atm={atm}
                canEdit={canEdit}
                draft={drafts[atm.terminal_id] ?? EMPTY_DRAFT}
                onChange={(d) => setDrafts((cur) => ({ ...cur, [atm.terminal_id]: d }))}
              />
            ))}
          </tbody>
        </table>
      </div>

      {canEdit && !allAssigned && (
        <p className="text-right text-xs text-[var(--n-600)]">
          Simpan branch vault untuk semua ATM sebelum mengirim untuk approval.
        </p>
      )}
      <div className="flex flex-wrap justify-end gap-3">
        {canEdit && (
          <>
            <Button variant="secondary" disabled={mutation.isPending || !saveValid} onClick={save}>
              Simpan
            </Button>
            <Button
              disabled={mutation.isPending || !allAssigned}
              onClick={() => run({ op: "submit", id: plan.id }, "Dikirim ke ACM-SPV")}
            >
              Kirim untuk Approval
            </Button>
          </>
        )}
        {canDecide && (
          <>
            <Button
              variant="danger"
              disabled={mutation.isPending}
              onClick={() => setRejectOpen(true)}
            >
              Tolak
            </Button>
            <Button
              disabled={mutation.isPending}
              onClick={() => run({ op: "approve", id: plan.id }, "Rencana vault disetujui")}
            >
              Setujui
            </Button>
          </>
        )}
      </div>

      {rejectOpen && (
        <ReasonModal
          title="Tolak Rencana Vault"
          reasonLabel="Alasan Penolakan"
          confirmLabel="Tolak"
          busyLabel="Menolak..."
          reason={rejectReason}
          onReasonChange={setRejectReason}
          valid={rejectValid}
          busy={mutation.isPending}
          onConfirm={() =>
            run(
              { op: "reject", id: plan.id, reason: rejectReason.trim() },
              "Rencana vault ditolak",
              () => setRejectOpen(false),
            )
          }
          onDismiss={() => setRejectOpen(false)}
        />
      )}
    </div>
  );
}

function candidateLabel(c: VaultCandidate): string {
  const capacity = c.saldo_known ? formatDenoms(c.capacity) : "saldo tidak diketahui";
  const warn = c.capacity_warning ? " (peringatan)" : "";
  return `${c.branch_code} ${c.branch_name} · ${c.vendor_name} — kapasitas ${capacity}${warn}`;
}

interface AtmRowProps {
  planId: number;
  atm: VaultPlanAtm;
  canEdit: boolean;
  draft: DraftRow;
  onChange: (d: DraftRow) => void;
}

function AtmRow({ planId, atm, canEdit, draft, onChange }: AtmRowProps) {
  // ponytail: one candidates request per ATM row; add a batch endpoint if plans grow past ~100 ATMs.
  const candidates = useVaultCandidates(planId, atm.terminal_id, draft.isUrgent, canEdit);
  const selected = candidates.data?.find((c) => c.vendor_branch_id === draft.vaultBranchId);
  // FR4.2: an unassigned ATM starts on the top Tier 1 candidate (server sorts by tier, then
  // capacity). Suggested once, so clearing the choice afterwards sticks.
  const suggested = useRef(false);
  useEffect(() => {
    if (suggested.current || !canEdit || draft.vaultBranchId !== null || !candidates.data) return;
    suggested.current = true;
    const top = candidates.data.find((c) => c.tier === 1);
    if (top) onChange({ ...draft, vaultBranchId: top.vendor_branch_id });
  }, [canEdit, candidates.data, draft, onChange]);
  const saved = atm.assignment;
  const tiers = [1, 2, 3].filter((t) => candidates.data?.some((c) => c.tier === t));

  return (
    <tr className="border-[var(--n-100)] border-b align-top last:border-0">
      <td className="px-3 py-2 font-mono">{atm.terminal_id}</td>
      <td className="px-3 py-2">
        {atm.replenish_branch.code} {atm.replenish_branch.name}
        <div className="text-xs text-[var(--n-500)]">
          {atm.replenish_branch.vendor_name} · {atm.replenish_branch.region_code ?? "tanpa region"}
        </div>
      </td>
      <td className="px-3 py-2 text-right tabular-nums">{formatDenoms(atm.order)}</td>
      <td className="px-3 py-2">
        {canEdit ? (
          <div className="flex flex-col gap-2">
            <select
              aria-label={`Branch vault untuk ${atm.terminal_id}`}
              value={draft.vaultBranchId ?? ""}
              onChange={(e) =>
                onChange({
                  ...draft,
                  vaultBranchId: e.target.value ? Number(e.target.value) : null,
                })
              }
              className="min-h-[40px] max-w-[520px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-2 text-sm"
            >
              <option value="">
                {candidates.isLoading ? "Memuat kandidat…" : "— Pilih branch vault —"}
              </option>
              {/* A saved choice stays selectable even when it is no longer a candidate. */}
              {saved && !selected && draft.vaultBranchId === saved.vault_branch.id && (
                <option value={saved.vault_branch.id}>
                  {saved.vault_branch.code} {saved.vault_branch.name} (tersimpan)
                </option>
              )}
              {tiers.map((t) => (
                <optgroup key={t} label={TIER_LABELS[t]}>
                  {candidates.data
                    ?.filter((c) => c.tier === t)
                    .map((c) => (
                      <option key={c.vendor_branch_id} value={c.vendor_branch_id}>
                        {candidateLabel(c)}
                      </option>
                    ))}
                </optgroup>
              ))}
            </select>
            {selected?.capacity_warning && (
              <Badge
                variant="warning"
                icon={AlertTriangle}
                label={
                  selected.saldo_known
                    ? "Kapasitas kurang dari order"
                    : "Saldo vault tidak diketahui"
                }
              />
            )}
            <label className="flex items-center gap-2 text-sm text-[var(--n-700)]">
              <input
                type="checkbox"
                checked={draft.isUrgent}
                onChange={(e) => onChange({ ...draft, isUrgent: e.target.checked })}
              />
              Urgent (boleh branch region lain)
            </label>
            {draft.isUrgent && (
              <div className="flex flex-col gap-1">
                <textarea
                  aria-label={`Alasan urgent untuk ${atm.terminal_id}`}
                  value={draft.urgentReason}
                  maxLength={URGENT_REASON_MAX}
                  rows={2}
                  onChange={(e) => onChange({ ...draft, urgentReason: e.target.value })}
                  className="rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-2 py-1 text-sm"
                />
                <span
                  className={`text-xs ${urgentReasonValid(draft) ? "text-[var(--n-500)]" : "text-[var(--danger-fg)]"}`}
                >
                  Alasan wajib {URGENT_REASON_MIN}-{URGENT_REASON_MAX} karakter (
                  {draft.urgentReason.trim().length})
                </span>
              </div>
            )}
          </div>
        ) : saved ? (
          <AssignmentSummary assignment={saved} />
        ) : (
          <span className="text-[var(--n-500)]">Belum ditetapkan</span>
        )}
      </td>
    </tr>
  );
}

function AssignmentSummary({ assignment: a }: { assignment: VaultAssignmentView }) {
  return (
    <div className="flex flex-col gap-1">
      <span>
        {a.vault_branch.code} {a.vault_branch.name} · {a.vault_branch.vendor_name}
      </span>
      <span className="text-xs text-[var(--n-600)]">
        {TIER_LABELS[a.tier]} · kapasitas {formatDenoms(a.capacity_snapshot)}
      </span>
      {a.is_urgent && (
        <span className="text-xs text-[var(--n-600)]">Urgent: {a.urgent_reason}</span>
      )}
      {a.capacity_warning && (
        <Badge variant="warning" icon={AlertTriangle} label="Peringatan kapasitas" />
      )}
    </div>
  );
}
