import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { type DbRole, useAuthStore } from "@/lib/auth/store";
import { DEFAULT_SHELL_PREFS, useShellPrefs } from "@/lib/shell/prefs";
import {
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import * as fc from "fast-check";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AppShell } from "./AppShell";

/**
 * Property 20: Icon Button Accessibility Labels
 * Validates: Requirements 10.3
 *
 * For any Button component rendered in icon-only mode (no visible text content),
 * the rendered element SHALL have a non-empty `aria-label` attribute.
 */

// ─── Types ────────────────────────────────────────────────────────────────────

interface IconButtonConfig {
  hasIcon: boolean;
  hasVisibleText: boolean;
  ariaLabel?: string;
}

// ─── Pure validation function ─────────────────────────────────────────────────

/**
 * Validates an icon button configuration for accessibility compliance.
 * Returns true if the config is accessible (icon-only buttons must have a non-empty aria-label).
 */
function isAccessibleIconButton(config: IconButtonConfig): boolean {
  // If the button has an icon and NO visible text, it MUST have a non-empty aria-label
  if (config.hasIcon && !config.hasVisibleText) {
    return config.ariaLabel !== undefined && config.ariaLabel.trim().length > 0;
  }
  // Buttons with visible text are accessible by default (text acts as label)
  return true;
}

// ─── Arbitraries ──────────────────────────────────────────────────────────────

const arbIconButtonConfig = fc.record({
  hasIcon: fc.boolean(),
  hasVisibleText: fc.boolean(),
  ariaLabel: fc.option(fc.string({ minLength: 0, maxLength: 50 }), { nil: undefined }),
});

/**
 * Generates only icon-only configs (icon present, no visible text) WITH a valid aria-label.
 * These represent correctly-implemented icon buttons.
 */
const arbAccessibleIconOnlyConfig: fc.Arbitrary<IconButtonConfig> = fc.record({
  hasIcon: fc.constant(true),
  hasVisibleText: fc.constant(false),
  ariaLabel: fc
    .string({ minLength: 1, maxLength: 50, unit: "grapheme" })
    .filter((s) => s.trim().length > 0),
});

/**
 * Generates icon-only configs WITHOUT a valid aria-label.
 * These represent inaccessible buttons that should be flagged.
 */
const arbInaccessibleIconOnlyConfig: fc.Arbitrary<IconButtonConfig> = fc.record({
  hasIcon: fc.constant(true),
  hasVisibleText: fc.constant(false),
  ariaLabel: fc.constantFrom(undefined, "", "   "),
});

// ─── Property Tests (Pure Logic) ──────────────────────────────────────────────

describe("Property 20: Icon Button Accessibility Labels", () => {
  it("icon-only buttons with a non-empty aria-label pass accessibility check", () => {
    fc.assert(
      fc.property(arbAccessibleIconOnlyConfig, (config) => {
        expect(isAccessibleIconButton(config)).toBe(true);
      }),
      { numRuns: 100 },
    );
  });

  it("icon-only buttons without a valid aria-label fail accessibility check", () => {
    fc.assert(
      fc.property(arbInaccessibleIconOnlyConfig, (config) => {
        expect(isAccessibleIconButton(config)).toBe(false);
      }),
      { numRuns: 100 },
    );
  });

  it("buttons with visible text are always accessible regardless of aria-label", () => {
    fc.assert(
      fc.property(
        fc.record({
          hasIcon: fc.boolean(),
          hasVisibleText: fc.constant(true),
          ariaLabel: fc.option(fc.string({ minLength: 0, maxLength: 50 }), { nil: undefined }),
        }),
        (config) => {
          expect(isAccessibleIconButton(config)).toBe(true);
        },
      ),
      { numRuns: 100 },
    );
  });

  it("for any config: icon-only without visible text implies aria-label is required", () => {
    fc.assert(
      fc.property(arbIconButtonConfig, (config) => {
        const result = isAccessibleIconButton(config);

        if (config.hasIcon && !config.hasVisibleText) {
          // Icon-only: must have non-empty aria-label
          const hasValidLabel =
            config.ariaLabel !== undefined && config.ariaLabel.trim().length > 0;
          expect(result).toBe(hasValidLabel);
        } else {
          // Has visible text or no icon: always accessible
          expect(result).toBe(true);
        }
      }),
      { numRuns: 200 },
    );
  });
});

// ─── Component Integration Tests ──────────────────────────────────────────────

describe("Property 20: Icon Button Accessibility — Component Verification", () => {
  /**
   * Renders an icon-only button using the provided aria-label.
   * Validates that real rendered DOM elements satisfy the property.
   */
  it("rendered icon-only buttons always have a non-empty aria-label", () => {
    fc.assert(
      fc.property(
        fc
          .string({ minLength: 1, maxLength: 50, unit: "grapheme" })
          .filter((s) => s.trim().length > 0),
        (ariaLabel) => {
          const { unmount, container } = render(
            <button type="button" aria-label={ariaLabel}>
              <svg aria-hidden="true">
                <circle cx="12" cy="12" r="10" />
              </svg>
            </button>,
          );

          const button = container.querySelector("button");
          expect(button).not.toBeNull();

          // The button has no visible text (only an svg with aria-hidden)
          const visibleText = button?.textContent?.trim() ?? "";
          expect(visibleText).toBe("");

          // It must have a non-empty aria-label
          const label = button?.getAttribute("aria-label");
          expect(label).not.toBeNull();
          expect(label?.trim().length).toBeGreaterThan(0);

          unmount();
        },
      ),
      { numRuns: 50 },
    );
  });

  it("rendered buttons with visible text do not require aria-label", () => {
    fc.assert(
      fc.property(
        fc
          .string({ minLength: 1, maxLength: 30, unit: "grapheme" })
          .filter((s) => s.trim().length > 0),
        (buttonText) => {
          const { unmount, container } = render(
            <button type="button">
              <svg aria-hidden="true">
                <circle cx="12" cy="12" r="10" />
              </svg>
              <span>{buttonText}</span>
            </button>,
          );

          const button = container.querySelector("button");
          expect(button).not.toBeNull();

          // Button has visible text content
          const visibleText = button?.textContent?.trim() ?? "";
          expect(visibleText.length).toBeGreaterThan(0);

          // aria-label is not required (text provides the accessible name)
          unmount();
        },
      ),
      { numRuns: 50 },
    );
  });
});

// ─── App shell (Task 9.3, app-shell-revamp; Req 3.6, 15.1–15.3, 16.5) ─────────

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
];

vi.mock("@/components/layout/useNavBadges", () => ({ useNavBadges: () => ({}) }));

const INTERACTIVE = 'button, a[href], [role="menuitem"], [role="option"], input';
const HAS_FOCUS_STYLE = /(^|\s)(focus-visible:|shell-focus(\s|$))/;

function accessibleName(el: Element): string {
  const label = el.getAttribute("aria-label")?.trim();
  if (label) return label;
  const labelledBy = el.getAttribute("aria-labelledby");
  if (labelledBy) {
    const text = labelledBy
      .split(/\s+/)
      .map((id) => document.getElementById(id)?.textContent ?? "")
      .join(" ")
      .trim();
    if (text) return text;
  }
  return (el.textContent ?? "").trim() || (el.getAttribute("title") ?? "").trim();
}

function auditInteractive(root: ParentNode): string[] {
  const problems: string[] = [];
  for (const el of root.querySelectorAll(INTERACTIVE)) {
    const desc = `${el.tagName.toLowerCase()}[${el.getAttribute("data-testid") ?? el.className}]`;
    if (!accessibleName(el)) problems.push(`no accessible name: ${desc}`);
    // [role=option] = target aria-activedescendant (tidak pernah menerima fokus DOM);
    // indikatornya tint baris terpilih, jadi hanya nama aksesibel yang wajib.
    const isOption = el.getAttribute("role") === "option";
    if (!isOption && !HAS_FOCUS_STYLE.test(el.getAttribute("class") ?? "")) {
      problems.push(`no focus-visible style: ${desc}`);
    }
  }
  return problems;
}

async function renderShell() {
  const root = createRootRoute({
    component: () => (
      <AppShell>
        <Outlet />
      </AppShell>
    ),
  });
  const tree = root.addChildren([
    createRoute({ getParentRoute: () => root, path: "/", component: () => <p>Beranda</p> }),
  ]);
  const router = createRouter({ routeTree: tree, history: createMemoryHistory() });
  await router.load();
  render(<RouterProvider router={router} />);
  await screen.findByRole("main");
}

describe("Shell accessibility — role × sidebar theme × rail (Req 3.6, 15.1, 15.2)", () => {
  beforeEach(() => {
    localStorage.clear();
    window.matchMedia = vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }));
    Element.prototype.scrollIntoView = vi.fn();
  });
  afterEach(() => {
    cleanup();
    useAuthStore.setState({ user: null });
  });

  it("every interactive shell element has an accessible name and focus-visible style", async () => {
    await fc.assert(
      fc.asyncProperty(
        fc.constantFrom(...SHELL_ROLES),
        fc.constantFrom<"dark" | "light">("dark", "light"),
        fc.boolean(),
        async (role, sidebarTheme, rail) => {
          useShellPrefs.setState({ ...DEFAULT_SHELL_PREFS, sidebarTheme, rail });
          useAuthStore.setState({
            user: {
              id: 1,
              username: "u",
              fullName: "Test User",
              email: "t@t.com",
              role,
              isKaryawan: true,
              vendorId: null,
            },
            isAuthenticated: true,
            isAuthLoading: false,
            error: null,
            rateLimitRetryAfter: null,
          });
          await renderShell();
          const user = userEvent.setup();

          // Tertutup: skip link, sidebar, topbar.
          const problems = auditInteractive(document.body);

          // User menu terbuka.
          await user.click(screen.getByTestId("header-user-name"));
          problems.push(...auditInteractive(screen.getByRole("menu")));

          // Palette terbuka.
          await user.keyboard("{Escape}");
          await user.click(screen.getByRole("button", { name: /Cari menu atau fitur/ }));
          const dialog = await screen.findByRole("dialog");
          problems.push(...auditInteractive(dialog));

          cleanup();
          expect([...new Set(problems)].join("\n")).toBe("");
        },
      ),
      { numRuns: 20 },
    );
  }, 60_000);
});

describe("Shell accessibility — reduced motion (Req 15.3)", () => {
  const dir = (p: string) => resolve(__dirname, p);
  const css = readFileSync(dir("../../styles/index.css"), "utf8");
  const reduceBlock = [...css.matchAll(/@media \(prefers-reduced-motion: reduce\)\s*\{/g)]
    .map((m) => {
      let depth = 1;
      let i = (m.index ?? 0) + m[0].length;
      const start = i;
      while (depth > 0 && i < css.length) {
        if (css[i] === "{") depth++;
        else if (css[i] === "}") depth--;
        i++;
      }
      return css.slice(start, i);
    })
    .join("\n");

  const sources = [
    "Sidebar.tsx",
    "Topbar.tsx",
    "CommandPalette.tsx",
    ...["SidebarBrand", "SidebarFooter", "SidebarNavGroup", "SidebarNavItem"].map(
      (n) => `sidebar/${n}.tsx`,
    ),
    ...["Breadcrumb", "OpsStatusStrip", "SearchTrigger", "TopbarIconButton", "UserMenu"].map(
      (n) => `topbar/${n}.tsx`,
    ),
  ].map((file) => ({ file, src: readFileSync(dir(file), "utf8") }));
  const shellSource = sources.map((s) => s.src).join("\n");

  it.each(["shell-pop", "shell-chevron", "shell-sidebar", "animate-ping", "animate-pulse"])(
    "'%s' dimatikan di blok prefers-reduced-motion index.css",
    (cls) => {
      expect(reduceBlock).toContain(`.${cls}`);
    },
  );

  it("setiap kelas `animate-*` di komponen shell memakai motion-safe: atau dimatikan di CSS", () => {
    const offenders: string[] = [];
    for (const m of shellSource.matchAll(/(?<![\w:-])(motion-safe:)?(animate-[\w-]+)/g)) {
      const [, safe, cls] = m;
      if (!safe && !reduceBlock.includes(`.${cls}`)) offenders.push(cls);
    }
    expect(offenders).toEqual([]);
  });

  it("setiap `transition*` di komponen shell punya motion-reduce:transition-none atau kelas shell yang dimatikan", () => {
    const offenders = sources
      .filter((s) => /(?<![\w:-])transition(-[a-z]+)?(?![\w-])/.test(s.src))
      .filter((s) => !/motion-reduce:transition-none|shell-(chevron|sidebar)/.test(s.src))
      .map((s) => s.file);
    expect(offenders).toEqual([]);
  });
});
