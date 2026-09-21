import { describe, expect, it } from "vitest";
import { changeSubject, diffFields, formatValue, humanizeField } from "./changeDiff";

describe("diffFields", () => {
  it("update: marks only the fields that differ from the submit-time snapshot", () => {
    const diff = diffFields({
      op: "update",
      payload: { name: "Vendor Baru", contact_email: "a@v.id" },
      before: { name: "Vendor Lama", contact_email: "a@v.id", id: 1 },
    });

    expect(diff).toEqual([
      { field: "name", label: "Name", before: "Vendor Lama", after: "Vendor Baru", changed: true },
      {
        field: "contact_email",
        label: "Contact email",
        before: "a@v.id",
        after: "a@v.id",
        changed: false,
      },
    ]);
  });

  it("create: every proposed field is new, before is empty", () => {
    const diff = diffFields({ op: "create", payload: { code: "V9", name: "Baru" }, before: null });

    expect(diff.every((d) => d.changed && d.before === "—")).toBe(true);
    expect(diff.map((d) => d.after)).toEqual(["V9", "Baru"]);
  });

  it("disable/enable: a single Status row describing the flip", () => {
    expect(diffFields({ op: "disable", payload: null, before: { is_active: true } })).toEqual([
      { field: "is_active", label: "Status", before: "Aktif", after: "Nonaktif", changed: true },
    ]);
    expect(diffFields({ op: "enable", payload: null, before: null })[0]).toMatchObject({
      before: "Nonaktif",
      after: "Aktif",
    });
  });

  it("keeps decimal strings exact (money is never parsed to a number)", () => {
    const diff = diffFields({
      op: "update",
      payload: { capacity_amount: "500000000.00" },
      before: { capacity_amount: "500000000.0" },
    });

    expect(diff[0]).toMatchObject({ before: "500000000.0", after: "500000000.00", changed: true });
  });
});

describe("formatValue / humanizeField / changeSubject", () => {
  it("formats empty, boolean and text values", () => {
    expect(formatValue(null)).toBe("—");
    expect(formatValue("")).toBe("—");
    expect(formatValue(true)).toBe("Ya");
    expect(formatValue(false)).toBe("Tidak");
    expect(formatValue("x")).toBe("x");
  });

  it("humanizes snake_case field names", () => {
    expect(humanizeField("low_threshold_amount")).toBe("Low threshold amount");
  });

  it("picks a natural identifier, falling back to the entity id", () => {
    expect(
      changeSubject({ payload: { code: "V1", name: "X" }, before: null, entity_id: null }),
    ).toBe("V1");
    expect(changeSubject({ payload: null, before: { terminal_id: "T9" }, entity_id: 4 })).toBe(
      "T9",
    );
    expect(changeSubject({ payload: {}, before: null, entity_id: 4 })).toBe("#4");
  });
});
