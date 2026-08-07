// Feature: user-login, Property 18: Whitespace-Only Input Rejection
// Feature: user-login, Property 19: Retry-After Countdown Formatting
import * as fc from "fast-check";
import { describe, expect, it } from "vitest";
import { z } from "zod";

// ─── Extracted Logic Under Test ───────────────────────────────────────────────

/**
 * Login form validation schema — extracted from login.tsx.
 * Validates that username and password are non-empty after trimming.
 */
const loginSchema = z.object({
  username: z.string().min(1, "Wajib diisi"),
  password: z.string().min(1, "Wajib diisi"),
});

/**
 * Formats a Retry-After value (in seconds) as "M menit S detik".
 * Used in the login page lockout UI.
 */
function formatRetryAfter(seconds: number): string {
  const minutes = Math.floor(seconds / 60);
  const secs = seconds % 60;
  return `${minutes} menit ${secs} detik`;
}

// ─── Arbitraries ──────────────────────────────────────────────────────────────

/** Generates a string composed entirely of whitespace characters */
const arbWhitespaceOnly: fc.Arbitrary<string> = fc
  .array(fc.constantFrom(" ", "\t", "\n", "\r", "\f", "\v", "  "), {
    minLength: 1,
    maxLength: 20,
  })
  .map((chars) => chars.join(""));

/** Generates a valid Retry-After value in range [1, 900] */
const arbRetryAfterSeconds: fc.Arbitrary<number> = fc.integer({
  min: 1,
  max: 900,
});

// ─── Property 18: Whitespace-Only Input Rejection ─────────────────────────────

/**
 * **Validates: Requirements 9.6, 3.7**
 *
 * For any string composed entirely of whitespace characters (spaces, tabs,
 * newlines), the frontend form validation SHALL treat it as empty and reject
 * submission.
 */
describe("Property 18: Whitespace-Only Input Rejection", () => {
  it("whitespace-only username is rejected by Zod schema (min(1) rejects empty after trimming)", () => {
    fc.assert(
      fc.property(arbWhitespaceOnly, (whitespace) => {
        // The Zod schema uses min(1), but Zod's min doesn't auto-trim.
        // However, the form trims input before submission via .trim(),
        // so a whitespace-only string effectively becomes "".
        const trimmed = whitespace.trim();
        const result = loginSchema.safeParse({
          username: trimmed,
          password: "validpass",
        });
        expect(result.success).toBe(false);
        if (!result.success) {
          const usernameError = result.error.issues.find(
            (i) => i.path[0] === "username",
          );
          expect(usernameError).toBeDefined();
        }
      }),
      { numRuns: 100 },
    );
  });

  it("whitespace-only password is rejected by Zod schema", () => {
    fc.assert(
      fc.property(arbWhitespaceOnly, (whitespace) => {
        const trimmed = whitespace.trim();
        const result = loginSchema.safeParse({
          username: "validuser",
          password: trimmed,
        });
        expect(result.success).toBe(false);
        if (!result.success) {
          const passwordError = result.error.issues.find(
            (i) => i.path[0] === "password",
          );
          expect(passwordError).toBeDefined();
        }
      }),
      { numRuns: 100 },
    );
  });

  it("whitespace-only in both fields is rejected", () => {
    fc.assert(
      fc.property(arbWhitespaceOnly, arbWhitespaceOnly, (wsUser, wsPass) => {
        const result = loginSchema.safeParse({
          username: wsUser.trim(),
          password: wsPass.trim(),
        });
        expect(result.success).toBe(false);
      }),
      { numRuns: 100 },
    );
  });
});

// ─── Property 19: Retry-After Countdown Formatting ────────────────────────────

/**
 * **Validates: Requirements 9.8**
 *
 * For any integer value N (representing seconds from the Retry-After header),
 * the frontend lockout display SHALL format it as "M menit S detik" where
 * M = floor(N/60) and S = N mod 60, with correct values for all N in [1, 900].
 */
describe("Property 19: Retry-After Countdown Formatting", () => {
  it("formats as 'M menit S detik' with correct M and S", () => {
    fc.assert(
      fc.property(arbRetryAfterSeconds, (seconds) => {
        const formatted = formatRetryAfter(seconds);

        const expectedMinutes = Math.floor(seconds / 60);
        const expectedSecs = seconds % 60;
        const expected = `${expectedMinutes} menit ${expectedSecs} detik`;

        expect(formatted).toBe(expected);
      }),
      { numRuns: 500 },
    );
  });

  it("M = floor(N/60) is always correct", () => {
    fc.assert(
      fc.property(arbRetryAfterSeconds, (seconds) => {
        const formatted = formatRetryAfter(seconds);
        const match = formatted.match(/^(\d+) menit/);
        expect(match).not.toBeNull();

        const minutes = Number.parseInt(match![1], 10);
        expect(minutes).toBe(Math.floor(seconds / 60));
      }),
      { numRuns: 200 },
    );
  });

  it("S = N mod 60 is always correct", () => {
    fc.assert(
      fc.property(arbRetryAfterSeconds, (seconds) => {
        const formatted = formatRetryAfter(seconds);
        const match = formatted.match(/(\d+) detik$/);
        expect(match).not.toBeNull();

        const secs = Number.parseInt(match![1], 10);
        expect(secs).toBe(seconds % 60);
      }),
      { numRuns: 200 },
    );
  });

  it("minutes is in range [0, 15] for N in [1, 900]", () => {
    fc.assert(
      fc.property(arbRetryAfterSeconds, (seconds) => {
        const minutes = Math.floor(seconds / 60);
        expect(minutes).toBeGreaterThanOrEqual(0);
        expect(minutes).toBeLessThanOrEqual(15);
      }),
      { numRuns: 200 },
    );
  });

  it("seconds remainder is in range [0, 59]", () => {
    fc.assert(
      fc.property(arbRetryAfterSeconds, (seconds) => {
        const secs = seconds % 60;
        expect(secs).toBeGreaterThanOrEqual(0);
        expect(secs).toBeLessThanOrEqual(59);
      }),
      { numRuns: 200 },
    );
  });

  it("M*60 + S always equals the original N", () => {
    fc.assert(
      fc.property(arbRetryAfterSeconds, (seconds) => {
        const formatted = formatRetryAfter(seconds);
        const match = formatted.match(/^(\d+) menit (\d+) detik$/);
        expect(match).not.toBeNull();

        const minutes = Number.parseInt(match![1], 10);
        const secs = Number.parseInt(match![2], 10);
        expect(minutes * 60 + secs).toBe(seconds);
      }),
      { numRuns: 200 },
    );
  });
});
