/**
 * Pure rules of the vault allocation editor (cit-vault-select FR4). The server
 * re-checks all of them; these only drive the on-screen hints and buttons.
 * Amounts are whole-IDR digit strings; Number is exact below 2^53.
 */

import type { AllocationInput, DenomAmounts, VaultAllocationView, VaultOption } from "./api";
import { formatAmount } from "./api";

export const REASON_MIN = 10;
export const REASON_MAX = 500;

export interface DraftRow {
  key: number;
  branchId: number | null;
  vaultId: number | null;
  amounts: DenomAmounts;
  isUrgent: boolean;
  urgentReason: string;
}

let nextKey = 0;

export function emptyRow(): DraftRow {
  nextKey += 1;
  return {
    key: nextKey,
    branchId: null,
    vaultId: null,
    amounts: {},
    isUrgent: false,
    urgentReason: "",
  };
}

export function rowsFromPlan(allocations: VaultAllocationView[]): DraftRow[] {
  if (allocations.length === 0) return [emptyRow()];
  return allocations.map((a) => ({
    ...emptyRow(),
    branchId: a.branch.id,
    vaultId: a.vendor_vault_id,
    amounts: { ...a.amounts },
    isUrgent: a.is_urgent,
    urgentReason: a.urgent_reason ?? "",
  }));
}

export function reasonValid(reason: string): boolean {
  const n = reason.trim().length;
  return n >= REASON_MIN && n <= REASON_MAX;
}

/** "Area <name>", without doubling when the name already starts with "Area". */
export function areaLabel(name: string): string {
  return /^area\b/i.test(name.trim()) ? name : `Area ${name}`;
}

export function denomLabel(denom: string): string {
  return `${Number(denom) / 1000}K`;
}

/** Empty and "0" mean "not allocated"; anything else must be a multiple of the denom. */
export function amountError(denom: string, raw: string): string | null {
  if (raw === "" || /^0+$/.test(raw)) return null;
  if (!/^\d+$/.test(raw)) return "Hanya angka rupiah";
  if (Number(raw) % Number(denom) !== 0) return `Harus kelipatan Rp${formatAmount(denom)}`;
  return null;
}

function filledAmounts(amounts: DenomAmounts): DenomAmounts {
  return Object.fromEntries(
    Object.entries(amounts)
      .filter(([, v]) => v !== "" && !/^0+$/.test(v))
      .sort(([a], [b]) => Number(b) - Number(a)),
  );
}

export function rowProblems(row: DraftRow): string[] {
  const out: string[] = [];
  if (row.vaultId === null) out.push("pilih vault");
  if (Object.keys(filledAmounts(row.amounts)).length === 0) out.push("isi minimal satu nominal");
  if (Object.entries(row.amounts).some(([d, v]) => amountError(d, v) !== null)) {
    out.push("nominal harus kelipatan pecahan");
  }
  if (row.isUrgent && !reasonValid(row.urgentReason)) {
    out.push(`alasan urgent ${REASON_MIN}-${REASON_MAX} karakter`);
  }
  return out;
}

/** The PUT body rows, normalised so drafts and saved rows compare equal. */
export function toInput(rows: DraftRow[]): AllocationInput[] {
  return rows
    .filter((r): r is DraftRow & { vaultId: number } => r.vaultId !== null)
    .map((r) => ({
      vendor_vault_id: r.vaultId,
      amounts: filledAmounts(r.amounts),
      is_urgent: r.isUrgent,
      urgent_reason: r.isUrgent ? r.urgentReason.trim() : null,
    }));
}

export function sameInput(a: AllocationInput[], b: AllocationInput[]): boolean {
  const key = (x: AllocationInput[]) =>
    JSON.stringify([...x].sort((p, q) => p.vendor_vault_id - q.vendor_vault_id));
  return key(a) === key(b);
}

export interface DenomCoverage {
  denom: string;
  order: number;
  allocated: number;
  /** allocated - order: negative = short, positive = buffer. */
  diff: number;
}

/** Order vs allocation per denom, high to low; denoms from both sides. */
export function coverageOf(order: DenomAmounts, rows: DraftRow[]): DenomCoverage[] {
  const allocated: Record<string, number> = {};
  for (const r of rows) {
    for (const [d, v] of Object.entries(filledAmounts(r.amounts))) {
      if (amountError(d, v) === null) allocated[d] = (allocated[d] ?? 0) + Number(v);
    }
  }
  const denoms = new Set([...Object.keys(order), ...Object.keys(allocated)]);
  return [...denoms]
    .sort((a, b) => Number(b) - Number(a))
    .map((d) => {
      const o = Number(order[d] ?? "0");
      const a = allocated[d] ?? 0;
      return { denom: d, order: o, allocated: a, diff: a - o };
    });
}

/** Capacity is a warning only (FR5.2), never a block. */
export function capacityIssue(row: DraftRow, vault: VaultOption | undefined): string | null {
  if (!vault) return null;
  if (!vault.saldo_known) return "Saldo vault tidak diketahui";
  const short = Object.entries(filledAmounts(row.amounts)).some(
    ([d, v]) => Number(v) > Number(vault.capacity?.[d] ?? "0"),
  );
  return short ? "Kapasitas vault kurang dari alokasi" : null;
}
