import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { AlertTriangle, CheckCircle2 } from "lucide-react";
import {
  type DenomCoverage,
  type DraftRow,
  REASON_MAX,
  REASON_MIN,
  amountError,
  capacityIssue,
  denomLabel,
  reasonValid,
} from "./allocation";
import {
  TIER_LABELS,
  type VaultAllocationView,
  type VaultBranchOption,
  type VaultOption,
  formatAmount,
  formatDenoms,
} from "./api";

const fieldClass =
  "min-h-[40px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-2 text-sm";
const thClass = "px-3 py-2 font-medium";

function vaultLabel(v: VaultOption): string {
  const capacity = v.saldo_known ? formatDenoms(v.capacity) : "saldo tidak diketahui";
  return `${v.vault_code} · ${v.category} — kapasitas ${capacity}`;
}

interface RowEditorProps {
  index: number;
  row: DraftRow;
  denoms: string[];
  branches: VaultBranchOption[];
  /** Saved rows: fallback option when a saved vault is no longer a candidate, and the ticket. */
  saved: VaultAllocationView[];
  /** Vaults picked on other rows (one row per vault). */
  taken: Set<number>;
  canRemove: boolean;
  onChange: (row: DraftRow) => void;
  onRemove: () => void;
}

/** One vault row: branch -> vault, amount per denom, urgent (cit-vault-select FR4.1). */
export function AllocationRowEditor({
  index,
  row,
  denoms,
  branches,
  saved,
  taken,
  canRemove,
  onChange,
  onRemove,
}: RowEditorProps) {
  const n = index + 1;
  // Tier 3 (other region) is only offered on urgent rows (FR3.3).
  const visible = branches.filter((b) => row.isUrgent || b.tier !== 3);
  const tiers = [1, 2, 3].filter((t) => visible.some((b) => b.tier === t));
  const branch = branches.find((b) => b.vendor_branch_id === row.branchId);
  const vault = branch?.vaults.find((v) => v.vendor_vault_id === row.vaultId);
  const savedRow = saved.find((a) => a.vendor_vault_id === row.vaultId);
  const stale = !vault && savedRow ? savedRow : undefined;
  const capacity = capacityIssue(row, vault);

  function pickBranch(value: string): void {
    const b = branches.find((x) => x.vendor_branch_id === Number(value));
    const free = b?.vaults.filter((v) => !taken.has(v.vendor_vault_id)) ?? [];
    onChange({
      ...row,
      branchId: b ? b.vendor_branch_id : null,
      vaultId: free.length === 1 ? (free[0]?.vendor_vault_id ?? null) : null,
    });
  }

  function toggleUrgent(checked: boolean): void {
    const drop = !checked && branch?.tier === 3;
    onChange({
      ...row,
      isUrgent: checked,
      branchId: drop ? null : row.branchId,
      vaultId: drop ? null : row.vaultId,
    });
  }

  return (
    <fieldset className="flex flex-col gap-3 rounded-[var(--radius-lg)] border border-[var(--n-200)] p-4">
      <legend className="px-1 text-sm font-semibold text-[var(--n-800)]">Vault {n}</legend>
      <div className="flex flex-wrap items-start gap-3">
        <select
          aria-label={`Branch vault baris ${n}`}
          value={row.branchId ?? ""}
          onChange={(e) => pickBranch(e.target.value)}
          className={`${fieldClass} min-w-[260px]`}
        >
          <option value="">— Pilih branch vault —</option>
          {stale && !branch && (
            <option value={stale.branch.id}>
              {stale.branch.code} {stale.branch.name} (tersimpan)
            </option>
          )}
          {tiers.map((t) => (
            <optgroup key={t} label={TIER_LABELS[t]}>
              {visible
                .filter((b) => b.tier === t)
                .map((b) => (
                  <option key={b.vendor_branch_id} value={b.vendor_branch_id}>
                    {b.branch_code} {b.branch_name} · {b.vendor_name} · {b.region_code}
                  </option>
                ))}
            </optgroup>
          ))}
        </select>
        <select
          aria-label={`Vault baris ${n}`}
          value={row.vaultId ?? ""}
          disabled={row.branchId === null}
          onChange={(e) =>
            onChange({ ...row, vaultId: e.target.value ? Number(e.target.value) : null })
          }
          className={`${fieldClass} min-w-[260px]`}
        >
          <option value="">— Pilih vault —</option>
          {stale && (
            <option value={stale.vendor_vault_id}>
              {stale.vault_code} (tersimpan, tidak lagi tersedia)
            </option>
          )}
          {branch?.vaults
            .filter((v) => v.vendor_vault_id === row.vaultId || !taken.has(v.vendor_vault_id))
            .map((v) => (
              <option key={v.vendor_vault_id} value={v.vendor_vault_id}>
                {vaultLabel(v)}
              </option>
            ))}
        </select>
        {savedRow?.ticket_number && (
          <span className="self-center font-mono text-xs text-[var(--n-700)]">
            Tiket {savedRow.ticket_number}
          </span>
        )}
        {canRemove && (
          <Button variant="ghost" className="ml-auto" onClick={onRemove}>
            Hapus<span className="sr-only"> baris {n}</span>
          </Button>
        )}
      </div>

      <div className="flex flex-wrap gap-3">
        {denoms.map((d) => {
          const value = row.amounts[d] ?? "";
          const err = amountError(d, value);
          return (
            <label key={d} className="flex w-[200px] flex-col gap-1 text-xs text-[var(--n-700)]">
              Pecahan {denomLabel(d)} (IDR)
              <input
                aria-label={`Nominal ${denomLabel(d)} baris ${n}`}
                inputMode="numeric"
                value={value}
                onChange={(e) =>
                  onChange({
                    ...row,
                    // Digits only, no leading zero: the saved value must compare equal to the server's.
                    amounts: {
                      ...row.amounts,
                      [d]: e.target.value.replace(/\D/g, "").replace(/^0+(?=\d)/, ""),
                    },
                  })
                }
                className={`${fieldClass} text-right tabular-nums`}
              />
              {err ? (
                <span className="text-[var(--danger-fg)]">{err}</span>
              ) : (
                value !== "" && (
                  <span className="text-right tabular-nums">Rp {formatAmount(value)}</span>
                )
              )}
            </label>
          );
        })}
      </div>

      {capacity && <Badge variant="warning" icon={AlertTriangle} label={capacity} />}

      <label className="flex items-center gap-2 text-sm text-[var(--n-700)]">
        <input
          type="checkbox"
          checked={row.isUrgent}
          onChange={(e) => toggleUrgent(e.target.checked)}
        />
        Urgent (boleh branch region lain)
      </label>
      {row.isUrgent && (
        <div className="flex flex-col gap-1">
          <textarea
            aria-label={`Alasan urgent baris ${n}`}
            value={row.urgentReason}
            maxLength={REASON_MAX}
            rows={2}
            onChange={(e) => onChange({ ...row, urgentReason: e.target.value })}
            className="rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-2 py-1 text-sm"
          />
          <span
            className={`text-xs ${reasonValid(row.urgentReason) ? "text-[var(--n-500)]" : "text-[var(--danger-fg)]"}`}
          >
            Alasan wajib {REASON_MIN}-{REASON_MAX} karakter ({row.urgentReason.trim().length})
          </span>
        </div>
      )}
    </fieldset>
  );
}

function signed(n: number): string {
  if (n === 0) return "0";
  return `${n < 0 ? "−" : "+"}${formatAmount(String(Math.abs(n)))}`;
}

/** Order vs allocation per denom (FR4.3): short is labelled, never colour alone. */
export function CoverageSummary({ lines }: { lines: DenomCoverage[] }) {
  return (
    <div className="overflow-x-auto rounded-[var(--radius-lg)] border border-[var(--n-200)]">
      <table className="w-full min-w-[640px] border-collapse text-sm">
        <caption className="sr-only">Ringkasan order vs alokasi per pecahan</caption>
        <thead>
          <tr className="border-[var(--n-200)] border-b bg-[var(--n-50)] text-left text-[var(--n-600)]">
            <th className={thClass}>Pecahan</th>
            <th className={`${thClass} text-right`}>Order (IDR)</th>
            <th className={`${thClass} text-right`}>Alokasi (IDR)</th>
            <th className={`${thClass} text-right`}>Selisih (IDR)</th>
            <th className={thClass}>Status</th>
          </tr>
        </thead>
        <tbody>
          {lines.map((l) => (
            <tr key={l.denom} className="border-[var(--n-100)] border-b last:border-0">
              <td className="px-3 py-2">{denomLabel(l.denom)}</td>
              <td className="px-3 py-2 text-right tabular-nums">{formatAmount(String(l.order))}</td>
              <td className="px-3 py-2 text-right tabular-nums">
                {formatAmount(String(l.allocated))}
              </td>
              <td
                className={`px-3 py-2 text-right tabular-nums ${l.diff < 0 ? "font-semibold text-[var(--danger-fg)]" : ""}`}
              >
                {signed(l.diff)}
              </td>
              <td className="px-3 py-2">
                {l.diff < 0 && <Badge variant="danger" icon={AlertTriangle} label="Kurang" />}
                {l.diff > 0 && <Badge variant="info" label="Buffer" />}
                {l.diff === 0 && <Badge variant="success" icon={CheckCircle2} label="Sesuai" />}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

/** Saved allocations, read-only (ACM-SPV, ATM-SPV review, after submit). */
export function AllocationTable({ allocations }: { allocations: VaultAllocationView[] }) {
  return (
    <div className="overflow-x-auto rounded-[var(--radius-lg)] border border-[var(--n-200)]">
      <table className="w-full min-w-[960px] border-collapse text-sm">
        <thead>
          <tr className="border-[var(--n-200)] border-b bg-[var(--n-50)] text-left text-[var(--n-600)]">
            <th className={thClass}>Branch Vault</th>
            <th className={thClass}>Vault</th>
            <th className={thClass}>Tier</th>
            <th className={`${thClass} text-right`}>Alokasi (IDR)</th>
            <th className={`${thClass} text-right`}>Kapasitas (IDR)</th>
            <th className={thClass}>Tiket</th>
          </tr>
        </thead>
        <tbody>
          {allocations.map((a) => (
            <tr key={a.id} className="border-[var(--n-100)] border-b align-top last:border-0">
              <td className="px-3 py-2">
                {a.branch.code} {a.branch.name}
                <div className="text-xs text-[var(--n-500)]">
                  {a.branch.vendor_name} · {a.branch.region_code ?? "tanpa region"}
                </div>
              </td>
              <td className="px-3 py-2 font-mono">
                {a.vault_code}
                <div className="font-sans text-xs text-[var(--n-500)]">{a.vault_category}</div>
              </td>
              <td className="px-3 py-2">
                {TIER_LABELS[a.tier]}
                {a.is_urgent && (
                  <div className="text-xs text-[var(--n-600)]">Urgent: {a.urgent_reason}</div>
                )}
              </td>
              <td className="px-3 py-2 text-right tabular-nums">{formatDenoms(a.amounts)}</td>
              <td className="px-3 py-2 text-right tabular-nums">
                {formatDenoms(a.capacity_snapshot)}
                {a.capacity_warning && (
                  <div className="mt-1 flex justify-end">
                    <Badge variant="warning" icon={AlertTriangle} label="Peringatan kapasitas" />
                  </div>
                )}
              </td>
              <td className="px-3 py-2 font-mono text-xs">
                {a.ticket_number ?? "Terbit saat kirim"}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
