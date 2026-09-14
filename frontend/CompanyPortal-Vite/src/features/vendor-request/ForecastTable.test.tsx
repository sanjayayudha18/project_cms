import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ForecastTable } from "./ForecastTable";
import type { ForecastRow } from "./types";

const rowWithContext: ForecastRow = {
  terminal_id: "1234",
  periode_pred: "2027-01-15",
  denom: 50000,
  amount_replenish: 100000000,
  amount_refund: 0,
  dmaa_file_id: 1,
  lokasi_atm: "Test Location",
  brand: "Hyosung",
  flm_vendor: "TAG",
  flm_vendor_region: "TAG Jawa Barat",
};

const rowWithoutContext: ForecastRow = {
  ...rowWithContext,
  terminal_id: "5678",
  lokasi_atm: "",
  brand: "",
  flm_vendor: "",
  flm_vendor_region: "",
};

const noop = () => {};

function renderTable(props: Partial<React.ComponentProps<typeof ForecastTable>> = {}) {
  return render(
    <ForecastTable
      data={props.data ?? []}
      isLoading={props.isLoading ?? false}
      isError={props.isError ?? false}
      onRetry={props.onRetry ?? vi.fn()}
      sorting={props.sorting ?? []}
      onSortingChange={props.onSortingChange ?? noop}
      rowSelection={props.rowSelection ?? {}}
      onRowSelectionChange={props.onRowSelectionChange ?? noop}
      pagination={props.pagination}
      page={props.page ?? 1}
      pageSize={props.pageSize ?? 25}
      onPageChange={props.onPageChange ?? noop}
      onPageSizeChange={props.onPageSizeChange ?? noop}
    />,
  );
}

describe("ForecastTable — four new context columns (Task 4)", () => {
  it("renders headers for the four new columns", () => {
    renderTable({ data: [rowWithContext] });

    expect(screen.getByText("Lokasi ATM")).toBeInTheDocument();
    expect(screen.getByText("Brand")).toBeInTheDocument();
    expect(screen.getByText("FLM Vendor")).toBeInTheDocument();
    expect(screen.getByText("FLM Vendor Region")).toBeInTheDocument();
  });

  it("renders populated values for the four new columns", () => {
    renderTable({ data: [rowWithContext] });

    expect(screen.getByText("Test Location")).toBeInTheDocument();
    expect(screen.getByText("Hyosung")).toBeInTheDocument();
    expect(screen.getByText("TAG")).toBeInTheDocument();
    expect(screen.getByText("TAG Jawa Barat")).toBeInTheDocument();
  });

  it('renders "-" placeholder (never the literal em-dash) for empty values', () => {
    renderTable({ data: [rowWithoutContext] });

    const placeholders = screen.getAllByText("-");
    expect(placeholders).toHaveLength(4);
    for (const el of placeholders) {
      expect(el.textContent).toBe("-");
      expect(el.textContent).not.toBe("—");
    }
  });

  it("spans all 9 columns on the empty-result row", () => {
    renderTable({ data: [] });

    const emptyCell = screen.getByText("Tidak ada data forecast untuk tanggal yang dipilih");
    expect(emptyCell.closest("td")).toHaveAttribute("colspan", "9");
  });

  it("spans all 9 columns on the error row", () => {
    renderTable({ data: [], isError: true });

    const errorCell = screen.getByText("Gagal memuat data forecast");
    expect(errorCell.closest("td")).toHaveAttribute("colspan", "9");
  });

  it("spans all 9 columns on skeleton rows while loading", () => {
    const { container } = renderTable({ data: [], isLoading: true });

    const skeletonRow = container.querySelector("tbody tr");
    expect(skeletonRow?.querySelectorAll("td")).toHaveLength(9);
  });
});
