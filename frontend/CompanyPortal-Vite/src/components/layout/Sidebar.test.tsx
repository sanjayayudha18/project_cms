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
import { render, screen } from "@testing-library/react";
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
