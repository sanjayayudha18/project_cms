/**
 * Task 13.1 unit tests (design.md Testing Strategy table):
 * - "Sidebar renders 'Monitoring' group after 'Umum'" (Req 6.1)
 * - "'ATM Portal' link with Monitor icon navigates to /atm-portal" (Req 6.2, 6.3)
 *
 * Icon identity (Monitor) is already asserted against NAV_CONFIG directly in
 * navigation.test.ts — this file focuses on Sidebar's own rendering/ordering
 * and click-to-navigate behavior, not re-deriving icon equality from the DOM.
 */

import { useAuthStore } from "@/lib/auth/store";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { Sidebar } from "./Sidebar";

function setUser(role: "ATM-USER" | "ATM-SPV" | "VENDOR-USER") {
  useAuthStore.setState({
    user: {
      id: 1,
      username: "test.user",
      fullName: "Test User",
      email: "test@example.com",
      role,
      isKaryawan: true,
      vendorId: null,
    },
    accessToken: "test-token",
    isAuthenticated: true,
  });
}

describe("Sidebar", () => {
  beforeEach(() => {
    useAuthStore.setState({ user: null, accessToken: null, isAuthenticated: false });
  });

  it("renders the 'Monitoring' group after 'Umum'", () => {
    setUser("ATM-USER");
    render(<Sidebar collapsed={false} onToggle={() => {}} />);

    const groups = screen.getAllByRole("group").map((g) => g.getAttribute("aria-label"));
    const umumIndex = groups.indexOf("Umum");
    const monitoringIndex = groups.indexOf("Monitoring");

    expect(umumIndex).toBeGreaterThanOrEqual(0);
    expect(monitoringIndex).toBeGreaterThan(umumIndex);
  });

  it("clicking the 'ATM Portal' link calls onNavigate with /atm-portal", async () => {
    setUser("ATM-SPV");
    const onNavigate = vi.fn();
    render(<Sidebar collapsed={false} onToggle={() => {}} onNavigate={onNavigate} />);

    const user = userEvent.setup();
    await user.click(screen.getByText("ATM Portal"));

    expect(onNavigate).toHaveBeenCalledWith("/atm-portal");
  });

  it("does not render the ATM Portal link for a role without access (VENDOR-USER)", () => {
    setUser("VENDOR-USER");
    render(<Sidebar collapsed={false} onToggle={() => {}} />);

    expect(screen.queryByText("ATM Portal")).not.toBeInTheDocument();
  });
});

// ─── fe-coverage C5: keyboard, disabled items, active item, collapsed tooltip ─

describe("Sidebar (C5 behaviour)", () => {
  beforeEach(() => {
    useAuthStore.setState({ user: null, accessToken: null, isAuthenticated: false });
  });

  const items = () => Array.from(document.querySelectorAll<HTMLButtonElement>("[data-nav-item]"));

  it("renders nothing to navigate without a signed-in user", () => {
    render(<Sidebar collapsed={false} onToggle={vi.fn()} />);

    expect(items()).toHaveLength(0);
    fireEvent.keyDown(screen.getByRole("navigation", { name: "Menu navigasi" }), {
      key: "ArrowDown",
    });
  });

  it("arrow keys move focus (wrapping) and Enter/Space open the focused item", () => {
    setUser("ATM-USER");
    const onNavigate = vi.fn();
    render(<Sidebar collapsed={false} onToggle={vi.fn()} onNavigate={onNavigate} />);
    const nav = screen.getByRole("navigation", { name: "Menu navigasi" });
    const all = items();

    fireEvent.keyDown(nav, { key: "ArrowDown" });
    expect(document.activeElement).toBe(all[0]);
    fireEvent.keyDown(nav, { key: "ArrowUp" });
    expect(document.activeElement).toBe(all.at(-1));
    fireEvent.keyDown(nav, { key: "ArrowDown" });
    expect(document.activeElement).toBe(all[0]);

    fireEvent.keyDown(nav, { key: "Enter" });
    expect(onNavigate).toHaveBeenCalledWith("/");
    fireEvent.keyDown(nav, { key: "ArrowDown" });
    fireEvent.keyDown(nav, { key: " " });
    expect(onNavigate).toHaveBeenCalledTimes(2);
    fireEvent.keyDown(nav, { key: "Tab" });
    expect(onNavigate).toHaveBeenCalledTimes(2);
  });

  it("marks the current page and leaves its styling alone on hover / focus", () => {
    setUser("ATM-USER");
    render(<Sidebar collapsed={false} onToggle={vi.fn()} currentPath="/atm-portal" />);
    const active = screen.getByRole("button", { name: "ATM Portal" });
    const other = screen.getByRole("button", { name: "Dashboard" });

    expect(active).toHaveAttribute("aria-current", "page");
    expect(other).not.toHaveAttribute("aria-current");

    for (const [el, enter, leave] of [
      [other, fireEvent.mouseOver, fireEvent.mouseOut],
      [other, fireEvent.focus, fireEvent.blur],
    ] as const) {
      enter(el);
      expect(el.style.backgroundColor).toBe("var(--n-100)");
      leave(el);
      expect(el.style.backgroundColor).toBe("transparent");
    }
    fireEvent.mouseOver(active);
    expect(active.style.backgroundColor).toBe("var(--red-50)");
  });

  it("collapsed: no labels, shown as hover tooltips instead", () => {
    setUser("ATM-USER");
    render(<Sidebar collapsed onToggle={vi.fn()} />);

    expect(screen.queryByText("Dashboard")).not.toBeInTheDocument();
    const [first] = items();
    fireEvent.mouseEnter(first as HTMLElement);
    expect(screen.getByRole("tooltip")).toHaveTextContent("Dashboard");
    fireEvent.mouseLeave(first as HTMLElement);
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();
  });

  it("expanded: hovering never shows a tooltip", () => {
    setUser("ATM-USER");
    render(<Sidebar collapsed={false} onToggle={vi.fn()} />);

    fireEvent.mouseEnter(items()[0] as HTMLElement);

    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();
  });
});
