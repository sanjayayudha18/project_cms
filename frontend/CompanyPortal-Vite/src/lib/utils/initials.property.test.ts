// Feature: app-shell-revamp, Property 8: Inisial avatar
import * as fc from "fast-check";
import { describe, expect, it } from "vitest";
import { getInitials } from "./initials";

const LETTERS = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ";
const word = fc
  .array(fc.constantFrom(...LETTERS), { minLength: 1, maxLength: 10 })
  .map((a) => a.join(""));
const spaces = fc
  .array(fc.constantFrom(" ", "\t", "\n"), { minLength: 1, maxLength: 3 })
  .map((a) => a.join(""));

describe("Property 8: getInitials", () => {
  it('string tanpa karakter non-spasi → "?"', () => {
    fc.assert(
      fc.property(
        fc.array(fc.constantFrom(" ", "\t", "\n", " ")).map((a) => a.join("")),
        (s) => {
          expect(getInitials(s)).toBe("?");
        },
      ),
    );
  });

  it("string apa pun: hasil 1–2 karakter uppercase, atau ?", () => {
    fc.assert(
      fc.property(fc.string({ unit: "binary" }), (s) => {
        const out = getInitials(s);
        expect(out === "?" || (Array.from(out).length >= 1 && Array.from(out).length <= 2)).toBe(
          true,
        );
        expect(out).toBe(out.toUpperCase());
      }),
      { numRuns: 500 },
    );
  });

  it("nama berhuruf: inisial dari huruf pertama kata pertama dan terakhir", () => {
    fc.assert(
      fc.property(
        fc.array(word, { minLength: 2, maxLength: 5 }),
        fc.array(spaces, { minLength: 6, maxLength: 6 }),
        (words, seps) => {
          const name = (seps[0] ?? "") + words.map((w, i) => w + (seps[i + 1] ?? "")).join("");
          const first = words[0] ?? "";
          const last = words[words.length - 1] ?? "";
          expect(getInitials(name)).toBe(`${first.charAt(0)}${last.charAt(0)}`.toUpperCase());
        },
      ),
    );
  });

  it("satu kata: dua huruf pertamanya", () => {
    fc.assert(
      fc.property(word, spaces, (w, sp) => {
        expect(getInitials(sp + w + sp)).toBe(w.slice(0, 2).toUpperCase());
      }),
    );
  });
});
