import { describe, expect, it } from "vitest";
import { auditKeys } from "../hooks/useAuditQueries";
import { EMPTY_AUDIT_LOG_FILTERS } from "../types";

/**
 * Query-key stability (Task 6 test checklist): the same logical inputs must
 * produce the same key (deep-equal — TanStack Query hashes keys structurally,
 * not by reference), and any changed input must produce a different key so
 * filter/page/pageSize changes actually trigger a refetch instead of being
 * masked by cache hits.
 */
describe("auditKeys — query-key stability", () => {
  it("list() is deep-equal for identical inputs across calls", () => {
    const a = auditKeys.list(EMPTY_AUDIT_LOG_FILTERS, 1, 25);
    const b = auditKeys.list({ ...EMPTY_AUDIT_LOG_FILTERS }, 1, 25);
    expect(a).toEqual(b);
  });

  it("list() changes when page changes", () => {
    const a = auditKeys.list(EMPTY_AUDIT_LOG_FILTERS, 1, 25);
    const b = auditKeys.list(EMPTY_AUDIT_LOG_FILTERS, 2, 25);
    expect(a).not.toEqual(b);
  });

  it("list() changes when pageSize changes", () => {
    const a = auditKeys.list(EMPTY_AUDIT_LOG_FILTERS, 1, 25);
    const b = auditKeys.list(EMPTY_AUDIT_LOG_FILTERS, 1, 50);
    expect(a).not.toEqual(b);
  });

  it("list() changes when any filter changes", () => {
    const a = auditKeys.list(EMPTY_AUDIT_LOG_FILTERS, 1, 25);
    const b = auditKeys.list({ ...EMPTY_AUDIT_LOG_FILTERS, action: "approval.submit" }, 1, 25);
    expect(a).not.toEqual(b);
  });

  it("list() keys are always prefixed by the shared 'audit-log' root", () => {
    const key = auditKeys.list(EMPTY_AUDIT_LOG_FILTERS, 1, 25);
    expect(key[0]).toBe("audit-log");
    expect(key.slice(0, 2)).toEqual(["audit-log", "list"]);
  });

  it("detail() is deep-equal for the same id and changes for a different id", () => {
    expect(auditKeys.detail(42)).toEqual(auditKeys.detail(42));
    expect(auditKeys.detail(42)).not.toEqual(auditKeys.detail(43));
  });

  it("detail(null) is stable and distinct from any numeric id", () => {
    expect(auditKeys.detail(null)).toEqual(auditKeys.detail(null));
    expect(auditKeys.detail(null)).not.toEqual(auditKeys.detail(0));
  });

  it("list and detail keys never collide", () => {
    const list = auditKeys.list(EMPTY_AUDIT_LOG_FILTERS, 1, 25);
    const detail = auditKeys.detail(1);
    expect(list).not.toEqual(detail);
  });
});
