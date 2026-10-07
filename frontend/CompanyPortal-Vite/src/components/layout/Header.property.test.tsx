import type { AuthUser, DbRole } from "@/lib/auth";
import { fireEvent, render, screen } from "@testing-library/react";
import * as fc from "fast-check";
import { describe, expect, it, vi } from "vitest";
import { Header } from "./Header";

/**
 * Property 8: Header User Info Display
 * Validates: Requirements 1.7
 *
 * For any valid AuthUser object with a non-empty fullName, rendering the Header
 * component SHALL produce output containing the user's fullName and email text.
 */
describe("Header — Property 8: Header User Info Display", () => {
  const ALL_ROLES: DbRole[] = [
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

  const arbAuthUser: fc.Arbitrary<AuthUser> = fc.record({
    id: fc.integer({ min: 1 }),
    username: fc.string({ minLength: 1 }),
    fullName: fc.string({ minLength: 1, unit: "grapheme" }).filter((s) => s.trim().length > 0),
    email: fc.emailAddress(),
    role: fc.constantFrom(...ALL_ROLES),
    isKaryawan: fc.boolean(),
    vendorId: fc.option(fc.integer({ min: 1 }), { nil: null }),
  });

  const arbSidebarCollapsed = fc.boolean();

  const noop = vi.fn();

  it("rendered output contains the user's fullName", () => {
    fc.assert(
      fc.property(arbAuthUser, arbSidebarCollapsed, (user, sidebarCollapsed) => {
        const { unmount } = render(
          <Header
            user={user}
            onLogout={noop}
            onSidebarToggle={noop}
            sidebarCollapsed={sidebarCollapsed}
          />,
        );

        // The user button always shows the name (plus role subtext + chevron),
        // so the fullName must be present in the rendered output.
        const nameEl = screen.getByTestId("header-user-name");
        expect(nameEl.textContent).toContain(user.fullName);

        unmount();
      }),
      { numRuns: 50 },
    );
  });

  it("rendered output contains the user's email in the dropdown", () => {
    fc.assert(
      fc.property(arbAuthUser, arbSidebarCollapsed, (user, sidebarCollapsed) => {
        const { unmount } = render(
          <Header
            user={user}
            onLogout={noop}
            onSidebarToggle={noop}
            sidebarCollapsed={sidebarCollapsed}
          />,
        );

        // The email lives inside the user dropdown — open it first.
        fireEvent.click(screen.getByTestId("header-user-name"));

        const emailEl = screen.getByTestId("header-email");
        expect(emailEl.textContent).toBe(user.email);

        unmount();
      }),
      { numRuns: 50 },
    );
  });
});
