import { Button } from "@/components/ui/Button";
import { NoticeBanner } from "@/components/ui/NoticeBanner";
import { PageHeader } from "@/components/ui/PageHeader";
import { ReasonModal } from "@/features/vendor-request/VendorRequestDetail";
import { useAuthStore } from "@/lib/auth/store";
import { useToast } from "@/lib/hooks/useToast";
import { Link, useParams } from "@tanstack/react-router";
import { AlertTriangle } from "lucide-react";
import { useState } from "react";
import { AllocationRowEditor, AllocationTable, CoverageSummary } from "./AllocationRows";
import { PlanStatusBadge } from "./VaultPlanList";
import {
  type DraftRow,
  REASON_MAX,
  REASON_MIN,
  coverageOf,
  denomLabel,
  emptyRow,
  reasonValid,
  rowProblems,
  rowsFromPlan,
  sameInput,
  toInput,
} from "./allocation";
import {
  type VaultPlanDetail as VaultPlanDetailType,
  type VaultPlanMutation,
  formatDenoms,
  useVaultCandidates,
  useVaultPlan,
  useVaultPlanMutation,
} from "./api";

const REJECT_REASON_MAX = 500;

/** Detail penetapan vault (cit-vault-select FR4): vault rows per request, submit, approve/reject. */
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

/**
 * Why "Kirim untuk Approval" is off; empty = ready. Mirrors the server's submit
 * checks; submit sends the saved rows, so unsaved edits block it too.
 */
function submitBlockers(
  plan: VaultPlanDetailType,
  rows: DraftRow[],
  bufferReason: string,
): string[] {
  const out: string[] = [];
  if (rows.some((r) => rowProblems(r).length > 0)) out.push("Lengkapi setiap baris vault.");
  const lines = coverageOf(plan.order_totals, rows);
  const short = lines.filter((l) => l.diff < 0).map((l) => denomLabel(l.denom));
  if (short.length) out.push(`Alokasi ${short.join(", ")} masih kurang dari order.`);
  if (lines.some((l) => l.diff > 0) && !reasonValid(bufferReason)) {
    out.push(`Isi alasan buffer (${REASON_MIN}-${REASON_MAX} karakter).`);
  }
  const dirty =
    !sameInput(toInput(rows), toInput(rowsFromPlan(plan.allocations))) ||
    bufferReason.trim() !== (plan.buffer_reason ?? "");
  if (out.length === 0 && dirty) out.push("Simpan perubahan sebelum mengirim.");
  return out;
}

function PlanEditor({ plan }: { plan: VaultPlanDetailType }) {
  const user = useAuthStore((s) => s.user);
  const { toast } = useToast();
  const mutation = useVaultPlanMutation();
  const canEdit = plan.status === "draft" && user?.role === "ACM-USER";
  const candidates = useVaultCandidates(plan.id, canEdit);
  const [rows, setRows] = useState(() => rowsFromPlan(plan.allocations));
  const [bufferReason, setBufferReason] = useState(plan.buffer_reason ?? "");
  const [rejectOpen, setRejectOpen] = useState(false);
  const [rejectReason, setRejectReason] = useState("");

  const canDecide =
    plan.status === "pending_acm_approval" &&
    user?.role === "ACM-SPV" &&
    plan.submitted_by !== user.id;
  const lines = coverageOf(plan.order_totals, canEdit ? rows : rowsFromPlan(plan.allocations));
  const hasBuffer = lines.some((l) => l.diff > 0);
  // A reason typed while there was an excess is dropped once the excess is gone.
  const effectiveReason = hasBuffer ? bufferReason : "";
  const saveValid =
    rows.every((r) => rowProblems(r).length === 0) &&
    (effectiveReason.trim() === "" || reasonValid(effectiveReason));
  const blockers = submitBlockers(plan, rows, effectiveReason);
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
    const reason = effectiveReason.trim();
    run(
      {
        op: "save",
        id: plan.id,
        allocations: toInput(rows),
        bufferReason: reason === "" ? null : reason,
      },
      "Penetapan vault disimpan",
    );
  }

  function updateRow(i: number, row: DraftRow): void {
    setRows((cur) => cur.map((r, j) => (j === i ? row : r)));
  }

  return (
    <div className="flex flex-col gap-6 p-6">
      <PageHeader
        eyebrow="CIT · Penetapan Vault"
        title={plan.request_number}
        description={`Area ${plan.acm_area_name} · Region ${plan.region_code ?? "-"} · Tanggal replenish ${plan.replenish_date ?? "-"}`}
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

      <section aria-labelledby="atm-order-title" className="flex flex-col gap-2">
        <h2 id="atm-order-title" className="text-base font-semibold text-[var(--n-900)]">
          Order ATM
        </h2>
        <div className="overflow-x-auto rounded-[var(--radius-lg)] border border-[var(--n-200)]">
          <table className="w-full min-w-[640px] border-collapse text-sm">
            <thead>
              <tr className="border-[var(--n-200)] border-b bg-[var(--n-50)] text-left text-[var(--n-600)]">
                <th className="px-3 py-2 font-medium">ATM</th>
                <th className="px-3 py-2 font-medium">Cabang Replenish</th>
                <th className="px-3 py-2 text-right font-medium">Order (IDR)</th>
              </tr>
            </thead>
            <tbody>
              {plan.atms.map((atm) => (
                <tr key={atm.terminal_id} className="border-[var(--n-100)] border-b last:border-0">
                  <td className="px-3 py-2 font-mono">{atm.terminal_id}</td>
                  <td className="px-3 py-2">
                    {atm.replenish_branch.code} {atm.replenish_branch.name}
                    <span className="text-xs text-[var(--n-500)]">
                      {" "}
                      · {atm.replenish_branch.vendor_name}
                    </span>
                  </td>
                  <td className="px-3 py-2 text-right tabular-nums">{formatDenoms(atm.order)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section aria-labelledby="vault-rows-title" className="flex flex-col gap-3">
        <h2 id="vault-rows-title" className="text-base font-semibold text-[var(--n-900)]">
          Vault Pemenuhan
        </h2>
        {canEdit ? (
          <>
            {candidates.isError && (
              <p role="alert" className="text-sm text-[var(--danger-fg)]">
                Gagal memuat kandidat vault
              </p>
            )}
            {rows.map((row, i) => (
              <AllocationRowEditor
                key={row.key}
                index={i}
                row={row}
                denoms={lines.map((l) => l.denom)}
                branches={candidates.data ?? []}
                saved={plan.allocations}
                taken={
                  new Set(
                    rows.flatMap((r, j) => (j !== i && r.vaultId !== null ? [r.vaultId] : [])),
                  )
                }
                canRemove={rows.length > 1}
                onChange={(r) => updateRow(i, r)}
                onRemove={() => setRows((cur) => cur.filter((_, j) => j !== i))}
              />
            ))}
            <Button
              variant="secondary"
              className="self-start"
              onClick={() => setRows((cur) => [...cur, emptyRow()])}
            >
              Tambah Vault
            </Button>
          </>
        ) : plan.allocations.length > 0 ? (
          <AllocationTable allocations={plan.allocations} />
        ) : (
          <p className="text-sm text-[var(--n-500)]">Belum ada vault ditetapkan.</p>
        )}
      </section>

      <section aria-labelledby="coverage-title" className="flex flex-col gap-3">
        <h2 id="coverage-title" className="text-base font-semibold text-[var(--n-900)]">
          Ringkasan Order vs Alokasi
        </h2>
        <CoverageSummary lines={lines} />
        {canEdit && hasBuffer && (
          <label className="flex flex-col gap-1 text-sm font-medium text-[var(--n-700)]">
            Alasan buffer (alokasi melebihi order)
            <textarea
              value={bufferReason}
              maxLength={REASON_MAX}
              rows={2}
              onChange={(e) => setBufferReason(e.target.value)}
              className="rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-2 py-1 text-sm font-normal"
            />
            <span
              className={`text-xs font-normal ${reasonValid(bufferReason) ? "text-[var(--n-500)]" : "text-[var(--danger-fg)]"}`}
            >
              Wajib {REASON_MIN}-{REASON_MAX} karakter sebelum dikirim ({bufferReason.trim().length}
              )
            </span>
          </label>
        )}
        {!canEdit && plan.buffer_reason && (
          <p className="text-sm text-[var(--n-700)]">Alasan buffer: {plan.buffer_reason}</p>
        )}
      </section>

      {canEdit && blockers.length > 0 && (
        <ul
          aria-label="Syarat kirim"
          className="list-disc self-end pl-5 text-xs text-[var(--n-600)]"
        >
          {blockers.map((b) => (
            <li key={b}>{b}</li>
          ))}
        </ul>
      )}
      <div className="flex flex-wrap justify-end gap-3">
        {canEdit && (
          <>
            <Button variant="secondary" disabled={mutation.isPending || !saveValid} onClick={save}>
              Simpan
            </Button>
            <Button
              disabled={mutation.isPending || blockers.length > 0}
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
