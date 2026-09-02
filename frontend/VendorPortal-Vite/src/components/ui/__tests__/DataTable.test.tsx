import { DataTable } from "@/components/ui/DataTable";
import type { ColumnDef } from "@tanstack/react-table";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

interface TestRow {
  id: string;
  name: string;
  amount: number;
}

const testColumns: ColumnDef<TestRow, unknown>[] = [
  { accessorKey: "id", header: "ID" },
  { accessorKey: "name", header: "Name" },
  { accessorKey: "amount", header: "Amount", meta: { numeric: true } },
];

const testData: TestRow[] = [
  { id: "1", name: "Alpha", amount: 300 },
  { id: "2", name: "Beta", amount: 100 },
  { id: "3", name: "Charlie", amount: 200 },
];

describe("DataTable", () => {
  it("renders data in table format", () => {
    render(<DataTable data={testData} columns={testColumns} />);

    expect(screen.getByText("Alpha")).toBeInTheDocument();
    expect(screen.getByText("Beta")).toBeInTheDocument();
    expect(screen.getByText("Charlie")).toBeInTheDocument();
    // Table headers
    expect(screen.getByText("ID")).toBeInTheDocument();
    expect(screen.getByText("Name")).toBeInTheDocument();
    expect(screen.getByText("Amount")).toBeInTheDocument();
  });

  it("clicking column header triggers sort (asc then desc)", async () => {
    const user = userEvent.setup();
    render(<DataTable data={testData} columns={testColumns} />);

    const nameHeader = screen.getByText("Name");

    // Click once — should sort ascending
    await user.click(nameHeader);
    const rows = screen.getAllByRole("row");
    // Row 0 is header, rows 1-3 are data
    expect(rows[1]).toHaveTextContent("Alpha");
    expect(rows[2]).toHaveTextContent("Beta");
    expect(rows[3]).toHaveTextContent("Charlie");

    // Click again — should sort descending
    await user.click(nameHeader);
    const rowsDesc = screen.getAllByRole("row");
    expect(rowsDesc[1]).toHaveTextContent("Charlie");
    expect(rowsDesc[2]).toHaveTextContent("Beta");
    expect(rowsDesc[3]).toHaveTextContent("Alpha");
  });

  it("shows empty message when data is empty", () => {
    render(<DataTable data={[]} columns={testColumns} emptyMessage="No records found" />);

    expect(screen.getByText("No records found")).toBeInTheDocument();
  });

  it("numeric columns get right-aligned styling", () => {
    render(<DataTable data={testData} columns={testColumns} />);

    // The amount header should have text-right class
    const amountHeader = screen.getByText("Amount").closest("th");
    expect(amountHeader).toHaveClass("text-right");

    // Amount cells should have text-right and tabular-nums
    const cells = screen.getAllByRole("cell");
    // Amount is the 3rd column (index 2, 5, 8 in flat cell list)
    const amountCell = cells[2];
    expect(amountCell).toHaveClass("text-right");
    expect(amountCell).toHaveClass("tabular-nums");
  });
});
