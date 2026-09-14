import type { AuthUser } from "@/lib/auth";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { Header } from "./Header";

const mockUser: AuthUser = {
  id: 1,
  username: "budi.santoso",
  fullName: "Budi Santoso",
  email: "budi@cimb.com",
  role: "ADMIN",
  isKaryawan: true,
  vendorId: null,
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

  it("displays email inside the user dropdown", () => {
    renderHeader();
    fireEvent.click(screen.getByTestId("header-user-name"));
    expect(screen.getByTestId("header-email")).toHaveTextContent(mockUser.email);
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
