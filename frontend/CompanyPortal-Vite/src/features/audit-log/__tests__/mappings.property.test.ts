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
