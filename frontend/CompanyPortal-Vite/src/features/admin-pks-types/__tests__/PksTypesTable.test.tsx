import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { PksTypesTable } from "../components/PksTypesTable";
import type { PksType } from "../types";

vi.mock("@/features/master-data/components/PendingApprovalBadge", () => ({
  PendingApprovalBadge: ({ id }: { id: number | string }) => (
    <span data-testid={`pending-badge-${id}`}>Menunggu approval</span>
  ),
}));

describe("PksTypesTable", () => {
  const mockTypes: PksType[] = [
    {
      id: "1",
      code: "FLM",
      name: "First Line Maintenance",
      is_active: true,
      in_use: true,
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    },
    {
      id: "2",
      code: "CIT",
      name: "Cash in Transit",
      is_active: true,
      in_use: false,
      created_at: "2026-01-02T00:00:00Z",
      updated_at: "2026-01-02T00:00:00Z",
    },
    {
      id: "3",
      code: "SLM",
      name: "Second Line Maintenance",
      is_active: false,
      in_use: false,
      created_at: "2026-01-03T00:00:00Z",
      updated_at: "2026-01-03T00:00:00Z",
    },
  ];

  it("renders table with code, name, status, and in_use columns", () => {
    render(
      <PksTypesTable
        types={mockTypes}
        pendingIds={new Set()}
        onEdit={vi.fn()}
        onDisable={vi.fn()}
        onEnable={vi.fn()}
      />,
    );

    expect(screen.getByText("FLM")).toBeInTheDocument();
    expect(screen.getByText("First Line Maintenance")).toBeInTheDocument();
    expect(screen.getByText("CIT")).toBeInTheDocument();
    expect(screen.getByText("Cash in Transit")).toBeInTheDocument();
    expect(screen.getByText("SLM")).toBeInTheDocument();

    // Status: never by color alone (text is present)
    expect(screen.getAllByText("Aktif")).toHaveLength(2);
    expect(screen.getByText("Nonaktif")).toBeInTheDocument();

    // Dipakai: Ya / Tidak badges
    expect(screen.getByText("Ya")).toBeInTheDocument();
    expect(screen.getAllByText("Tidak")).toHaveLength(2);
  });

  it("disables Nonaktifkan button when in_use is true and displays text explanation", () => {
    render(
      <PksTypesTable
        types={mockTypes}
        pendingIds={new Set()}
        onEdit={vi.fn()}
        onDisable={vi.fn()}
        onEnable={vi.fn()}
      />,
    );

    const flmRow = screen.getByText("FLM").closest("tr");
    expect(flmRow).not.toBeNull();
    // FLM has in_use = true -> button is disabled
    const flmDisableBtn = within(flmRow as HTMLElement).getByRole("button", {
      name: "Nonaktifkan",
    });
    expect(flmDisableBtn).toBeDisabled();

    // Text explanation is shown
    expect(
      within(flmRow as HTMLElement).getByText("Sedang digunakan oleh PKS vendor"),
    ).toBeInTheDocument();

    const citRow = screen.getByText("CIT").closest("tr");
    expect(citRow).not.toBeNull();
    // CIT has in_use = false -> button is enabled
    const citDisableBtn = within(citRow as HTMLElement).getByRole("button", {
      name: "Nonaktifkan",
    });
    expect(citDisableBtn).not.toBeDisabled();
  });

  it("renders Aktifkan button for inactive items", () => {
    const onEnable = vi.fn();
    render(
      <PksTypesTable
        types={mockTypes}
        pendingIds={new Set()}
        onEdit={vi.fn()}
        onDisable={vi.fn()}
        onEnable={onEnable}
      />,
    );

    const slmRow = screen.getByText("SLM").closest("tr");
    expect(slmRow).not.toBeNull();
    const enableBtn = within(slmRow as HTMLElement).getByRole("button", {
      name: "Aktifkan",
    });
    expect(enableBtn).toBeInTheDocument();
    expect(enableBtn).not.toBeDisabled();

    fireEvent.click(enableBtn);
    expect(onEnable).toHaveBeenCalledWith(mockTypes[2]);
  });

  it("calls onEdit when Ubah button is clicked", () => {
    const onEdit = vi.fn();
    render(
      <PksTypesTable
        types={mockTypes}
        pendingIds={new Set()}
        onEdit={onEdit}
        onDisable={vi.fn()}
        onEnable={vi.fn()}
      />,
    );

    const citRow = screen.getByText("CIT").closest("tr");
    expect(citRow).not.toBeNull();
    const editBtn = within(citRow as HTMLElement).getByRole("button", {
      name: "Ubah",
    });
    fireEvent.click(editBtn);
    expect(onEdit).toHaveBeenCalledWith(mockTypes[1]);
  });

  it("calls onDisable when Nonaktifkan is clicked on an item not in use", () => {
    const onDisable = vi.fn();
    render(
      <PksTypesTable
        types={mockTypes}
        pendingIds={new Set()}
        onEdit={vi.fn()}
        onDisable={onDisable}
        onEnable={vi.fn()}
      />,
    );

    const citRow = screen.getByText("CIT").closest("tr");
    expect(citRow).not.toBeNull();
    const disableBtn = within(citRow as HTMLElement).getByRole("button", {
      name: "Nonaktifkan",
    });
    fireEvent.click(disableBtn);
    expect(onDisable).toHaveBeenCalledWith(mockTypes[1]);
  });

  it("shows PendingApprovalBadge and disables row actions when item is pending", () => {
    render(
      <PksTypesTable
        types={mockTypes}
        pendingIds={new Set(["2"])} // CIT is pending
        onEdit={vi.fn()}
        onDisable={vi.fn()}
        onEnable={vi.fn()}
      />,
    );

    expect(screen.getByText("Menunggu approval")).toBeInTheDocument();

    const citRow = screen.getByText("CIT").closest("tr");
    expect(citRow).not.toBeNull();
    const editBtn = within(citRow as HTMLElement).getByRole("button", {
      name: "Ubah",
    });
    const disableBtn = within(citRow as HTMLElement).getByRole("button", {
      name: "Nonaktifkan",
    });
    expect(editBtn).toBeDisabled();
    expect(disableBtn).toBeDisabled();
  });

  it("shows empty message when list is empty", () => {
    render(
      <PksTypesTable
        types={[]}
        pendingIds={new Set()}
        onEdit={vi.fn()}
        onDisable={vi.fn()}
        onEnable={vi.fn()}
        emptyMessage="Tidak ada type PKS ditemukan"
      />,
    );

    expect(screen.getByText("Tidak ada type PKS ditemukan")).toBeInTheDocument();
  });
});
