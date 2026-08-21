// Feature: user-login, Property 18: Whitespace-Only Input Rejection
// Feature: user-login, Property 19: Retry-After Countdown Formatting
// Feature: revamp-login, max-length and countdown clamp boundaries
import * as fc from "fast-check";
import { describe, expect, it } from "vitest";
import { formatRetryAfter, loginSchema, remainingRateLimitSeconds } from "./login";

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

const arbNonEmptyCredential: fc.Arbitrary<string> = fc
  .string({ minLength: 1, maxLength: 64 })
  .filter((s) => s.trim().length > 0);

// ─── Property 18: Whitespace-Only Input Rejection ─────────────────────────────

/**
 * For any string composed entirely of whitespace characters, the exported
 * loginSchema SHALL reject submission with a field error (no pre-trim in the test).
 */
describe("Property 18: Whitespace-Only Input Rejection", () => {
  it("whitespace-only username is rejected by exported loginSchema", () => {
    fc.assert(
      fc.property(arbWhitespaceOnly, (whitespace) => {
        const result = loginSchema.safeParse({
          username: whitespace,
          password: "validpass",
        });
        expect(result.success).toBe(false);
        if (!result.success) {
          const usernameError = result.error.issues.find((i) => i.path[0] === "username");
          expect(usernameError).toBeDefined();
          expect(usernameError?.message).toBe("Wajib diisi");
        }
      }),
      { numRuns: 100 },
    );
  });

  it("whitespace-only password is rejected by exported loginSchema", () => {
    fc.assert(
      fc.property(arbWhitespaceOnly, (whitespace) => {
        const result = loginSchema.safeParse({
          username: "validuser",
          password: whitespace,
        });
        expect(result.success).toBe(false);
        if (!result.success) {
          const passwordError = result.error.issues.find((i) => i.path[0] === "password");
          expect(passwordError).toBeDefined();
          expect(passwordError?.message).toBe("Wajib diisi");
        }
      }),
      { numRuns: 100 },
    );
  });

  it("whitespace-only in both fields is rejected", () => {
    fc.assert(
      fc.property(arbWhitespaceOnly, arbWhitespaceOnly, (wsUser, wsPass) => {
        const result = loginSchema.safeParse({
          username: wsUser,
          password: wsPass,
        });
        expect(result.success).toBe(false);
      }),
      { numRuns: 100 },
    );
  });

  it("does not trim a valid password that only has surrounding spaces when core is non-empty", () => {
    // Password with leading/trailing space but non-empty core must still be accepted
    // (schema rejects only pure whitespace; it must not strip password content).
    const result = loginSchema.safeParse({
      username: "validuser",
      password: " pass ",
    });
    expect(result.success).toBe(true);
  });
});

// ─── Max length boundaries ────────────────────────────────────────────────────

describe("Login schema max-length boundaries", () => {
  it("accepts username at 128 and rejects 129", () => {
    const ok = loginSchema.safeParse({
      username: "a".repeat(128),
      password: "validpass",
    });
    expect(ok.success).toBe(true);

    const over = loginSchema.safeParse({
      username: "a".repeat(129),
      password: "validpass",
    });
    expect(over.success).toBe(false);
  });

  it("accepts password at 72 and rejects 73", () => {
    const ok = loginSchema.safeParse({
      username: "validuser",
      password: "p".repeat(72),
    });
    expect(ok.success).toBe(true);

    const over = loginSchema.safeParse({
      username: "validuser",
      password: "p".repeat(73),
    });
    expect(over.success).toBe(false);
  });

  it("accepts arbitrary valid credential pairs within limits", () => {
    fc.assert(
      fc.property(arbNonEmptyCredential, arbNonEmptyCredential, (username, password) => {
        fc.pre(username.length <= 128 && password.length <= 72);
        const result = loginSchema.safeParse({ username, password });
        expect(result.success).toBe(true);
      }),
      { numRuns: 100 },
    );
  });
});

// ─── Property 19: Retry-After Countdown Formatting ────────────────────────────

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

  it("M*60 + S always equals the original N", () => {
    fc.assert(
      fc.property(arbRetryAfterSeconds, (seconds) => {
        const formatted = formatRetryAfter(seconds);
        const match = formatted.match(/^(\d+) menit (\d+) detik$/);
        expect(match).not.toBeNull();

        // biome-ignore lint/style/noNonNullAssertion: null-checked via the assertion above.
        const minutes = Number.parseInt(match![1], 10);
        // biome-ignore lint/style/noNonNullAssertion: null-checked via the assertion above.
        const secs = Number.parseInt(match![2], 10);
        expect(minutes * 60 + secs).toBe(seconds);
      }),
      { numRuns: 200 },
    );
  });

  it("formats zero as 0 menit 0 detik", () => {
    expect(formatRetryAfter(0)).toBe("0 menit 0 detik");
  });
});

// ─── Countdown clamp ──────────────────────────────────────────────────────────

describe("remainingRateLimitSeconds clamp", () => {
  it("never returns negative values", () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 0, max: 1_000_000 }),
        fc.integer({ min: 0, max: 2_000_000 }),
        (deadlineOffset, nowOffset) => {
          const base = 1_700_000_000_000;
          const deadline = base + deadlineOffset;
          const now = base + nowOffset;
          const remaining = remainingRateLimitSeconds(deadline, now);
          expect(remaining).toBeGreaterThanOrEqual(0);
        },
      ),
      { numRuns: 200 },
    );
  });

  it("returns ceil remaining seconds until deadline", () => {
    const deadline = 1_000_000;
    expect(remainingRateLimitSeconds(deadline, 999_000)).toBe(1);
    expect(remainingRateLimitSeconds(deadline, 1_000_000)).toBe(0);
    expect(remainingRateLimitSeconds(deadline, 1_001_000)).toBe(0);
  });
});
