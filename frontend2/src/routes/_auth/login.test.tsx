import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { LoginPage } from "./login";

// ─── Mock auth store ──────────────────────────────────────────────────────────

const mockLogin = vi.fn();

vi.mock("@/lib/auth", () => ({
  useAuthStore: (selector: (s: Record<string, unknown>) => unknown) =>
    selector({ login: mockLogin }),
}));

// ─── Mock router ──────────────────────────────────────────────────────────────
// LoginPage now navigates to "/" on success via useRouter().navigate.
const { mockNavigate } = vi.hoisted(() => ({ mockNavigate: vi.fn() }));

vi.mock("@tanstack/react-router", () => ({
  useRouter: () => ({ navigate: mockNavigate }),
}));

// ─── Tests ────────────────────────────────────────────────────────────────────

describe("LoginPage", () => {
  beforeEach(() => {
    mockLogin.mockReset();
    mockNavigate.mockReset();
  });

  it("renders email and password fields", () => {
    render(<LoginPage />);

    expect(screen.getByLabelText("Email")).toBeInTheDocument();
    expect(screen.getByLabelText("Kata Sandi")).toBeInTheDocument();
  });

  it("renders submit button with correct label", () => {
    render(<LoginPage />);

    const button = screen.getByRole("button", { name: "Masuk" });
    expect(button).toBeInTheDocument();
    expect(button).toHaveAttribute("type", "submit");
  });

  it("renders form title 'Selamat datang'", () => {
    render(<LoginPage />);

    expect(screen.getByRole("heading", { name: "Selamat datang" })).toBeInTheDocument();
  });

  it("shows validation error when fields are empty on submit", async () => {
    const user = userEvent.setup();
    render(<LoginPage />);

    await user.click(screen.getByRole("button", { name: "Masuk" }));

    await waitFor(() => {
      expect(screen.getAllByText("Wajib diisi")).toHaveLength(2);
    });
  });

  it("shows generic error on failed login", async () => {
    mockLogin.mockResolvedValue({ success: false, error: "invalid" });
    const user = userEvent.setup();
    render(<LoginPage />);

    await user.type(screen.getByLabelText("Email"), "user@test.com");
    await user.type(screen.getByLabelText("Kata Sandi"), "wrongpass");
    await user.click(screen.getByRole("button", { name: "Masuk" }));

    await waitFor(() => {
      expect(screen.getByTestId("login-error")).toBeInTheDocument();
    });

    expect(screen.getByTestId("login-error")).toHaveTextContent("Kredensial tidak valid");
  });

  it("error message is generic — does NOT contain field-specific hints", async () => {
    mockLogin.mockResolvedValue({
      success: false,
      error: "email not found",
    });
    const user = userEvent.setup();
    render(<LoginPage />);

    await user.type(screen.getByLabelText("Email"), "user@test.com");
    await user.type(screen.getByLabelText("Kata Sandi"), "wrongpass");
    await user.click(screen.getByRole("button", { name: "Masuk" }));

    await waitFor(() => {
      expect(screen.getByTestId("login-error")).toBeInTheDocument();
    });

    const errorText = screen.getByTestId("login-error").textContent ?? "";
    const forbiddenWords = ["email", "password", "kata sandi", "surel"];

    for (const word of forbiddenWords) {
      expect(errorText.toLowerCase()).not.toContain(word);
    }
  });

  it("navigates to home on successful login", async () => {
    mockLogin.mockResolvedValue({
      success: true,
      user: {
        id: "1",
        fullName: "Test User",
        email: "test@example.com",
        roles: ["Admin"],
        primaryRole: "Admin",
      },
    });
    const user = userEvent.setup();
    render(<LoginPage />);

    await user.type(screen.getByLabelText("Email"), "test@example.com");
    await user.type(screen.getByLabelText("Kata Sandi"), "correctpass");
    await user.click(screen.getByRole("button", { name: "Masuk" }));

    // There is no success banner anymore — on success the page redirects home.
    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith({ to: "/" });
    });
  });

  it("login error has role=alert for accessibility", async () => {
    mockLogin.mockResolvedValue({ success: false, error: "fail" });
    const user = userEvent.setup();
    render(<LoginPage />);

    await user.type(screen.getByLabelText("Email"), "a@b.com");
    await user.type(screen.getByLabelText("Kata Sandi"), "x");
    await user.click(screen.getByRole("button", { name: "Masuk" }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toBeInTheDocument();
    });
  });
});
