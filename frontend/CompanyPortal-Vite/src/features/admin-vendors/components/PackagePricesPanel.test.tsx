import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { AdminVendorPackagePrice } from "../types";
import { PackagePricesPanel } from "./PackagePricesPanel";

function daysFromToday(offset: number): string {
  const d = new Date();
  d.setDate(d.getDate() + offset);
  return d.toISOString().slice(0, 10);
}

function price(overrides: Partial<AdminVendorPackagePrice> = {}): AdminVendorPackagePrice {
  return {
    id: 1,
    vendor_id: 1,
    package: "PAKET 3",
    package_code: "PKG3_ABA_001",
    machine_group: "ATM",
    price_class: "REGULAR",
    tier_min: 1,
    tier_max: 50,
    base_price: "1000000.00",
    vendor_branch_id: null,
    atm_id: null,
    sla_note: null,
    currency: "IDR",
    effective_start_date: daysFromToday(-30),
    effective_end_date: null,
    ...overrides,
  };
}

const berlaku = price({
  id: 1,
  package: "PAKET 3",
  package_code: "PKG3_ABA_001",
  machine_group: "ATM",
});
const dijadwalkan = price({
  id: 2,
  package: "PAKET 5",
  package_code: "PKG5_ABA_002",
  machine_group: "CDM_CRM",
  effective_start_date: daysFromToday(10),
});
const berakhir = price({
  id: 3,
  package: "PAKET 7",
  package_code: "PKG7_ABA_003",
  effective_start_date: daysFromToday(-60),
  effective_end_date: daysFromToday(-1),
});

const disableMutate = vi.fn();
const useVendorPackagePricesMock = vi.fn();
const usePendingEntityIdsMock = vi.fn();
const toastMock = vi.fn();

vi.mock("../hooks", () => ({
  useVendorPackagePrices: (...args: unknown[]) => useVendorPackagePricesMock(...args),
  useDisableVendorPackagePrice: () => ({ mutate: disableMutate, isPending: false }),
  useCreateVendorPackagePrice: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateVendorPackagePrice: () => ({ mutate: vi.fn(), isPending: false }),
}));

vi.mock("../../master-data/pending", () => ({
  usePendingEntityIds: (...args: unknown[]) => usePendingEntityIdsMock(...args),
}));

vi.mock("@/lib/hooks/useToast", () => ({
  useToast: () => ({ toast: toastMock }),
}));

vi.mock("./VendorPackagePriceFormDialog", () => ({
  VendorPackagePriceFormDialog: ({ open }: { open: boolean }) =>
    open ? <div data-testid="price-form-dialog" /> : null,
}));

function setPrices(prices: AdminVendorPackagePrice[]) {
  useVendorPackagePricesMock.mockReturnValue({
    isLoading: false,
    isError: false,
    data: { package_prices: prices, page: 1, page_size: 100, total: prices.length },
  });
}

beforeEach(() => {
  vi.clearAllMocks();
  usePendingEntityIdsMock.mockReturnValue(new Set<number>());
});

describe("PackagePricesPanel", () => {
  it("renders three-state status badges with icon and label", () => {
    setPrices([berlaku, dijadwalkan, berakhir]);
    render(<PackagePricesPanel vendorId={1} />);
    const table = within(screen.getByRole("table"));

    const berlakuBadge = table.getByText("Berlaku");
    expect(berlakuBadge.parentElement?.querySelector("svg")).toBeTruthy();

    const dijadwalkanBadge = table.getByText("Dijadwalkan");
    expect(dijadwalkanBadge.parentElement?.querySelector("svg")).toBeTruthy();

    const berakhirBadge = table.getByText("Berakhir");
    expect(berakhirBadge.parentElement?.querySelector("svg")).toBeTruthy();
  });

  it("gates actions by status: Berlaku gets Ubah+Akhiri, Dijadwalkan gets Ubah only, Berakhir gets neither", () => {
    setPrices([berlaku, dijadwalkan, berakhir]);
    render(<PackagePricesPanel vendorId={1} />);

    function rowFor(packageCode: string): HTMLElement {
      const cell = screen.getByText(packageCode);
      const row = cell.closest("tr");
      if (!row) throw new Error(`row not found for ${packageCode}`);
      return row;
    }
    const berlakuRow = rowFor(berlaku.package_code);
    const dijadwalkanRow = rowFor(dijadwalkan.package_code);
    const berakhirRow = rowFor(berakhir.package_code);

    expect(within(berlakuRow).getByRole("button", { name: "Ubah" })).toBeTruthy();
    expect(within(berlakuRow).getByRole("button", { name: "Akhiri" })).toBeTruthy();

    expect(within(dijadwalkanRow).getByRole("button", { name: "Ubah" })).toBeTruthy();
    expect(within(dijadwalkanRow).queryByRole("button", { name: "Akhiri" })).toBeNull();

    expect(within(berakhirRow).queryByRole("button", { name: "Ubah" })).toBeNull();
    expect(within(berakhirRow).queryByRole("button", { name: "Akhiri" })).toBeNull();
    expect(within(berakhirRow).getByText(`Diakhiri ${berakhir.effective_end_date}`)).toBeTruthy();
  });

  it("disables Ubah and Akhiri for a row with a pending approval", () => {
    setPrices([berlaku]);
    usePendingEntityIdsMock.mockReturnValue(new Set([berlaku.id]));
    render(<PackagePricesPanel vendorId={1} />);

    expect(screen.getByRole("button", { name: "Ubah" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Akhiri" })).toBeDisabled();
    expect(screen.getByText("Menunggu approval")).toBeTruthy();
  });

  it("filters narrow the list and update the tier count", async () => {
    const user = userEvent.setup();
    setPrices([berlaku, dijadwalkan, berakhir]);
    render(<PackagePricesPanel vendorId={1} />);

    expect(screen.getByText("3 tingkat")).toBeTruthy();

    await user.selectOptions(screen.getByLabelText("Status"), "Berlaku");

    expect(screen.getByText("1 tingkat")).toBeTruthy();
    expect(screen.getByText("PKG3_ABA_001")).toBeTruthy();
    expect(within(screen.getByRole("table")).queryByText("Dijadwalkan")).toBeNull();
  });

  it("shows every price when filters are reset to all", async () => {
    const user = userEvent.setup();
    setPrices([berlaku, dijadwalkan, berakhir]);
    render(<PackagePricesPanel vendorId={1} />);

    await user.selectOptions(screen.getByLabelText("Status"), "Berlaku");
    await user.selectOptions(screen.getByLabelText("Status"), "");

    expect(screen.getByText("3 tingkat")).toBeTruthy();
  });

  it("shows a distinct no-match message when filters exclude every row", async () => {
    const user = userEvent.setup();
    setPrices([berlaku]);
    render(<PackagePricesPanel vendorId={1} />);

    await user.selectOptions(screen.getByLabelText("Status"), "Berakhir");

    expect(screen.getByText("Tidak ada tingkat yang cocok dengan filter.")).toBeTruthy();
    expect(screen.queryByText("Vendor ini belum punya harga paket")).toBeNull();
  });

  it("shows the vendor-has-no-prices empty state distinctly when the list itself is empty", () => {
    setPrices([]);
    render(<PackagePricesPanel vendorId={1} />);

    expect(screen.getByText("Vendor ini belum punya harga paket")).toBeTruthy();
    expect(screen.queryByText("Tidak ada tingkat yang cocok dengan filter.")).toBeNull();
  });

  it("shows the staged-for-approval message, not an applied-change message, on a successful Akhiri", async () => {
    const user = userEvent.setup();
    setPrices([berlaku]);
    disableMutate.mockImplementation((_id, { onSuccess }) => {
      onSuccess({ change_request_id: 42 });
    });
    render(<PackagePricesPanel vendorId={1} />);

    await user.click(screen.getByRole("button", { name: "Akhiri" }));
    const dialog = within(screen.getByRole("dialog"));
    await user.click(dialog.getByRole("button", { name: "Akhiri" }));

    expect(toastMock).toHaveBeenCalledWith(
      expect.objectContaining({
        type: "success",
        message: expect.stringContaining("menunggu persetujuan"),
      }),
    );
    expect(toastMock).not.toHaveBeenCalledWith(
      expect.objectContaining({ message: expect.stringContaining("disimpan") }),
    );
  });

  it("surfaces an error when the Akhiri mutation fails", async () => {
    const user = userEvent.setup();
    setPrices([berlaku]);
    disableMutate.mockImplementation((_id, { onError }) => {
      onError({ message: "Gagal mengakhiri harga" });
    });
    render(<PackagePricesPanel vendorId={1} />);

    await user.click(screen.getByRole("button", { name: "Akhiri" }));
    const dialog = within(screen.getByRole("dialog"));
    await user.click(dialog.getByRole("button", { name: "Akhiri" }));

    expect(toastMock).toHaveBeenCalledWith({ type: "error", message: "Gagal mengakhiri harga" });
  });
});
