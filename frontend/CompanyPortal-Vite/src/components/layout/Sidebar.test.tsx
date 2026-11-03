/**
 * Task 6.6 (app-shell-revamp): Sidebar + sidebar/{Brand,NavGroup,NavItem,Footer}.
 * Req 1.1–1.3, 2.1, 2.3, 2.5, 2.6, 3.3, 3.5–3.8, 4.3–4.5, 5.1, 5.3, 5.4, 16.5.
 */

import type { DbRole } from "@/lib/auth/store";
import { APP_VERSION, ENV_LABEL } from "@/lib/config/appEnv";
import {
  GROUP_LABELS,
  type NavGroup,
  SIDEBAR_GROUP_ORDER,
  getVisibleNav,
} from "@/lib/config/navigation";
import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import * as fc from "fast-check";
import type { ComponentProps, ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { Sidebar } from "./Sidebar";

const SHELL_ROLES: DbRole[] = [
  "ADMIN",
  "ADMIN_PARAM",
  "ATM-USER",
  "ATM-SPV",
  "BRANCH-USER",
  "BRANCH-SPV",
  "BRANCH-ATM-USER",
  "BRANCH-ATM-SPV",
  "VENDOR-USER",
  "APPACCESS",
  "VENDOR-ATM-USER",
  "VENDOR-VAULT-USER",
  "ACM-USER",
  "ACM-SPV",
];
const arbShellRole: fc.Arbitrary<DbRole | null> = fc.oneof(
  fc.constant(null),
  fc.constantFrom(...SHELL_ROLES),
);

// Link jadi <a href> biasa: test ini tidak butuh RouterProvider.
vi.mock("@tanstack/react-router", () => ({
  Link: ({ to, children, ...rest }: { to: string; children: ReactNode }) => (
    <a href={to} {...rest}>
      {children}
    </a>
  ),
}));

const admin = getVisibleNav("ADMIN");
const atmUser = getVisibleNav("ATM-USER");

function renderSidebar(overrides: Partial<ComponentProps<typeof Sidebar>> = {}) {
  const props: ComponentProps<typeof Sidebar> = {
    visible: admin,
    active: undefined,
    badges: {},
    rail: false,
    theme: "dark",
    collapsedGroups: [],
    onToggleGroup: vi.fn(),
    onToggleRail: vi.fn(),
    ...overrides,
  };
  return { props, ...render(<Sidebar {...props} />) };
}

const find = (id: string) => {
  const item = admin.all.find((i) => i.id === id);
  if (!item) throw new Error(`nav item ${id} missing`);
  return item;
};

const multiItemGroup = () =>
  SIDEBAR_GROUP_ORDER.find((g) => (admin.sidebar.get(g)?.length ?? 0) > 1) as NavGroup;

const navRegion = () => screen.getByRole("navigation", { name: "Menu navigasi" });

describe("Sidebar — brand & rail", () => {
  it("expanded: shows CROWN + Cash Management, online status, 264px", () => {
    renderSidebar();
    expect(screen.getByText("CROWN")).toBeInTheDocument();
    expect(screen.getByText("Cash Management")).toBeInTheDocument();
    expect(screen.getByText("Status: online")).toBeInTheDocument();
    expect(screen.getByRole("complementary", { name: "Navigasi utama" })).toHaveStyle({
      "--sb-w": "264px",
    });
  });

  it("rail: hides brand text, keeps logo + status, 76px (Req 1.3)", () => {
    renderSidebar({ rail: true });
    expect(screen.queryByText("CROWN")).not.toBeInTheDocument();
    expect(screen.queryByText("Cash Management")).not.toBeInTheDocument();
    expect(screen.getByText("Status: online")).toBeInTheDocument();
    expect(screen.getByRole("complementary")).toHaveStyle({ "--sb-w": "76px" });
  });
});

describe("Sidebar — groups", () => {
  it("renders groups in SIDEBAR_GROUP_ORDER for the role (Req 2.1)", () => {
    renderSidebar({ visible: atmUser });
    const expected = SIDEBAR_GROUP_ORDER.filter((g) => atmUser.sidebar.has(g)).map(
      (g) => GROUP_LABELS[g],
    );
    const rendered = within(navRegion())
      .getAllByRole("button", { expanded: true })
      .map((b) => b.textContent);
    expect(rendered).toEqual(expected);
  });

  it("renders Pengaturan in the footer, outside the 'Menu navigasi' nav (Req 1.2)", () => {
    renderSidebar();
    const { label } = find("settings");
    const link = screen.getByRole("link", { name: label });
    expect(within(navRegion()).queryByRole("link", { name: label })).not.toBeInTheDocument();
    expect(navRegion().contains(link)).toBe(false);
    expect(screen.getByRole("complementary").contains(link)).toBe(true);
    expect(screen.queryByText(GROUP_LABELS.system)).not.toBeInTheDocument();
  });

  it("toggle reflects aria-expanded and calls onToggleGroup (Req 2.5)", async () => {
    const group = SIDEBAR_GROUP_ORDER[0] as NavGroup;
    const { props } = renderSidebar({ collapsedGroups: [group] });
    const btn = screen.getByRole("button", { name: GROUP_LABELS[group] });
    expect(btn).toHaveAttribute("aria-expanded", "false");
    await userEvent.setup().click(btn);
    expect(props.onToggleGroup).toHaveBeenCalledWith(group);
  });

  it("collapsed group keeps the active item, hides the rest as inert (Req 2.6)", () => {
    const group = multiItemGroup();
    const [first, second] = admin.sidebar.get(group) ?? [];
    renderSidebar({ collapsedGroups: [group], active: first });
    expect(screen.getByRole("link", { name: first?.label }).closest("[inert]")).toBeNull();
    expect(screen.getByText(second?.label ?? "").closest("[inert]")).not.toBeNull();
  });
});

describe("Sidebar — nav item state", () => {
  it("every link has the right href", () => {
    renderSidebar();
    for (const item of admin.all.filter((i) => i.placement !== "sidebar-footer")) {
      expect(screen.getByRole("link", { name: item.label })).toHaveAttribute("href", item.href);
    }
    const settings = find("settings");
    expect(screen.getByRole("link", { name: settings.label })).toHaveAttribute(
      "href",
      settings.href,
    );
  });

  it("Pengaturan has aria-current=page only when it is the active item (Req 1.4)", () => {
    const settings = find("settings");
    const { unmount } = renderSidebar({ active: settings });
    expect(screen.getByRole("link", { name: settings.label })).toHaveAttribute(
      "aria-current",
      "page",
    );
    unmount();

    renderSidebar({ active: find("dashboard") });
    expect(screen.getByRole("link", { name: settings.label })).not.toHaveAttribute("aria-current");
  });

  it("only the active item has aria-current=page (Req 3.3)", () => {
    const active = find("dashboard");
    renderSidebar({ active });
    expect(screen.getByRole("link", { name: active.label })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(navRegion().querySelectorAll('[aria-current="page"]')).toHaveLength(1);
  });

  it("count badge: number shown, name '{label}, {n} menunggu' (Req 3.5, 3.7)", () => {
    renderSidebar({ badges: { "approvals-inbox": { kind: "count", value: "12", raw: 12 } } });
    const link = screen.getByRole("link", { name: `${find("approvals").label}, 12 menunggu` });
    expect(link).toHaveTextContent("12");
  });

  it("dot badge: pulse + name 'perlu perhatian' (Req 3.6, 3.7)", () => {
    renderSidebar({ badges: { "eod-health": { kind: "dot" } } });
    const link = screen.getByRole("link", {
      name: `${find("eod-monitoring").label}, perlu perhatian`,
    });
    expect(link.querySelector('[class*="animate-pulse"]')).not.toBeNull();
  });
});

describe("Sidebar — rail mode", () => {
  it("replaces group labels with dividers and hides group toggles (Req 4.3)", () => {
    const { container } = renderSidebar({ rail: true });
    expect(screen.queryByText(GROUP_LABELS.general)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { expanded: true })).not.toBeInTheDocument();
    expect(container.querySelectorAll("hr")).toHaveLength(admin.sidebar.size);
  });

  it("badge becomes a corner dot; name still carries the count", () => {
    renderSidebar({
      rail: true,
      badges: { "approvals-inbox": { kind: "count", value: "12", raw: 12 } },
    });
    const link = screen.getByRole("link", { name: `${find("approvals").label}, 12 menunggu` });
    expect(link).not.toHaveTextContent("12");
    expect(link.querySelector("span.absolute")).not.toBeNull();
  });

  it("tooltip shows on hover and on focus, linked by aria-describedby (Req 4.4)", () => {
    const item = find("dashboard");
    renderSidebar({ rail: true });
    const link = screen.getByRole("link", { name: item.label });
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();

    fireEvent.mouseEnter(link);
    let tip = screen.getByRole("tooltip");
    expect(tip).toHaveTextContent(item.label);
    expect(link).toHaveAttribute("aria-describedby", tip.id);
    fireEvent.mouseLeave(link);
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();

    fireEvent.focus(link);
    tip = screen.getByRole("tooltip");
    expect(link).toHaveAttribute("aria-describedby", tip.id);
    fireEvent.blur(link);
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();
  });

  it("tooltip includes the badge value (Req 4.4)", () => {
    renderSidebar({
      rail: true,
      badges: { "approvals-inbox": { kind: "count", value: "12", raw: 12 } },
    });
    const name = `${find("approvals").label}, 12 menunggu`;
    fireEvent.focus(screen.getByRole("link", { name }));
    expect(screen.getByRole("tooltip")).toHaveTextContent(name);
  });

  it("Pengaturan tooltip shows on hover and focus, linked by aria-describedby (Req 1.5)", () => {
    const { label } = find("settings");
    renderSidebar({ rail: true });
    const link = screen.getByRole("link", { name: label });

    fireEvent.mouseEnter(link);
    let tip = screen.getByRole("tooltip");
    expect(tip).toHaveTextContent(label);
    expect(link).toHaveAttribute("aria-describedby", tip.id);
    fireEvent.mouseLeave(link);
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();

    fireEvent.focus(link);
    tip = screen.getByRole("tooltip");
    expect(link).toHaveAttribute("aria-describedby", tip.id);
  });

  it("expanded mode never shows a tooltip", () => {
    renderSidebar();
    fireEvent.mouseEnter(screen.getByRole("link", { name: find("dashboard").label }));
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();
  });
});

describe("Sidebar — footer", () => {
  // Feature: sidebar-settings-footer, Property 1 (sisi Sidebar)
  it("Property 1: Pengaturan link exists ⇔ getVisibleNav(role).settings is defined", () => {
    fc.assert(
      fc.property(arbShellRole, fc.boolean(), (role, rail) => {
        const visible = getVisibleNav(role);
        const { unmount } = renderSidebar({ visible, rail });
        const link = screen.queryByRole("link", { name: "Pengaturan" });
        const ok =
          (link !== null) === (visible.settings !== undefined) &&
          (link === null ||
            (!navRegion().contains(link) && link.getAttribute("href") === "/settings"));
        unmount();
        return ok;
      }),
      { numRuns: 40 },
    );
  });

  // Feature: sidebar-settings-footer, Property 3
  it("Property 3: no 'Lingkungan', ENV_LABEL or version text; collapse button stays", () => {
    const forbidden = ["Lingkungan", ...Object.values(ENV_LABEL), `v${APP_VERSION}`];
    fc.assert(
      fc.property(arbShellRole, fc.boolean(), (role, rail) => {
        const { container, unmount } = renderSidebar({ visible: getVisibleNav(role), rail });
        const text = container.textContent ?? "";
        const ok =
          forbidden.every((t) => !text.includes(t)) &&
          screen.queryByRole("button", { name: rail ? "Lebarkan menu" : "Ciutkan menu" }) !== null;
        unmount();
        return ok;
      }),
      { numRuns: 40 },
    );
  });

  it("expanded: ChevronsLeft + 'Ciutkan menu' + kbd [; rail: ChevronsRight + 'Lebarkan menu' (Req 4.5)", async () => {
    const { props, unmount } = renderSidebar();
    const btn = screen.getByRole("button", { name: "Ciutkan menu" });
    expect(btn.querySelector("svg.lucide-chevrons-left")).not.toBeNull();
    expect(within(btn).getByText("[")).toBeInTheDocument();
    await userEvent.setup().click(btn);
    expect(props.onToggleRail).toHaveBeenCalledTimes(1);
    unmount();

    renderSidebar({ rail: true });
    const railBtn = screen.getByRole("button", { name: "Lebarkan menu" });
    expect(railBtn.querySelector("svg.lucide-chevrons-right")).not.toBeNull();
  });
});

describe("Sidebar — keyboard (Req 3.8, 16.5)", () => {
  it("roving tabindex: only the active item is a tab stop", () => {
    const active = find("dashboard");
    renderSidebar({ active });
    const stops = within(navRegion())
      .getAllByRole("link")
      .filter((a) => a.tabIndex === 0);
    expect(stops).toEqual([screen.getByRole("link", { name: active.label })]);
  });

  it("Pengaturan link is reachable with Tab (Req 1.6)", async () => {
    renderSidebar();
    const settings = screen.getByRole("link", { name: find("settings").label });
    const user = userEvent.setup();
    for (let i = 0; i < 100 && document.activeElement !== settings; i++) {
      await user.tab();
    }
    expect(settings).toHaveFocus();
  });

  it("ArrowDown/ArrowUp move focus with wrap; Home/End jump", async () => {
    renderSidebar();
    const links = within(navRegion()).getAllByRole("link");
    const user = userEvent.setup();

    links[0]?.focus();
    await user.keyboard("{ArrowDown}");
    expect(links[1]).toHaveFocus();
    await user.keyboard("{ArrowUp}");
    expect(links[0]).toHaveFocus();
    await user.keyboard("{ArrowUp}");
    expect(links.at(-1)).toHaveFocus();
    await user.keyboard("{ArrowDown}");
    expect(links[0]).toHaveFocus();
    await user.keyboard("{End}");
    expect(links.at(-1)).toHaveFocus();
    await user.keyboard("{Home}");
    expect(links[0]).toHaveFocus();
  });

  it("skips items inside collapsed (inert) groups", async () => {
    const group = multiItemGroup();
    renderSidebar({ collapsedGroups: [group] });
    const hidden = (admin.sidebar.get(group) ?? []).map((i) => i.label);
    const links = within(navRegion()).getAllByRole("link");
    links[0]?.focus();
    const user = userEvent.setup();
    for (let i = 0; i < links.length; i++) {
      await user.keyboard("{ArrowDown}");
      expect(hidden).not.toContain(document.activeElement?.textContent);
    }
  });
});
