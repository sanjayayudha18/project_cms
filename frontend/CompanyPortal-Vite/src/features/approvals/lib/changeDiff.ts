/**
 * Pure helpers that turn a staged master-data change ({op, payload, before})
 * into something a checker can read: labelled fields with before/after values.
 */

import type { MasterDataChange, MasterDataOp } from "../api";

export const ENTITY_LABELS: Record<string, string> = {
  vendor: "Vendor",
  vendor_branch: "Cabang vendor",
  vendor_vault: "Vault",
  vendor_pic: "PIC vendor",
  vendor_package: "Paket",
  atm: "ATM",
  atm_assignment: "Kelolaan ATM",
};

export const OP_LABELS: Record<MasterDataOp, string> = {
  create: "Baru",
  update: "Ubah",
  disable: "Nonaktifkan",
  enable: "Aktifkan",
};

export interface FieldDiff {
  field: string;
  label: string;
  before: string;
  after: string;
  changed: boolean;
}

export function entityLabel(entityType: string): string {
  return ENTITY_LABELS[entityType] ?? entityType;
}

/** "contact_email" -> "Contact email". */
export function humanizeField(field: string): string {
  const spaced = field.replace(/_/g, " ").trim();
  return spaced.charAt(0).toUpperCase() + spaced.slice(1);
}

const EMPTY = "—";

/** Display text for one value. Money/decimals stay the exact strings the API sent. */
export function formatValue(value: unknown): string {
  if (value === null || value === undefined || value === "") return EMPTY;
  if (typeof value === "boolean") return value ? "Ya" : "Tidak";
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}

/**
 * Field-by-field view of one change.
 * - create: every proposed field (before is empty).
 * - update: every proposed field with `changed` telling which differ from the snapshot.
 * - disable/enable: a single is_active row (the payload only carries the flip).
 */
export function diffFields(
  change: Pick<MasterDataChange, "op" | "payload" | "before">,
): FieldDiff[] {
  if (change.op === "disable" || change.op === "enable") {
    const enabling = change.op === "enable";
    return [
      {
        field: "is_active",
        label: "Status",
        before: enabling ? "Nonaktif" : "Aktif",
        after: enabling ? "Aktif" : "Nonaktif",
        changed: true,
      },
    ];
  }
  const payload = change.payload ?? {};
  const before = change.before ?? {};
  return Object.keys(payload).map((field) => {
    const after = formatValue(payload[field]);
    const prev = change.op === "create" ? EMPTY : formatValue(before[field]);
    return { field, label: humanizeField(field), before: prev, after, changed: after !== prev };
  });
}

/** Natural identifier of the record a change targets, for list rows. */
export function changeSubject(
  change: Pick<MasterDataChange, "payload" | "before" | "entity_id">,
): string {
  const sources = [change.payload, change.before];
  for (const key of ["code", "terminal_id", "vault_code", "branch_code", "name", "package_code"]) {
    for (const src of sources) {
      const v = src?.[key];
      if (typeof v === "string" && v !== "") return v;
    }
  }
  return change.entity_id !== null ? `#${change.entity_id}` : EMPTY;
}
