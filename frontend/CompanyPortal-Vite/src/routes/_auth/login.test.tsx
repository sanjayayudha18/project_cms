import { useAuthStore } from "@/lib/auth/store";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LoginPage } from "./login";

// ─── Mock router ──────────────────────────────────────────────────────────────

const { mockNavigate } = vi.hoisted(() => ({ mockNavigate: vi.fn() }));

vi.mock("@tanstack/react-router", () => ({
  useRouter: () => ({ navigate: mockNavigate }),
}));

// ─── Mock fetch (store.login calls fetch internally) ──────────────────────────

const mockFetch = vi.fn();

beforeEach(() => {
  vi.stubGlobal("fetch", mockFetch);
  useAuthStore.setState({
    user: null,
    accessToken: null,
    isAuthenticated: false,
    isAuthLoading: false,
    error: null,
    rateLimitRetryAfter: null,
  });
  mockNavigate.mockReset();
  mockFetch.mockReset();
});

afterEach(() => {
  vi.restoreAllMocks();
});

// ─── Tests ────────────────────────────────────────────────────────────────────

describe("LoginPage", () => {
  it("renders username and password fields", () => {
    render(<LoginPage />);

    expect(screen.getByLabelText("Username")).toBeInTheDocument();
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

  it("shows error on failed login (401)", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 401,
      json: async () => ({ error: "auth_failed", message: "Username atau password salah" }),
    });

    const user = userEvent.setup();
    render(<LoginPage />);

    await user.type(screen.getByLabelText("Username"), "wronguser");
    await user.type(screen.getByLabelText("Kata Sandi"), "wrongpass");
    await user.click(screen.getByRole("button", { name: "Masuk" }));

    await waitFor(() => {
      expect(screen.getByTestId("login-error")).toBeInTheDocument();
    });
  });

  it("navigates to home on successful login", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({
        access_token: "jwt-token",
        user: {
          id: 1,
          username: "john.admin",
          full_name: "John Admin",
          email: "john@cimb.local",
          role: "ADMIN",
          is_karyawan: true,
          vendor_id: null,
        },
      }),
    });

    const user = userEvent.setup();
    render(<LoginPage />);

    await user.type(screen.getByLabelText("Username"), "john.admin");
    await user.type(screen.getByLabelText("Kata Sandi"), "Password123!");
    await user.click(screen.getByRole("button", { name: "Masuk" }));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith({ to: "/" });
    });
  });

  it("login error has role=alert for accessibility", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 401,
      json: async () => ({ error: "auth_failed", message: "Username atau password salah" }),
    });

    const user = userEvent.setup();
    render(<LoginPage />);

    await user.type(screen.getByLabelText("Username"), "user");
    await user.type(screen.getByLabelText("Kata Sandi"), "pass");
    await user.click(screen.getByRole("button", { name: "Masuk" }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toBeInTheDocument();
    });
  });
});
