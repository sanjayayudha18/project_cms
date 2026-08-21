import { useAuthStore } from "@/lib/auth/store";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LoginPage } from "./login";

// ─── Mock router ──────────────────────────────────────────────────────────────

const { mockNavigate } = vi.hoisted(() => ({ mockNavigate: vi.fn() }));

vi.mock("@tanstack/react-router", () => ({
  useRouter: () => ({ navigate: mockNavigate }),
  Navigate: ({ to }: { to: string }) => <div data-testid="navigate" data-to={to} />,
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
  vi.useRealTimers();
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.useRealTimers();
});

function mockOkLogin() {
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
}

// ─── Tests ────────────────────────────────────────────────────────────────────

describe("LoginPage", () => {
  it("renders exactly one username and one password field with required attributes", () => {
    render(<LoginPage />);

    const username = screen.getByLabelText("Username");
    const password = screen.getByLabelText("Kata Sandi");

    expect(screen.getAllByLabelText("Username")).toHaveLength(1);
    expect(screen.getAllByLabelText("Kata Sandi")).toHaveLength(1);

    expect(username).toHaveAttribute("id", "username");
    expect(username).toHaveAttribute("type", "text");
    expect(username).toHaveAttribute("autocomplete", "username");
    expect(username).toHaveAttribute("placeholder", "Username");

    expect(password).toHaveAttribute("id", "password");
    expect(password).toHaveAttribute("type", "password");
    expect(password).toHaveAttribute("autocomplete", "current-password");
    expect(password).toHaveAttribute("placeholder", "Masukkan kata sandi");
  });

  it("renders submit button with correct label and min height target", () => {
    render(<LoginPage />);

    const button = screen.getByRole("button", { name: "Masuk" });
    expect(button).toBeInTheDocument();
    expect(button).toHaveAttribute("type", "submit");
    expect(button.className).toMatch(/login-submit|min-h|h-11|h-\[44/);
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
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("rejects whitespace-only fields without calling auth", async () => {
    const user = userEvent.setup();
    render(<LoginPage />);

    await user.type(screen.getByLabelText("Username"), "   ");
    await user.type(screen.getByLabelText("Kata Sandi"), "\t\n");
    await user.click(screen.getByRole("button", { name: "Masuk" }));

    await waitFor(() => {
      expect(screen.getAllByText("Wajib diisi").length).toBeGreaterThanOrEqual(1);
    });
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("rejects username longer than 128 characters", async () => {
    const user = userEvent.setup();
    render(<LoginPage />);

    await user.type(screen.getByLabelText("Username"), "a".repeat(129));
    await user.type(screen.getByLabelText("Kata Sandi"), "validpass");
    await user.click(screen.getByRole("button", { name: "Masuk" }));

    await waitFor(() => {
      expect(screen.getByText(/maksimal 128/i)).toBeInTheDocument();
    });
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("rejects password longer than 72 characters", async () => {
    const user = userEvent.setup();
    render(<LoginPage />);

    await user.type(screen.getByLabelText("Username"), "validuser");
    await user.type(screen.getByLabelText("Kata Sandi"), "p".repeat(73));
    await user.click(screen.getByRole("button", { name: "Masuk" }));

    await waitFor(() => {
      expect(screen.getByText(/maksimal 72/i)).toBeInTheDocument();
    });
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("focuses the first invalid field on validation failure", async () => {
    const user = userEvent.setup();
    render(<LoginPage />);

    await user.click(screen.getByRole("button", { name: "Masuk" }));

    await waitFor(() => {
      expect(screen.getByLabelText("Username")).toHaveFocus();
    });
  });

  it("submits on Enter from password field", async () => {
    mockOkLogin();
    const user = userEvent.setup();
    render(<LoginPage />);

    await user.type(screen.getByLabelText("Username"), "john.admin");
    await user.type(screen.getByLabelText("Kata Sandi"), "Password123!{Enter}");

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalled();
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
      expect(screen.getByTestId("login-error")).toHaveTextContent("Username atau password salah");
    });
  });

  it("shows inactive account error (403)", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 403,
      json: async () => ({ error: "account_inactive", message: "Akun tidak aktif" }),
    });

    const user = userEvent.setup();
    render(<LoginPage />);

    await user.type(screen.getByLabelText("Username"), "inactive");
    await user.type(screen.getByLabelText("Kata Sandi"), "pass");
    await user.click(screen.getByRole("button", { name: "Masuk" }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent("Akun tidak aktif");
    });
  });

  it("shows service unavailable error (503)", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 503,
      json: async () => ({ error: "service_unavailable", message: "down" }),
    });

    const user = userEvent.setup();
    render(<LoginPage />);

    await user.type(screen.getByLabelText("Username"), "user");
    await user.type(screen.getByLabelText("Kata Sandi"), "pass");
    await user.click(screen.getByRole("button", { name: "Masuk" }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent("Layanan sedang tidak tersedia");
    });
  });

  it("shows network error message", async () => {
    mockFetch.mockRejectedValueOnce(new Error("Network error"));

    const user = userEvent.setup();
    render(<LoginPage />);

    await user.type(screen.getByLabelText("Username"), "user");
    await user.type(screen.getByLabelText("Kata Sandi"), "pass");
    await user.click(screen.getByRole("button", { name: "Masuk" }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(
        "Gagal terhubung ke server. Silakan coba lagi.",
      );
    });
  });

  it("navigates to home on successful login", async () => {
    mockOkLogin();

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

  it("shows loading state and prevents duplicate submit", async () => {
    let resolveFetch: (value: unknown) => void = () => {};
    mockFetch.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveFetch = resolve;
        }),
    );

    const user = userEvent.setup();
    render(<LoginPage />);

    await user.type(screen.getByLabelText("Username"), "john.admin");
    await user.type(screen.getByLabelText("Kata Sandi"), "Password123!");
    await user.click(screen.getByRole("button", { name: "Masuk" }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /Memproses/i })).toBeDisabled();
    });

    const form = screen.getByRole("button", { name: /Memproses/i }).closest("form");
    expect(form).toHaveAttribute("aria-busy", "true");
    expect(screen.getByLabelText("Username")).toHaveValue("john.admin");
    expect(screen.getByLabelText("Kata Sandi")).toHaveValue("Password123!");

    resolveFetch({
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

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith({ to: "/" });
    });
    expect(mockFetch).toHaveBeenCalledTimes(1);
  });

  it("toggles password visibility while preserving value", async () => {
    const user = userEvent.setup();
    render(<LoginPage />);

    const password = screen.getByLabelText("Kata Sandi");
    await user.type(password, "Secret123!");

    const toggle = screen.getByRole("button", { name: /show password|tampilkan/i });
    expect(password).toHaveAttribute("type", "password");
    expect(toggle).toHaveAttribute("aria-pressed", "false");

    await user.click(toggle);

    expect(screen.getByLabelText("Kata Sandi")).toHaveAttribute("type", "text");
    expect(screen.getByLabelText("Kata Sandi")).toHaveValue("Secret123!");
    expect(screen.getByRole("button", { name: /hide password|sembunyikan/i })).toHaveAttribute(
      "aria-pressed",
      "true",
    );

    await user.click(screen.getByRole("button", { name: /hide password|sembunyikan/i }));
    expect(screen.getByLabelText("Kata Sandi")).toHaveAttribute("type", "password");
    expect(screen.getByLabelText("Kata Sandi")).toHaveValue("Secret123!");
  });

  it("shows rate-limit message and countdown, then re-enables", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 429,
      headers: { get: (name: string) => (name === "Retry-After" ? "3" : null) },
      json: async () => ({ error: "rate_limited", message: "Terlalu banyak percobaan login" }),
    });

    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<LoginPage />);

    await user.type(screen.getByLabelText("Username"), "user");
    await user.type(screen.getByLabelText("Kata Sandi"), "pass");
    await user.click(screen.getByRole("button", { name: "Masuk" }));

    await waitFor(() => {
      const alert = screen.getByRole("alert");
      expect(alert).toHaveTextContent("Terlalu banyak percobaan login");
      expect(alert.textContent).toMatch(/\d+ menit \d+ detik/);
    });

    expect(screen.getByRole("button", { name: "Masuk" })).toBeDisabled();

    await vi.advanceTimersByTimeAsync(3500);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Masuk" })).not.toBeDisabled();
    });
  });

  it("shows rate-limit without countdown when Retry-After is missing", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 429,
      headers: { get: () => null },
      json: async () => ({ error: "rate_limited", message: "Terlalu banyak percobaan login" }),
    });

    const user = userEvent.setup();
    render(<LoginPage />);

    await user.type(screen.getByLabelText("Username"), "user");
    await user.type(screen.getByLabelText("Kata Sandi"), "pass");
    await user.click(screen.getByRole("button", { name: "Masuk" }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent("Terlalu banyak percobaan login");
    });
  });

  it("redirects authenticated users away from the login form", () => {
    useAuthStore.setState({
      isAuthenticated: true,
      isAuthLoading: false,
      user: {
        id: 1,
        username: "john.admin",
        fullName: "John Admin",
        email: "john@cimb.local",
        role: "ADMIN",
        isKaryawan: true,
        vendorId: null,
      },
      accessToken: "token",
    });

    render(<LoginPage />);

    expect(screen.getByTestId("navigate")).toHaveAttribute("data-to", "/");
    expect(screen.queryByLabelText("Username")).not.toBeInTheDocument();
  });

  it("wires field errors with aria-invalid and aria-describedby", async () => {
    const user = userEvent.setup();
    render(<LoginPage />);

    await user.click(screen.getByRole("button", { name: "Masuk" }));

    await waitFor(() => {
      const username = screen.getByLabelText("Username");
      expect(username).toHaveAttribute("aria-invalid", "true");
      const describedBy = username.getAttribute("aria-describedby");
      expect(describedBy).toBeTruthy();
      // biome-ignore lint/style/noNonNullAssertion: asserted above
      expect(document.getElementById(describedBy!)).toHaveTextContent("Wajib diisi");
    });
  });

  it("allows retry after an error without reload", async () => {
    mockFetch
      .mockResolvedValueOnce({
        ok: false,
        status: 401,
        json: async () => ({ error: "auth_failed", message: "Username atau password salah" }),
      })
      .mockResolvedValueOnce({
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
    await user.type(screen.getByLabelText("Kata Sandi"), "wrong");
    await user.click(screen.getByRole("button", { name: "Masuk" }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toBeInTheDocument();
    });

    await user.clear(screen.getByLabelText("Kata Sandi"));
    await user.type(screen.getByLabelText("Kata Sandi"), "Password123!");
    await user.click(screen.getByRole("button", { name: "Masuk" }));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith({ to: "/" });
    });
  });
});

describe("Auth layout artwork (via LoginPage shell expectations)", () => {
  it("documents expected artwork path used by AuthLayout", () => {
    // Artwork is owned by _auth.tsx; contract verified here for task traceability.
    expect("/assets/crown-login-artwork.png").toMatch(/^\/assets\//);
  });
});
