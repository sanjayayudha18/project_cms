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
  dsr_location_map: "Mapping lokasi DSR",
  vendor_pks: "PKS vendor",
};

export const OP_LABELS: Record<MasterDataOp, string> = {
  create: "Baru",
  update: "Ubah",
  disable: "Nonaktifkan",
  enable: "Aktifkan",
};

/** Per-entity wording where disable/enable are not "Nonaktifkan"/"Aktifkan". */
const ENTITY_OP_LABELS: Record<string, Partial<Record<MasterDataOp, string>>> = {
  vendor_pks: { disable: "Terminate PKS", enable: "Batalkan terminate" },
};

export function opLabel(entityType: string, op: MasterDataOp): string {
  return ENTITY_OP_LABELS[entityType]?.[op] ?? OP_LABELS[op];
}

/** Fields whose raw name reads badly after humanizeField. */
const FIELD_LABELS: Record<string, string> = {
  is_director: "Direktur Utama",
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
  change: Pick<MasterDataChange, "op" | "payload" | "before"> & { entity_type?: string },
): FieldDiff[] {
  if (change.entity_type === "vendor_pks" && (change.op === "disable" || change.op === "enable")) {
    // Terminate / cancel terminate: show which PKS and why (payload carries only vendor_id + reason).
    const pks = formatValue(change.before?.pks_name ?? change.payload?.pks_name);
    const reason = formatValue(change.payload?.reason);
    return [
      { field: "pks_name", label: "Nama PKS", before: pks, after: pks, changed: false },
      { field: "reason", label: "Alasan", before: EMPTY, after: reason, changed: true },
    ];
  }
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
    return {
      field,
      label: FIELD_LABELS[field] ?? humanizeField(field),
      before: prev,
      after,
      changed: after !== prev,
    };
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
