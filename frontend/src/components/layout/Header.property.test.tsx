import type { AuthUser, Role } from "@/lib/auth";
import { fireEvent, render, screen } from "@testing-library/react";
import * as fc from "fast-check";
import { describe, expect, it, vi } from "vitest";
import { Header } from "./Header";

/**
 * Property 8: Header User Info Display
 * Validates: Requirements 1.7
 *
 * For any valid AuthUser object with a non-empty fullName and a valid primaryRole,
 * rendering the Header component SHALL produce output containing the user's fullName
 * text and the primaryRole badge text.
 */
describe("Header — Property 8: Header User Info Display", () => {
  const ALL_ROLES: Role[] = [
    "Admin",
    "ATM_Support",
    "Cash_Management",
    "Vendor",
    "WMO",
    "Finance",
    "Cash_Count_PIC",
    "Cash_Count_Lead",
    "Branch",
    "Approver",
  ];

  const arbRole = fc.constantFrom(...ALL_ROLES);

  const arbRolesIncluding = (primary: Role) =>
    fc
      .uniqueArray(arbRole, { minLength: 1 })
      .map((roles) => (roles.includes(primary) ? roles : [primary, ...roles]));

  const arbAuthUser: fc.Arbitrary<AuthUser> = arbRole.chain((primaryRole) =>
    fc.record({
      id: fc.string({ minLength: 1 }),
      fullName: fc.string({ minLength: 1, unit: "grapheme" }).filter((s) => s.trim().length > 0),
      email: fc.emailAddress(),
      roles: arbRolesIncluding(primaryRole),
      primaryRole: fc.constant(primaryRole),
    }),
  );

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

  it("rendered output contains the user's primaryRole in the badge", () => {
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

        // The role badge lives inside the user dropdown — open it first.
        fireEvent.click(screen.getByTestId("header-user-name"));

        const badgeEl = screen.getByTestId("header-role-badge");
        expect(badgeEl.textContent).toBe(user.primaryRole);

        unmount();
      }),
      { numRuns: 50 },
    );
  });
});
