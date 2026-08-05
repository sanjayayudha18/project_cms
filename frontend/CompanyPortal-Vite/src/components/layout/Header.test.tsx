import type { AuthUser } from "@/lib/auth";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { Header } from "./Header";

const mockUser: AuthUser = {
  id: "user-1",
  fullName: "Budi Santoso",
  email: "budi@cimb.com",
  roles: ["Admin", "Approver"],
  primaryRole: "Admin",
};

function renderHeader(overrides: Partial<Parameters<typeof Header>[0]> = {}) {
  const defaultProps = {
    user: mockUser,
    onLogout: vi.fn(),
    onSidebarToggle: vi.fn(),
    sidebarCollapsed: false,
  };

  return {
    ...render(<Header {...defaultProps} {...overrides} />),
    props: { ...defaultProps, ...overrides },
  };
}

describe("Header", () => {
  it("displays user full name", () => {
    renderHeader();
    expect(screen.getByTestId("header-user-name")).toHaveTextContent("Budi Santoso");
  });

  it("displays primary role in a badge", () => {
    renderHeader();
    // Role badge lives inside the user dropdown — open it first.
    fireEvent.click(screen.getByTestId("header-user-name"));
    expect(screen.getByTestId("header-role-badge")).toHaveTextContent("Admin");
  });

  it("displays different role correctly", () => {
    const user: AuthUser = {
      ...mockUser,
      primaryRole: "Cash_Management",
    };
    renderHeader({ user });
    fireEvent.click(screen.getByTestId("header-user-name"));
    expect(screen.getByTestId("header-role-badge")).toHaveTextContent("Cash_Management");
  });

  it("calls onLogout when logout button is clicked", () => {
    const onLogout = vi.fn();
    renderHeader({ onLogout });

    // Logout button lives inside the user dropdown — open it first.
    fireEvent.click(screen.getByTestId("header-user-name"));
    fireEvent.click(screen.getByTestId("header-logout-button"));
    expect(onLogout).toHaveBeenCalledTimes(1);
  });

  it("renders logout button with Bahasa Indonesia label", () => {
    renderHeader();
    fireEvent.click(screen.getByTestId("header-user-name"));
    expect(screen.getByTestId("header-logout-button")).toHaveTextContent("Keluar");
    expect(screen.getByTestId("header-logout-button")).toHaveAccessibleName("Keluar dari sistem");
  });

  it("renders hamburger menu button with lg:hidden class for mobile-only visibility", () => {
    renderHeader();
    const hamburger = screen.getByLabelText("Tutup menu navigasi");
    expect(hamburger).toBeInTheDocument();
    expect(hamburger.className).toContain("lg:hidden");
  });

  it("calls onSidebarToggle when hamburger is clicked", () => {
    const onSidebarToggle = vi.fn();
    renderHeader({ onSidebarToggle });

    const hamburger = screen.getByLabelText("Tutup menu navigasi");
    fireEvent.click(hamburger);
    expect(onSidebarToggle).toHaveBeenCalledTimes(1);
  });

  it("shows correct aria-label for hamburger when sidebar is collapsed", () => {
    renderHeader({ sidebarCollapsed: true });
    expect(screen.getByLabelText("Buka menu navigasi")).toBeInTheDocument();
  });

  it("shows correct aria-label for hamburger when sidebar is expanded", () => {
    renderHeader({ sidebarCollapsed: false });
    expect(screen.getByLabelText("Tutup menu navigasi")).toBeInTheDocument();
  });
});
