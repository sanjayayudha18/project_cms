import { renderWithRouter } from "@/test/routerTestUtils";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LoginPage } from "../LoginPage";

// fe-coverage C1: VendorPortal login screen — validation, error display, rate-limit
// countdown, show/hide password, caps lock, redirect after login.

const mockFetch = vi.fn();

const vendorLogin = {
  access_token: "tok",
  user: {
    id: 1,
    username: "vendor.user",
    full_name: "Budi Santoso",
    email: "budi@vendor.com",
    role: "VENDOR-USER",
    is_karyawan: false,
    vendor_id: 42,
  },
};

function respondLoginWith(response: () => Response) {
  mockFetch.mockImplementation(async (url: string) => {
    if (url.includes("/refresh")) {
      return new Response(JSON.stringify({ error: "unauthorized" }), { status: 401 });
    }
    if (url.includes("/login")) return response();
    return new Response(JSON.stringify({ items: [], total: 0 }), { status: 200 });
  });
}

async function renderLogin(path = "/login") {
  renderWithRouter(path, { login: () => <LoginPage /> });
  return screen.findByRole("button", { name: "Masuk" });
}

async function fillAndSubmit(email: string, password: string) {
  const user = userEvent.setup();
  await user.type(screen.getByLabelText("Email"), email);
  await user.type(screen.getByLabelText("Kata sandi"), password);
  await user.click(screen.getByRole("button", { name: "Masuk" }));
  return user;
}

beforeEach(() => {
  mockFetch.mockReset();
  vi.stubGlobal("fetch", mockFetch);
  respondLoginWith(() => new Response("", { status: 500 }));
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("LoginPage", () => {
  it("requires email and password, without calling the API", async () => {
    await renderLogin();
    const user = userEvent.setup();

    await user.click(screen.getByRole("button", { name: "Masuk" }));

    expect(await screen.findByText("Email wajib diisi")).toBeInTheDocument();
    expect(screen.getByText("Password wajib diisi")).toBeInTheDocument();
    expect(mockFetch.mock.calls.some(([u]) => String(u).includes("/login"))).toBe(false);
  });

  it("treats whitespace-only input as empty", async () => {
    await renderLogin();

    await fillAndSubmit("   ", "   ");

    expect(await screen.findByText("Email wajib diisi")).toBeInTheDocument();
    expect(screen.getByText("Password wajib diisi")).toBeInTheDocument();
  });

  it("shows the server error message from a failed login", async () => {
    respondLoginWith(
      () => new Response(JSON.stringify({ error: "invalid_credentials" }), { status: 401 }),
    );
    await renderLogin();

    await fillAndSubmit("budi@vendor.com", "salah");

    expect(await screen.findByRole("alert")).toHaveTextContent("Email atau password salah");
  });

  it("toggles password visibility", async () => {
    await renderLogin();
    const user = userEvent.setup();
    const password = screen.getByLabelText("Kata sandi");
    expect(password).toHaveAttribute("type", "password");

    await user.click(screen.getByRole("button", { name: "Tampilkan kata sandi" }));
    expect(password).toHaveAttribute("type", "text");

    await user.click(screen.getByRole("button", { name: "Sembunyikan kata sandi" }));
    expect(password).toHaveAttribute("type", "password");
  });

  it("warns when Caps Lock is on", async () => {
    await renderLogin();
    const user = userEvent.setup();

    await user.click(screen.getByLabelText("Kata sandi"));
    await user.keyboard("{CapsLock}");

    expect(screen.getByLabelText("Kata sandi")).toHaveAttribute(
      "aria-describedby",
      "pass-capslock",
    );
  });

  it("locks the form with a minutes countdown after 429", async () => {
    respondLoginWith(
      () =>
        new Response(JSON.stringify({ error: "rate_limited" }), {
          status: 429,
          headers: { "Retry-After": "75" },
        }),
    );
    await renderLogin();

    await fillAndSubmit("budi@vendor.com", "x");

    expect(await screen.findByText(/Coba lagi dalam 1 menit 15 detik/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Masuk" })).toBeDisabled();
    expect(screen.getByLabelText("Email")).toBeDisabled();
  });

  it("unlocks the form when the countdown ends", async () => {
    respondLoginWith(
      () =>
        new Response(JSON.stringify({ error: "rate_limited" }), {
          status: 429,
          headers: { "Retry-After": "1" },
        }),
    );
    await renderLogin();

    await fillAndSubmit("budi@vendor.com", "x");

    expect(await screen.findByText(/Coba lagi dalam 1 detik/)).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole("button", { name: "Masuk" })).toBeEnabled(), {
      timeout: 3000,
    });
  });

  it("goes to the preserved redirect target after a successful login", async () => {
    respondLoginWith(() => new Response(JSON.stringify(vendorLogin), { status: 200 }));
    await renderLogin("/login?redirect=%2Fdsr");

    await fillAndSubmit("budi@vendor.com", "password123");

    expect(await screen.findByTestId("page-dsr")).toBeInTheDocument();
    // It must stay there, not bounce on to the default /orders.
    await new Promise((r) => setTimeout(r, 300));
    expect(screen.queryByTestId("page-orders")).not.toBeInTheDocument();
  });

  it("defaults to /orders when there is no redirect target", async () => {
    respondLoginWith(() => new Response(JSON.stringify(vendorLogin), { status: 200 }));
    await renderLogin();

    await fillAndSubmit("budi@vendor.com", "password123");

    expect(await screen.findByTestId("page-orders")).toBeInTheDocument();
  });
});

describe("Guest guard for an already-signed-in vendor (AuthLayout)", () => {
  function signedIn() {
    mockFetch.mockImplementation(async (url: string) =>
      url.includes("/refresh")
        ? new Response(JSON.stringify(vendorLogin), { status: 200 })
        : new Response(JSON.stringify({ items: [], total: 0 }), { status: 200 }),
    );
  }

  it("opening /login with a safe redirect goes straight to it", async () => {
    signedIn();
    renderWithRouter("/login?redirect=%2Fdsr", { login: () => <LoginPage /> });

    expect(await screen.findByTestId("page-dsr")).toBeInTheDocument();
  });

  it("an unsafe redirect falls back to /orders", async () => {
    signedIn();
    renderWithRouter("/login?redirect=%2F%2Fevil.com", { login: () => <LoginPage /> });

    expect(await screen.findByTestId("page-orders")).toBeInTheDocument();
  });
});
