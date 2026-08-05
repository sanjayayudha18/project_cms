import * as fc from "fast-check";
import { describe, expect, it } from "vitest";
import { containsSensitiveDetails, sanitizeServerError } from "./ServerError";

/**
 * Property 19: Server Error Message Sanitization
 * Validates: Requirements 9.3
 *
 * For any HTTP 500 error response (regardless of the response body content —
 * which may contain stack traces, SQL errors, or internal messages), the error
 * handler SHALL produce only a fixed generic user-facing message that does not
 * contain any substring from the original error response body.
 */

const GENERIC_SERVER_ERROR_MESSAGE = "Terjadi kesalahan pada server. Silakan coba lagi nanti.";

describe("Property 19: Server Error Message Sanitization", () => {
  // --- Generators ---

  const arbitraryErrorMessage = fc.oneof(
    fc.string({ minLength: 1 }),
    fc.constantFrom(
      "Error: ECONNREFUSED 127.0.0.1:5432",
      "panic: runtime error: invalid memory address or nil pointer dereference",
      "at Object.<anonymous> (/app/src/server.ts:42)",
      'pq: relation "users" does not exist',
      'FATAL: password authentication failed for user "postgres"',
      "goroutine 1 [running]:\nmain.main()\n\t/app/main.go:15 +0x1a2",
      "TypeError: Cannot read properties of undefined (reading 'map')",
      "segfault at address 0x0000000000000000",
      "errno ENOENT: no such file or directory, open '/etc/secrets/db.key'",
      "internal server error: database connection pool exhausted",
      "SQL syntax error near 'SELECT * FROM users WHERE id = 1; DROP TABLE users;--'",
      "stack trace:\n  at PostgresDriver.connect (node_modules/pg/lib/client.js:132:17)",
    ),
  );

  // These patterns each contain a substring matching at least one SENSITIVE_PATTERNS regex
  const dangerousPatterns = fc.constantFrom(
    "stack trace: at main.go:42", // matches /stack\s*trace/i
    "at processTicksAndRejections (internal/process/task_queues.js:95:5)", // matches /at\s+\w+\s*\(/i
    "/app/src/handlers/auth.go:128", // matches /\.go:\d+/i
    "/usr/src/app/dist/server.ts:55", // matches /\.ts:\d+/i
    "index.js:1042 Uncaught ReferenceError", // matches /\.js:\d+/i
    "SQL injection attempt detected in query", // matches /sql/i (contains "SQL")
    "postgres://admin:secret@db.internal:5432/cms", // matches /postgres/i
    "internal server error", // matches /internal\s+server/i
    "panic: send on closed channel", // matches /panic/i
    "segfault in worker pid 12345", // matches /segfault/i
    "null pointer dereference at 0x7fff", // matches /null\s*pointer/i
    "undefined reference to `main'", // matches /undefined\s+reference/i
    "ECONNREFUSED 10.0.0.5:6379", // matches /ECONNREFUSED/i
    "errno=ENOMEM", // matches /errno/i
  );

  // --- Property Tests ---

  it("sanitizeServerError always returns the fixed generic message for any input", () => {
    fc.assert(
      fc.property(arbitraryErrorMessage, (errorMsg) => {
        const result = sanitizeServerError(errorMsg);
        expect(result).toBe(GENERIC_SERVER_ERROR_MESSAGE);
      }),
    );
  });

  it("sanitizeServerError returns generic message for null and undefined inputs", () => {
    expect(sanitizeServerError(null)).toBe(GENERIC_SERVER_ERROR_MESSAGE);
    expect(sanitizeServerError(undefined)).toBe(GENERIC_SERVER_ERROR_MESSAGE);
    expect(sanitizeServerError("")).toBe(GENERIC_SERVER_ERROR_MESSAGE);
  });

  it("output never contains substrings from the original error (for inputs >= 5 chars that differ from generic)", () => {
    const longDistinctInput = fc
      .string({ minLength: 5 })
      .filter((s) => !GENERIC_SERVER_ERROR_MESSAGE.includes(s));

    fc.assert(
      fc.property(longDistinctInput, (errorMsg) => {
        const result = sanitizeServerError(errorMsg);
        expect(result).not.toContain(errorMsg);
      }),
    );
  });

  it("dangerous patterns never leak through sanitizeServerError", () => {
    fc.assert(
      fc.property(dangerousPatterns, (pattern) => {
        const result = sanitizeServerError(pattern);
        expect(result).toBe(GENERIC_SERVER_ERROR_MESSAGE);
        expect(result).not.toContain(pattern);
      }),
    );
  });

  it("containsSensitiveDetails detects all known dangerous patterns", () => {
    fc.assert(
      fc.property(dangerousPatterns, (pattern) => {
        expect(containsSensitiveDetails(pattern)).toBe(true);
      }),
    );
  });

  it("containsSensitiveDetails returns false for benign user-facing messages", () => {
    const benignMessages = fc.constantFrom(
      "Terjadi kesalahan pada server. Silakan coba lagi nanti.",
      "Koneksi terputus",
      "Silakan periksa koneksi internet Anda",
      "Data tidak ditemukan",
      "Sesi Anda telah berakhir",
    );

    fc.assert(
      fc.property(benignMessages, (msg) => {
        expect(containsSensitiveDetails(msg)).toBe(false);
      }),
    );
  });
});
