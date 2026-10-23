// Feature: app-shell-revamp, Property 6: Preferensi shell aman dimuat
import * as fc from "fast-check";
import { describe, expect, it } from "vitest";
import { SIDEBAR_GROUP_ORDER } from "../config/navigation";
import { type ShellPrefsState, sanitizeShellPrefs } from "./prefs";

function expectValid(p: ShellPrefsState) {
  expect(typeof p.rail).toBe("boolean");
  expect(["dark", "light"]).toContain(p.sidebarTheme);
  expect(new Set(p.collapsedGroups).size).toBe(p.collapsedGroups.length);
  for (const g of p.collapsedGroups) expect(SIDEBAR_GROUP_ORDER).toContain(g);
}

// Objek yang "hampir valid": field benar bercampur nilai asing/duplikat.
const nearValid = fc.record(
  {
    rail: fc.oneof(fc.boolean(), fc.jsonValue()),
    sidebarTheme: fc.oneof(fc.constantFrom("dark", "light"), fc.jsonValue()),
    collapsedGroups: fc.oneof(
      fc.array(fc.oneof(fc.constantFrom(...SIDEBAR_GROUP_ORDER), fc.jsonValue())),
      fc.jsonValue(),
    ),
  },
  { requiredKeys: [] },
);

const validPrefs: fc.Arbitrary<ShellPrefsState> = fc.record({
  rail: fc.boolean(),
  sidebarTheme: fc.constantFrom("dark" as const, "light" as const),
  collapsedGroups: fc.subarray([...SIDEBAR_GROUP_ORDER]),
});

describe("Property 6: sanitizeShellPrefs", () => {
  it("selalu menghasilkan prefs valid untuk JSON acak", () => {
    fc.assert(
      fc.property(fc.oneof(fc.jsonValue(), nearValid), (raw) =>
        expectValid(sanitizeShellPrefs(raw)),
      ),
      { numRuns: 500 },
    );
  });

  it("idempoten", () => {
    fc.assert(
      fc.property(fc.oneof(fc.jsonValue(), nearValid), (raw) => {
        const once = sanitizeShellPrefs(raw);
        expect(sanitizeShellPrefs(once)).toEqual(once);
      }),
      { numRuns: 300 },
    );
  });

  it("round-trip persist (JSON) mempertahankan prefs valid", () => {
    fc.assert(
      fc.property(validPrefs, (p) => {
        expect(sanitizeShellPrefs(JSON.parse(JSON.stringify(p)))).toEqual(p);
      }),
      { numRuns: 200 },
    );
  });
});
