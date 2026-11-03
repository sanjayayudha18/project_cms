// Feature: app-shell-revamp, Property 7: Shortcut menghormati target editable
import * as fc from "fast-check";
import { describe, expect, it } from "vitest";
import { matchShellShortcut } from "./shortcuts";

type TargetKind =
  | "none"
  | "plain"
  | "button"
  | "input"
  | "textarea"
  | "select"
  | "contenteditable"
  | "combobox"
  | "insideEditable";

const EDITABLE: TargetKind[] = [
  "input",
  "textarea",
  "select",
  "contenteditable",
  "combobox",
  "insideEditable",
];

function withAttr(attr: string, value: string): HTMLElement {
  const el = document.createElement("div");
  el.setAttribute(attr, value);
  return el;
}

function makeTarget(kind: TargetKind): EventTarget | null {
  switch (kind) {
    case "none":
      return null;
    case "plain":
      return document.createElement("div");
    case "button":
      return document.createElement("button");
    case "input":
      return document.createElement("input");
    case "textarea":
      return document.createElement("textarea");
    case "select":
      return document.createElement("select");
    case "contenteditable":
      return withAttr("contenteditable", "true");
    case "combobox":
      return withAttr("role", "combobox");
    case "insideEditable": {
      const host = withAttr("contenteditable", "true");
      const child = document.createElement("span");
      host.append(child);
      return child;
    }
  }
}

const kind = fc.constantFrom<TargetKind>(
  "none",
  "plain",
  "button",
  "input",
  "textarea",
  "select",
  "contenteditable",
  "combobox",
  "insideEditable",
);
const key = fc.oneof(
  fc.constantFrom("k", "K", "[", "]", "{", "Enter", "Escape", "a"),
  fc.string({ minLength: 1, maxLength: 3 }),
);
const event = fc.record({
  key,
  ctrlKey: fc.boolean(),
  metaKey: fc.boolean(),
  altKey: fc.boolean(),
  shiftKey: fc.boolean(),
  kind,
});

describe("Property 7: matchShellShortcut", () => {
  it("cocok dengan model referensi untuk kombinasi key/modifier/target acak", () => {
    fc.assert(
      fc.property(event, ({ kind: k, ...e }) => {
        const noMods = !e.ctrlKey && !e.metaKey && !e.altKey && !e.shiftKey;
        const expected =
          e.key.toLowerCase() === "k" && (e.ctrlKey || e.metaKey)
            ? "palette"
            : e.key === "[" && noMods && !EDITABLE.includes(k)
              ? "toggle-rail"
              : null;
        expect(matchShellShortcut({ ...e, target: makeTarget(k) })).toBe(expected);
      }),
      { numRuns: 500 },
    );
  });

  it("palette aktif di target mana pun, termasuk input", () => {
    fc.assert(
      fc.property(
        kind,
        fc.constantFrom("k", "K"),
        fc.constantFrom(
          { ctrlKey: true, metaKey: false },
          { ctrlKey: false, metaKey: true },
          { ctrlKey: true, metaKey: true },
        ),
        fc.boolean(),
        fc.boolean(),
        (k, kk, mods, altKey, shiftKey) => {
          expect(
            matchShellShortcut({ key: kk, ...mods, altKey, shiftKey, target: makeTarget(k) }),
          ).toBe("palette");
        },
      ),
    );
  });

  it("toggle-rail tidak pernah aktif di target editable", () => {
    fc.assert(
      fc.property(fc.constantFrom(...EDITABLE), (k) => {
        const e = {
          key: "[",
          ctrlKey: false,
          metaKey: false,
          altKey: false,
          shiftKey: false,
          target: makeTarget(k),
        };
        expect(matchShellShortcut(e)).toBeNull();
      }),
    );
  });
});
