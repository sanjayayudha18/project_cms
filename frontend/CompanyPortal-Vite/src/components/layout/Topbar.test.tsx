/**
 * Task 7.7 (app-shell-revamp): Topbar + topbar/{Breadcrumb,SearchTrigger,OpsStatusStrip,
 * TopbarIconButton,UserMenu}. Replaces Header.test.tsx + Header.property.test.tsx.
 * Req 6.2, 7.1–7.4, 8.1, 8.3, 9.1, 9.3–9.5, 9.7, 10.1–10.5, 11.1–11.6, 16.5.
 */

import type { AuthUser, DbRole } from "@/lib/auth";
import { useAuthStore } from "@/lib/auth/store";
import { GROUP_LABELS, findActiveNavItem, getVisibleNav } from "@/lib/config/navigation";
import { act, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import * as fc from "fast-check";
import type { ComponentProps, ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { Topbar } from "./Topbar";
import { OpsStatusStrip } from "./topbar/OpsStatusStrip";
import { UserMenu } from "./topbar/UserMenu";

// Link jadi <a href> biasa: test ini tidak butuh RouterProvider.
vi.mock("@tanstack/react-router", () => ({
  Link: ({ to, children, ...rest }: { to: string; children: ReactNode }) => (
    <a href={to} {...rest}>
      {children}
    </a>
  ),
}));

const admin = getVisibleNav("ADMIN");
const settingsItem = admin.settings;
const dashboard = admin.all.find((i) => i.id === "dashboard");

const mockUser: AuthUser = {
  id: 1,
  username: "budi.santoso",
  fullName: "Budi Santoso",
  email: "budi@cimb.com",
  role: "ADMIN",
  isKaryawan: true,
  vendorId: null,
};

const logout = vi.fn().mockResolvedValue(undefined);

function renderTopbar(overrides: Partial<ComponentProps<typeof Topbar>> = {}) {
  const props: ComponentProps<typeof Topbar> = {
    active: dashboard,
    sidebarTheme: "dark",
    onToggleTheme: vi.fn(),
    onOpenPalette: vi.fn(),
    ...overrides,
  };
  return { props, ...render(<Topbar {...props} />) };
}

beforeEach(() => {
  logout.mockClear();
  useAuthStore.setState({ user: mockUser, logout });
});

afterEach(() => {
  useAuthStore.setState({ user: null });
});

describe("Topbar — breadcrumb (Req 7.2–7.4)", () => {
  it("regular item: group › label, label is aria-current", () => {
    renderTopbar();
    const nav = screen.getByRole("navigation", { name: "Breadcrumb" });
    expect(nav).toHaveTextContent(GROUP_LABELS[dashboard?.group ?? "general"]);
    expect(nav.querySelector('[aria-current="page"]')).toHaveTextContent(dashboard?.label ?? "");
  });

  it("settings: 'Sistem › Pengaturan'", () => {
    renderTopbar({ active: settingsItem });
    const nav = screen.getByRole("navigation", { name: "Breadcrumb" });
    expect(nav).toHaveTextContent(GROUP_LABELS.system);
    expect(nav.querySelector('[aria-current="page"]')).toHaveTextContent("Pengaturan");
  });

  it("no active item: breadcrumb not rendered", () => {
    renderTopbar({ active: findActiveNavItem(admin.all, "/tidak-ada/halaman") });
    expect(screen.queryByRole("navigation", { name: "Breadcrumb" })).not.toBeInTheDocument();
  });
});

describe("Topbar — search trigger (Req 8.1, 8.3)", () => {
  it("has dialog popup + keyshortcuts ARIA and opens the palette on click", async () => {
    const { props } = renderTopbar();
    const btn = screen.getByRole("button", { name: /Cari menu atau fitur/ });
    expect(btn).toHaveAttribute("aria-haspopup", "dialog");
    expect(btn).toHaveAttribute("aria-keyshortcuts", "Control+K Meta+K");
    await userEvent.setup().click(btn);
    expect(props.onOpenPalette).toHaveBeenCalledTimes(1);
  });

  it("forwards searchRef to the button", () => {
    const ref = { current: null as HTMLButtonElement | null };
    renderTopbar({ searchRef: ref });
    expect(ref.current).toBe(screen.getByRole("button", { name: /Cari menu atau fitur/ }));
  });
});

describe("OpsStatusStrip (Req 9)", () => {
  // Cut-off CIT 15:00 WIB = 08:00 UTC.
  const at = (iso: string) => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(iso));
  };
  afterEach(() => vi.useRealTimers());

  it("open: shows HH:MM:SS countdown, no warning text, no aria-live", () => {
    at("2026-10-20T03:00:00Z");
    const { container } = render(<OpsStatusStrip />);
    expect(container.querySelector("time")).toHaveTextContent("05:00:00");
    expect(screen.queryByText("kurang dari 1 jam")).not.toBeInTheDocument();
    expect(container.querySelector("[aria-live]")).toBeNull();
  });

  it("group has accessible label with the cut-off and is hidden below lg via class", () => {
    at("2026-10-20T03:00:00Z");
    render(<OpsStatusStrip />);
    const group = screen.getByRole("group", { name: /cut-off CIT pukul 15\.00 WIB/ });
    expect(group).toHaveClass("hidden", "lg:flex");
  });

  it("warning (< 1 h): sr-only text + accent tint", () => {
    at("2026-10-20T07:30:00Z");
    const { container } = render(<OpsStatusStrip />);
    expect(container.querySelector("time")).toHaveTextContent("00:30:00");
    expect(screen.getByText("kurang dari 1 jam")).toHaveClass("sr-only");
    expect(container.querySelector("[class*='shell-accent-tint']")).not.toBeNull();
    expect(container.querySelector("[aria-live]")).toBeNull();
  });

  it("closed (>= 15:00 WIB): 'ditutup', no countdown", () => {
    at("2026-10-20T08:30:00Z");
    const { container } = render(<OpsStatusStrip />);
    expect(screen.getByText("ditutup")).toBeInTheDocument();
    expect(container.querySelector("time")).toBeNull();
    expect(container.querySelector("[aria-live]")).toBeNull();
  });

  it("countdown ticks every second", () => {
    at("2026-10-20T03:00:00Z");
    const { container } = render(<OpsStatusStrip />);
    act(() => {
      vi.advanceTimersByTime(1000);
    });
    expect(container.querySelector("time")).toHaveTextContent("04:59:59");
    act(() => {
      vi.advanceTimersByTime(2000);
    });
    expect(container.querySelector("time")).toHaveTextContent("04:59:57");
  });
});

describe("Topbar — icon buttons (Req 6.2, 10.1–10.5)", () => {
  it("theme toggle on dark: label names the target theme, Sun icon, calls onToggleTheme", async () => {
    const { props } = renderTopbar({ sidebarTheme: "dark" });
    const btn = screen.getByRole("button", { name: "Ganti ke sidebar terang" });
    expect(btn.querySelector("svg.lucide-sun")).not.toBeNull();
    await userEvent.setup().click(btn);
    expect(props.onToggleTheme).toHaveBeenCalledTimes(1);
  });

  it("theme toggle on light: 'Ganti ke sidebar gelap' + Moon icon", () => {
    renderTopbar({ sidebarTheme: "light" });
    const btn = screen.getByRole("button", { name: "Ganti ke sidebar gelap" });
    expect(btn.querySelector("svg.lucide-moon")).not.toBeNull();
  });

  it.each<DbRole>(["ADMIN", "ADMIN_PARAM", "APPACCESS", "ATM-USER", "BRANCH-USER", "VENDOR-USER"])(
    "no Pengaturan link in Topbar for %s",
    (role) => {
      useAuthStore.setState({ user: { ...mockUser, role } });
      renderTopbar();
      expect(screen.queryByRole("link", { name: "Pengaturan" })).not.toBeInTheDocument();
    },
  );

  it("renders the notifications slot", () => {
    renderTopbar({ notifications: <button type="button">Lonceng</button> });
    expect(screen.getByRole("button", { name: "Lonceng" })).toBeInTheDocument();
  });
});

describe("UserMenu (Req 11)", () => {
  const openMenu = async () => {
    const user = userEvent.setup();
    const trigger = screen.getByTestId("header-user-name");
    await user.click(trigger);
    return { user, trigger };
  };

  it("trigger shows initials, full name and role; menu closed initially", () => {
    render(<UserMenu />);
    const trigger = screen.getByTestId("header-user-name");
    expect(trigger).toHaveTextContent("BS");
    expect(trigger).toHaveTextContent("Budi Santoso");
    expect(trigger).toHaveTextContent("ADMIN");
    expect(trigger).toHaveAttribute("aria-haspopup", "menu");
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
  });

  it("renders nothing without a user", () => {
    useAuthStore.setState({ user: null });
    const { container } = render(<UserMenu />);
    expect(container).toBeEmptyDOMElement();
  });

  it("opening focuses the first item; email and old test ids present", async () => {
    render(<UserMenu />);
    const { trigger } = await openMenu();
    expect(trigger).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByRole("menu", { name: "Menu pengguna" })).toBeInTheDocument();
    expect(screen.getByRole("menuitem")).toHaveFocus();
    expect(screen.getByTestId("header-email")).toHaveTextContent(mockUser.email);
    expect(screen.getByTestId("header-logout-button")).toHaveTextContent("Keluar");
    expect(screen.getByTestId("header-logout-button")).toHaveAccessibleName("Keluar dari sistem");
  });

  it("ArrowUp/ArrowDown/Home/End are handled and keep focus on a menu item", async () => {
    render(<UserMenu />);
    await openMenu();
    const item = screen.getByRole("menuitem");
    for (const key of ["ArrowDown", "ArrowUp", "Home", "End"]) {
      // fireEvent returns false when preventDefault() was called.
      expect(fireEvent.keyDown(item, { key })).toBe(false);
      expect(item).toHaveFocus();
    }
    expect(fireEvent.keyDown(item, { key: "a" })).toBe(true);
  });

  it("Escape closes the menu and returns focus to the trigger", async () => {
    render(<UserMenu />);
    const { user, trigger } = await openMenu();
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
    expect(trigger).toHaveAttribute("aria-expanded", "false");
  });

  it("pointerdown outside closes the menu; inside does not", async () => {
    render(
      <>
        <button type="button">luar</button>
        <UserMenu />
      </>,
    );
    await openMenu();
    fireEvent.pointerDown(screen.getByRole("menu"));
    expect(screen.getByRole("menu")).toBeInTheDocument();
    fireEvent.pointerDown(screen.getByRole("button", { name: "luar" }));
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
  });

  it("Keluar calls logout", async () => {
    render(<UserMenu />);
    const { user } = await openMenu();
    await user.click(screen.getByTestId("header-logout-button"));
    expect(logout).toHaveBeenCalledTimes(1);
  });

  it("Keluar uses the danger colour token", async () => {
    render(<UserMenu />);
    await openMenu();
    expect(screen.getByTestId("header-logout-button").className).toContain(
      "text-[var(--danger-fg)]",
    );
  });
});

// Dipindahkan dari Header.property.test.tsx (Property 8: user info display).
describe("UserMenu — Property 8: user info display", () => {
  const roles: DbRole[] = ["ADMIN", "ATM-USER", "BRANCH-SPV", "VENDOR-USER"];
  const arbUser: fc.Arbitrary<AuthUser> = fc.record({
    id: fc.integer({ min: 1 }),
    username: fc.string({ minLength: 1 }),
    fullName: fc.string({ minLength: 1, unit: "grapheme" }).filter((s) => s.trim().length > 0),
    email: fc.emailAddress(),
    role: fc.constantFrom(...roles),
    isKaryawan: fc.boolean(),
    vendorId: fc.option(fc.integer({ min: 1 }), { nil: null }),
  });

  it("trigger contains fullName and the menu shows the exact email", () => {
    fc.assert(
      fc.property(arbUser, (user) => {
        useAuthStore.setState({ user });
        const { unmount } = render(<UserMenu />);
        const trigger = screen.getByTestId("header-user-name");
        expect(trigger.textContent).toContain(user.fullName);
        fireEvent.click(trigger);
        expect(screen.getByTestId("header-email").textContent).toBe(user.email);
        unmount();
      }),
      { numRuns: 30 },
    );
  });
});
