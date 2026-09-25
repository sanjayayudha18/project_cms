import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ATMsPanel } from "./ATMsPanel";
import type { ManagedATM } from "../types";

const activeATM: ManagedATM = {
  atm_id: 4021,
  terminal_id: "ATM00412",
  location_name: "Kantor Cabang Menteng",
  location_city_or_regency: "Jakarta Pusat",
  priority_class: "VIP",
  is_active: true,
  package_code: "PKG-JKT-01",
};

const inactiveATM: ManagedATM = {
  atm_id: 4022,
  terminal_id: "ATM00413",
  location_name: null,
  location_city_or_regency: null,
  priority_class: null,
  is_active: false,
  package_code: "PKG-JKT-02",
};

const useBranchATMsMock = vi.fn();
vi.mock("../hooks", () => ({
  useBranchATMs: (...args: unknown[]) => useBranchATMsMock(...args),
}));

describe("ATMsPanel", () => {
  it("renders rows on success with terminal_id in tabular figures", () => {
    useBranchATMsMock.mockReturnValue({
      isLoading: false,
      isError: false,
      data: { atms: [activeATM], page: 1, page_size: 100, total: 1 },
    });
    render(<ATMsPanel vendorId={3} branchId={9} />);

    const terminalCell = screen.getByText("ATM00412");
    expect(terminalCell.className).toContain("tabular-nums");
    expect(screen.getByText("Kantor Cabang Menteng · Jakarta Pusat")).toBeTruthy();
    expect(screen.getByText("PKG-JKT-01")).toBeTruthy();
  });

  it("shows a success badge with icon and label for an active ATM, danger for an inactive one", () => {
    useBranchATMsMock.mockReturnValue({
      isLoading: false,
      isError: false,
      data: { atms: [activeATM, inactiveATM], page: 1, page_size: 100, total: 2 },
    });
    render(<ATMsPanel vendorId={3} branchId={9} />);

    const activeLabel = screen.getByText("Aktif");
    expect(activeLabel.parentElement?.querySelector("svg")).toBeTruthy();

    const inactiveLabel = screen.getByText("Nonaktif");
    expect(inactiveLabel.parentElement?.querySelector("svg")).toBeTruthy();
  });

  it("renders a placeholder marker, not an empty cell, when location is null", () => {
    useBranchATMsMock.mockReturnValue({
      isLoading: false,
      isError: false,
      data: { atms: [inactiveATM], page: 1, page_size: 100, total: 1 },
    });
    render(<ATMsPanel vendorId={3} branchId={9} />);

    // inactiveATM has both location and priority_class null -> two "—" cells.
    expect(screen.getAllByText("—")).toHaveLength(2);
  });

  it("shows a loading indicator while the request is in progress", () => {
    useBranchATMsMock.mockReturnValue({ isLoading: true, isError: false, data: undefined });
    render(<ATMsPanel vendorId={3} branchId={9} />);
    expect(screen.getByText("Memuat…")).toBeTruthy();
  });

  it("shows an empty-state message when the branch has no managed ATMs", () => {
    useBranchATMsMock.mockReturnValue({
      isLoading: false,
      isError: false,
      data: { atms: [], page: 1, page_size: 100, total: 0 },
    });
    render(<ATMsPanel vendorId={3} branchId={9} />);
    expect(screen.getByText("Cabang ini belum mengelola ATM")).toBeTruthy();
  });

  it("shows an error message when the request fails", () => {
    useBranchATMsMock.mockReturnValue({ isLoading: false, isError: true, data: undefined });
    render(<ATMsPanel vendorId={3} branchId={9} />);
    expect(screen.getByRole("alert")).toHaveTextContent("Gagal memuat data ATM cabang");
  });
});
