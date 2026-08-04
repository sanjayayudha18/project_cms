import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import * as fc from "fast-check";
import { describe, expect, it, vi } from "vitest";
import { LoginPage } from "./login";

/**
 * Property 5: Login Error Message Safety
 * Validates: Requirements 2.4
 *
 * For any failed login attempt, the displayed error message SHALL NOT contain
 * field-specific identifiers (such as "email", "password", "kata sandi", or "surel")
 * that would reveal which credential was incorrect. The message must be generic.
 */

// ─── Mock auth store ──────────────────────────────────────────────────────────

const mockLogin = vi.fn();

vi.mock("@/lib/auth", () => ({
  useAuthStore: (selector: (s: Record<string, unknown>) => unknown) =>
    selector({ login: mockLogin }),
}));

// LoginPage uses useRouter().navigate — mock it so the component renders.
vi.mock("@tanstack/react-router", () => ({
  useRouter: () => ({ navigate: vi.fn() }),
}));

// ─── Forbidden words (case-insensitive) ───────────────────────────────────────

const FORBIDDEN_WORDS = ["email", "password", "kata sandi", "surel"];

// ─── Tests ────────────────────────────────────────────────────────────────────

describe("LoginPage — Property 5: Login Error Message Safety", () => {
  it("error message never contains field-specific identifiers for any backend error string", async () => {
    // Arbitrary strings that may contain forbidden words, special chars, etc.
    const arbErrorMessage = fc.oneof(
      fc.string({ minLength: 1 }),
      // Strings that explicitly include forbidden words to stress the property
      fc.constantFrom(
        "email not found",
        "invalid password",
        "wrong email or password",
        "kata sandi salah",
        "surel tidak ditemukan",
        "Email address is invalid",
        "Password must be at least 8 characters",
        "email: required field",
        "password is incorrect",
      ),
    );

    await fc.assert(
      fc.asyncProperty(arbErrorMessage, async (errorString) => {
        mockLogin.mockResolvedValue({ success: false, error: errorString });

        const { unmount } = render(<LoginPage />);

        // Use fireEvent for speed — avoids per-character delay of userEvent.type
        const emailInput = screen.getByLabelText("Email");
        const passwordInput = screen.getByLabelText("Kata Sandi");

        fireEvent.change(emailInput, {
          target: { value: "test@example.com" },
        });
        fireEvent.change(passwordInput, {
          target: { value: "somepassword123" },
        });

        fireEvent.submit(screen.getByRole("button", { name: "Masuk" }));

        await waitFor(() => {
          expect(screen.getByTestId("login-error")).toBeInTheDocument();
        });

        const errorText = screen.getByTestId("login-error").textContent ?? "";
        const lowerErrorText = errorText.toLowerCase();

        // The displayed error must NOT contain any forbidden field-specific identifiers
        for (const word of FORBIDDEN_WORDS) {
          expect(lowerErrorText).not.toContain(word);
        }

        unmount();
      }),
      { numRuns: 50 },
    );
  }, 30_000);
});
