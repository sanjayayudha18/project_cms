/**
 * Top-level before/after classification for an audit entry (Req 7, Property 11).
 * Nested values are compared structurally, so a nested object is one
 * "modified" row rather than a recursive diff.
 */

export type DiffStatus = "added" | "removed" | "modified" | "unchanged";

export interface DiffRow {
  key: string;
  status: DiffStatus;
  before: unknown;
  after: unknown;
}

type AuditValues = Record<string, unknown> | null;

function isEqual(a: unknown, b: unknown): boolean {
  if (a === b) return true;
  if (typeof a !== "object" || typeof b !== "object" || a === null || b === null) return false;
  if (Array.isArray(a) !== Array.isArray(b)) return false;
  const aKeys = Object.keys(a);
  const bKeys = Object.keys(b);
  if (aKeys.length !== bKeys.length) return false;
  return aKeys.every(
    (k) =>
      Object.hasOwn(b, k) &&
      isEqual((a as Record<string, unknown>)[k], (b as Record<string, unknown>)[k]),
  );
}

/** One row per top-level key in `before` then any keys only in `after`. A null side counts as empty. */
export function diffAuditValues(before: AuditValues, after: AuditValues): DiffRow[] {
  const b = before ?? {};
  const a = after ?? {};
  const keys = [...Object.keys(b), ...Object.keys(a).filter((k) => !Object.hasOwn(b, k))];

  return keys.map((key) => {
    const inBefore = Object.hasOwn(b, key);
    const inAfter = Object.hasOwn(a, key);
    let status: DiffStatus = "unchanged";
    if (!inBefore) status = "added";
    else if (!inAfter) status = "removed";
    else if (!isEqual(b[key], a[key])) status = "modified";
    return { key, status, before: b[key], after: a[key] };
  });
}
