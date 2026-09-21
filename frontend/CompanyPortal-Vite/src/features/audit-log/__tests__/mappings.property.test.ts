import * as fc from "fast-check";
import { describe, expect, it } from "vitest";
import { type BadgeVariant, actionBadgeVariant } from "../types";

/**
 * Property 9 (partial — badge mapping half; icon+label is a rendering
 * concern for AuditLogTable, Task 7): For any action string,
 * actionBadgeVariant SHALL return one of the five defined variants.
 */
describe("actionBadgeVariant — Property 9: action-to-badge mapping", () => {
  const VALID_VARIANTS: BadgeVariant[] = ["success", "warning", "danger", "info", "neutral"];

  it("returns one of the five variants for any string", () => {
    fc.assert(
      fc.property(fc.string(), (action) => {
        expect(VALID_VARIANTS).toContain(actionBadgeVariant(action));
      }),
    );
  });

  // Regression: real audit_logs actions are bare verbs / snake_case, not dotted, and used to
  // all fall through to "neutral" because the regexes required a leading ".".
  it("classifies the bare-verb actions actually written to audit_logs", () => {
    expect(actionBadgeVariant("approve")).toBe("success");
    expect(actionBadgeVariant("create")).toBe("success");
    expect(actionBadgeVariant("reject")).toBe("danger");
    expect(actionBadgeVariant("submit")).toBe("info");
    expect(actionBadgeVariant("revise")).toBe("warning");
    expect(actionBadgeVariant("cancel")).toBe("warning");
    expect(actionBadgeVariant("admin_set_hierarchy")).toBe("warning");
  });

  it("classifies known suffixes correctly", () => {
    expect(actionBadgeVariant("approval.reject")).toBe("danger");
    expect(actionBadgeVariant("atm.delete")).toBe("danger");
    expect(actionBadgeVariant("dsr.upload.fail")).toBe("danger");
    expect(actionBadgeVariant("approval.approve")).toBe("success");
    expect(actionBadgeVariant("invoice.create")).toBe("success");
    expect(actionBadgeVariant("approval.submit")).toBe("info");
    expect(actionBadgeVariant("vendor.request")).toBe("info");
    expect(actionBadgeVariant("atm.update")).toBe("warning");
    expect(actionBadgeVariant("profile.edit")).toBe("warning");
    expect(actionBadgeVariant("unknown.action")).toBe("neutral");
    expect(actionBadgeVariant("")).toBe("neutral");
  });
});
