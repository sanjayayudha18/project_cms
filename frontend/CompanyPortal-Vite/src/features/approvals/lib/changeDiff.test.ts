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

describe("vendor_pks labels and diff (vendor-pks Req 10.1, 11.4)", () => {
  it("labels the entity and the per-entity op wording", async () => {
    const { ENTITY_LABELS, OP_LABELS, opLabel } = await import("./changeDiff");

    expect(ENTITY_LABELS.vendor_pks).toBe("PKS vendor");
    expect(opLabel("vendor_pks", "disable")).toBe("Terminate PKS");
    expect(opLabel("vendor_pks", "enable")).toBe("Batalkan terminate");
    expect(opLabel("vendor_pks", "create")).toBe(OP_LABELS.create);
    expect(opLabel("vendor", "disable")).toBe(OP_LABELS.disable);
  });

  it("terminate diff names the PKS (from the snapshot) and the reason", () => {
    const diff = diffFields({
      entity_type: "vendor_pks",
      op: "disable",
      payload: { vendor_id: 42, reason: "Kontrak diputus" },
      before: { pks_name: "PKS 2026" },
    });

    expect(diff).toEqual([
      {
        field: "pks_name",
        label: "Nama PKS",
        before: "PKS 2026",
        after: "PKS 2026",
        changed: false,
      },
      { field: "reason", label: "Alasan", before: "—", after: "Kontrak diputus", changed: true },
    ]);
  });

  it("cancel-terminate diff has the same shape", () => {
    const diff = diffFields({
      entity_type: "vendor_pks",
      op: "enable",
      payload: { reason: "Salah input" },
      before: { pks_name: "PKS 2026" },
    });

    expect(diff.map((d) => d.field)).toEqual(["pks_name", "reason"]);
    expect(diff[1].after).toBe("Salah input");
  });

  it("other entities keep the plain is_active diff for disable", () => {
    const diff = diffFields({
      entity_type: "vendor",
      op: "disable",
      payload: {},
      before: { is_active: true },
    });

    expect(diff).toHaveLength(1);
    expect(diff[0].field).toBe("is_active");
  });

  it("labels is_director on a PIC diff as 'Direktur Utama'", () => {
    const diff = diffFields({
      entity_type: "vendor_pic",
      op: "update",
      payload: { is_director: true },
      before: { is_director: false },
    });

    expect(diff).toEqual([
      {
        field: "is_director",
        label: "Direktur Utama",
        before: "Tidak",
        after: "Ya",
        changed: true,
      },
    ]);
  });
});
