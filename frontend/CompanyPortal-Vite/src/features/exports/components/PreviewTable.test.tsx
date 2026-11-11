import { render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { type MasterDataset, findMasterDataset } from "../datasets";
import type { ExportPreview } from "../types";
import { PreviewTable } from "./PreviewTable";

const dataset = findMasterDataset("vendors") as MasterDataset;

const COLUMNS = ["id", "name", "pks_status", "pks_validity_months", "pks_remaining_days"];

function renderTable(rows: string[][]) {
  const data = { columns: COLUMNS, rows } as unknown as ExportPreview;
  return render(
    <PreviewTable
      dataset={dataset}
      data={data}
      isLoading={false}
      isFetching={false}
      isError={false}
      onRetry={vi.fn()}
      pageNumber={1}
      canPrev={false}
      canNext={false}
      onPrev={vi.fn()}
      onNext={vi.fn()}
    />,
  );
}

const cellsOf = (rowIndex: number) =>
  within(screen.getAllByRole("row")[rowIndex + 1]).getAllByRole("cell");

describe("PreviewTable pks_status", () => {
  it.each(["Aktif", "Need Renewal", "Expired", "Terminated", "Belum diisi"])(
    "label %s tampil sebagai badge dengan ikon",
    (label) => {
      renderTable([["1", "PT A", label, "12", "30"]]);
      const cell = cellsOf(0)[2];
      expect(within(cell).getByText(label)).toBeInTheDocument();
      expect(cell.querySelector("svg")).not.toBeNull();
    },
  );

  it("sel kosong tetap kosong", () => {
    renderTable([["1", "PT A", "", "", ""]]);
    const cell = cellsOf(0)[2];
    expect(cell).toBeEmptyDOMElement();
  });

  it("label tak dikenal tampil sebagai teks tanpa badge", () => {
    renderTable([["1", "PT A", "Entah", "12", "30"]]);
    const cell = cellsOf(0)[2];
    expect(cell).toHaveTextContent("Entah");
    expect(cell.querySelector("svg")).toBeNull();
  });

  it("dua kolom angka PKS rata kanan, kolom teks tidak", () => {
    renderTable([["1", "PT A", "Aktif", "12", "30"]]);
    const cells = cellsOf(0);
    expect(cells[3]).toHaveClass("text-right");
    expect(cells[4]).toHaveClass("text-right");
    expect(cells[1]).not.toHaveClass("text-right");
    expect(cells[2]).not.toHaveClass("text-right");
  });
});
