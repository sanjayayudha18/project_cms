import { describe, expect, it } from "vitest";
import { pksStatusFromLabel } from "./PksStatusBadge";

// Disalin eksplisit dari pkg/reports.PksStatuses (key -> label CSV).
const GO_PKS_STATUSES: Record<string, string> = {
  aktif: "Aktif",
  need_renewal: "Need Renewal",
  expired: "Expired",
  terminated: "Terminated",
  belum_diisi: "Belum diisi",
};

describe("pksStatusFromLabel", () => {
  it.each(Object.entries(GO_PKS_STATUSES))("label %s -> key %s", (key, label) => {
    expect(pksStatusFromLabel(label)).toBe(key);
  });

  it("memetakan tepat lima label yang sama dengan Go", () => {
    const labels = Object.values(GO_PKS_STATUSES);
    expect(new Set(labels).size).toBe(5);
    expect(labels.map((l) => pksStatusFromLabel(l)).sort()).toEqual(
      Object.keys(GO_PKS_STATUSES).sort(),
    );
  });

  it.each(["", "aktif", "AKTIF", "Entah"])("tak dikenal %j -> undefined", (label) => {
    expect(pksStatusFromLabel(label)).toBeUndefined();
  });
});
