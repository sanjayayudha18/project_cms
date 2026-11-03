/**
 * Task 9.2 (app-shell-revamp): integrasi AppShell (Sidebar + Topbar + CommandPalette lazy +
 * prefs + shortcuts) di atas router memory sungguhan.
 * Req 4.1, 4.2, 4.6, 4.7, 6.1, 6.3, 6.4, 8.2, 12.7, 12.8, 15.4, 16.1–16.3, 16.5.
 */

import { useAuthStore } from "@/lib/auth/store";
import { DEFAULT_SHELL_PREFS, SHELL_PREFS_KEY, useShellPrefs } from "@/lib/shell/prefs";
import {
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AppShell } from "./AppShell";

// Badge memanggil query jaringan; tidak relevan untuk test shell.
vi.mock("@/components/layout/useNavBadges", () => ({ useNavBadges: () => ({}) }));

function setViewport(narrow: boolean) {
  window.matchMedia = vi.fn().mockImplementation((query: string) => ({
    matches: narrow,
    media: query,
    onchange: null,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }));
}

function Page({ title }: { title: string }) {
  return (
    <div>
      <h1>{title}</h1>
      <input aria-label="isian" />
      <textarea aria-label="catatan" />
      <select aria-label="pilihan">
        <option>a</option>
      </select>
      <div
        role="textbox"
        aria-label="editor"
        tabIndex={0}
        contentEditable
        suppressContentEditableWarning
      />
    </div>
  );
}

async function renderShell(path = "/") {
  const root = createRootRoute({
    component: () => (
      <AppShell>
        <Outlet />
      </AppShell>
    ),
  });
  const tree = root.addChildren([
    createRoute({
      getParentRoute: () => root,
      path: "/",
      component: () => <Page title="Beranda" />,
    }),
    createRoute({
      getParentRoute: () => root,
      path: "/settings",
      component: () => <Page title="Pengaturan halaman" />,
    }),
  ]);
  const router = createRouter({
    routeTree: tree,
    history: createMemoryHistory({ initialEntries: [path] }),
  });
  await router.load();
  const view = render(<RouterProvider router={router} />);
  await screen.findByRole("main");
  return { router, ...view };
}

const sidebar = () => screen.getByRole("complementary", { name: "Navigasi utama" });
const searchTrigger = () => screen.getByRole("button", { name: /Cari menu atau fitur/ });
const pressKey = (init: KeyboardEventInit, target: Element = document.body) =>
  fireEvent.keyDown(target, init);

beforeEach(() => {
  localStorage.clear();
  useShellPrefs.setState({ ...DEFAULT_SHELL_PREFS });
  setViewport(false);
  // jsdom tidak punya scrollIntoView; CommandPalette memanggilnya di effect.
  Element.prototype.scrollIntoView = vi.fn();
  useAuthStore.setState({
    user: {
      id: 1,
      username: "test.user",
      fullName: "Test User",
      email: "t@t.com",
      role: "ADMIN",
      isKaryawan: true,
      vendorId: null,
    },
    isAuthenticated: true,
    isAuthLoading: false,
    error: null,
    rateLimitRetryAfter: null,
  });
});

afterEach(() => {
  cleanup();
  useAuthStore.setState({ user: null });
});

describe("AppShell — struktur & skip link", () => {
  it("skip link adalah elemen pertama di DOM dan menuju #main-content", async () => {
    const { container } = await renderShell();
    const grid = container.firstElementChild as HTMLElement;
    const skip = grid.firstElementChild as HTMLAnchorElement;
    expect(skip.tagName).toBe("A");
    expect(skip).toHaveAttribute("href", "#main-content");
    expect(screen.getByRole("main")).toHaveAttribute("id", "main-content");
  });

  it("landmark: sidebar, banner, main; children dirender di dalam main", async () => {
    await renderShell();
    expect(sidebar()).toBeInTheDocument();
    expect(screen.getByRole("banner")).toBeInTheDocument();
    expect(screen.getByRole("main")).toContainElement(screen.getByText("Beranda"));
  });

  it("<main> scroll terpisah dari shell", async () => {
    const { container } = await renderShell();
    expect(screen.getByRole("main")).toHaveClass("overflow-y-auto", "relative");
    expect(container.firstElementChild).toHaveClass("h-screen", "overflow-hidden");
  });
});

describe("AppShell — fokus saat route berubah (Req 16.1)", () => {
  it("tidak memindahkan fokus ke <main> pada render pertama", async () => {
    await renderShell();
    expect(screen.getByRole("main")).not.toHaveFocus();
  });

  it("memindahkan fokus ke <main> setelah route berubah", async () => {
    const { router } = await renderShell();
    await act(() => router.navigate({ to: "/settings" }));
    await waitFor(() => expect(screen.getByRole("main")).toHaveFocus());
    expect(screen.getByText("Pengaturan halaman")).toBeInTheDocument();
  });
});

describe("AppShell — shortcut `[` (Req 4.1, 4.2)", () => {
  it("toggle rail: 264px ↔ 76px dan prefs.rail ikut berubah", async () => {
    await renderShell();
    expect(sidebar()).toHaveStyle({ "--sb-w": "264px" });
    pressKey({ key: "[" });
    await waitFor(() => expect(sidebar()).toHaveStyle({ "--sb-w": "76px" }));
    expect(useShellPrefs.getState().rail).toBe(true);
    pressKey({ key: "[" });
    await waitFor(() => expect(sidebar()).toHaveStyle({ "--sb-w": "264px" }));
  });

  it.each([
    ["input", "isian"],
    ["textarea", "catatan"],
    ["select", "pilihan"],
    ["contenteditable", "editor"],
  ])("diabaikan saat fokus di %s", async (_name, label) => {
    await renderShell();
    const el = screen.getByLabelText(label);
    el.focus();
    pressKey({ key: "[" }, el);
    expect(useShellPrefs.getState().rail).toBe(false);
    expect(sidebar()).toHaveStyle({ "--sb-w": "264px" });
  });
});

describe("AppShell — command palette (Req 12.7, 12.8)", () => {
  it.each([
    ["Ctrl+K", { key: "k", ctrlKey: true }],
    ["⌘K", { key: "k", metaKey: true }],
  ])("%s membuka palette (lazy)", async (_name, init) => {
    await renderShell();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    pressKey(init);
    expect(await screen.findByRole("dialog", { name: "Cari menu" })).toBeInTheDocument();
  });

  it("klik search trigger membuka palette; Escape menutup dan fokus kembali ke trigger", async () => {
    await renderShell();
    await userEvent.setup().click(searchTrigger());
    const input = await screen.findByRole("combobox", { name: "Cari menu" });
    await waitFor(() => expect(input).toHaveFocus());
    await userEvent.setup().keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(searchTrigger()).toHaveFocus();
  });

  it("Enter pada item menavigasi ke href item; palette tutup, fokus akhir di <main>", async () => {
    const { router } = await renderShell();
    const user = userEvent.setup();
    pressKey({ key: "k", ctrlKey: true });
    await screen.findByRole("dialog");
    await user.keyboard("Pengaturan");
    await user.keyboard("{Enter}");
    await waitFor(() => expect(router.state.location.pathname).toBe("/settings"));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    // closePalette → trigger, lalu perubahan route → <main> (efek route berjalan terakhir).
    await waitFor(() => expect(screen.getByRole("main")).toHaveFocus());
  });

  it("`[` tidak toggle rail saat palette terbuka", async () => {
    await renderShell();
    pressKey({ key: "k", ctrlKey: true });
    const input = await screen.findByRole("combobox", { name: "Cari menu" });
    pressKey({ key: "[" }, input);
    expect(useShellPrefs.getState().rail).toBe(false);
  });
});

describe("AppShell — rail paksa & persistensi (Req 4.6, 4.7, 6.3)", () => {
  it("viewport < 1024px memaksa rail tanpa mengubah prefs.rail", async () => {
    setViewport(true);
    await renderShell();
    expect(sidebar()).toHaveStyle({ "--sb-w": "76px" });
    expect(useShellPrefs.getState().rail).toBe(false);
    expect(JSON.parse(localStorage.getItem(SHELL_PREFS_KEY) ?? "{}").state.rail).toBe(false);
  });

  it("rail dan tema tersimpan ke localStorage lalu dipulihkan setelah remount", async () => {
    const first = await renderShell();
    pressKey({ key: "[" });
    await userEvent.setup().click(screen.getByRole("button", { name: /Ganti ke sidebar terang/ }));
    await waitFor(() => {
      const saved = JSON.parse(localStorage.getItem(SHELL_PREFS_KEY) ?? "{}");
      expect(saved.state).toMatchObject({ rail: true, sidebarTheme: "light" });
    });
    first.unmount();

    // Simulasi reload: store kembali ke default di memori, storage tetap.
    const saved = localStorage.getItem(SHELL_PREFS_KEY) as string;
    useShellPrefs.setState({ ...DEFAULT_SHELL_PREFS });
    localStorage.setItem(SHELL_PREFS_KEY, saved);
    await useShellPrefs.persist.rehydrate();

    await renderShell();
    expect(sidebar()).toHaveStyle({ "--sb-w": "76px" });
    expect(sidebar()).toHaveAttribute("data-sidebar-theme", "light");
  });
});

describe("AppShell — tema sidebar terisolasi (Req 3, 6.4)", () => {
  it("data-sidebar-theme hanya ada di sidebar; Topbar dan konten tidak terpengaruh", async () => {
    useShellPrefs.setState({ sidebarTheme: "light" });
    await renderShell();
    const themed = document.querySelectorAll("[data-sidebar-theme]");
    expect(themed).toHaveLength(1);
    expect(themed[0]).toBe(sidebar());
    expect(screen.getByRole("banner").closest("[data-sidebar-theme]")).toBeNull();
    expect(screen.getByRole("main").closest("[data-sidebar-theme]")).toBeNull();
  });
});
