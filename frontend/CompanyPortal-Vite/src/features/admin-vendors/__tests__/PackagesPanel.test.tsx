/**
 * fe-coverage C3: branch special-price list ("harga khusus cabang"). Data hooks and the
 * pending-change lookups are mocked; the form dialog is stubbed (own tests in
 * PackageFormDialogs.test.tsx). Dates are far past/future so the status never depends
 * on today's date.
 */

import { useToastStore } from "@/lib/hooks/useToast";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

const m = vi.hoisted(() => ({
  query: {} as Record<string, unknown>,
  listArgs: [] as unknown[],
  disable: vi.fn(),
  pendingIds: new Set<number>(),
  pendingCreates: [] as { id: number; payload: Record<string, unknown> }[],
}));
vi.mock("../hooks", () => ({
  useVendorPackages: (...args: unknown[]) => {
    m.listArgs.push(args);
    return m.query;
  },
  useDisableVendorPackage: () => ({ mutate: m.disable, isPending: false }),
}));
vi.mock("../../master-data/pending", () => ({
  usePendingEntityIds: () => m.pendingIds,
  usePendingCreates: () => m.pendingCreates,
}));
vi.mock("../components/VendorPackageFormDialog", () => ({
  VendorPackageFormDialog: ({
    open,
    pkg,
  }: { open: boolean; pkg: { package_code: string } | null }) =>
    open ? <output data-testid="form">{pkg ? `edit ${pkg.package_code}` : "create"}</output> : null,
}));

import { PackagesPanel } from "../components/PackagesPanel";

const pkg = (over: Record<string, unknown>) => ({
  id: 1,
  vendor_branch_id: 3,
  package_code: "PKG-A",
  machine_group: "ATM",
  price_class: "REGULAR",
  tier_min: 1,
  tier_max: 5,
  atm_id: null,
  currency: "IDR",
  base_price: "1500000.50",
  sla_note: null,
  effective_start_date: "2026-01-01",
  effective_end_date: null,
  ...over,
});

const OPEN = pkg({});
const ENDED = pkg({
  id: 2,
  package_code: "PKG-OLD",
  machine_group: "CDM_CRM",
  price_class: "VIP_INDUSTRI",
  tier_max: null,
  base_price: null,
  effective_start_date: "2020-01-01",
  effective_end_date: "2020-12-31",
});
const FUTURE_END = pkg({ id: 3, package_code: "PKG-B", effective_end_date: "2999-12-31" });

beforeEach(() => {
  m.query = {
    isLoading: false,
    isError: false,
    data: { packages: [OPEN, ENDED, FUTURE_END], total: 3 },
  };
  m.listArgs = [];
  m.disable.mockReset();
  m.pendingIds = new Set();
  m.pendingCreates = [];
  useToastStore.setState({ toasts: [] });
});

const rowOf = (code: string) => screen.getByText(code).closest("tr") as HTMLElement;

describe("PackagesPanel", () => {
  it("loads this branch's rows (all statuses, first page)", () => {
    render(<PackagesPanel vendorId={42} branchId={3} />);

    expect(m.listArgs[0]).toEqual([
      42,
      { page: 1, page_size: expect.any(Number), status: "all", branch_id: 3 },
    ]);
  });

  it("shows tier, money as an IDR decimal string, and the period status", () => {
    render(<PackagesPanel vendorId={42} branchId={3} />);

    const open = within(rowOf("PKG-A"));
    expect(open.getByText("ATM · Regular")).toBeInTheDocument();
    expect(open.getByText("1-5")).toBeInTheDocument();
    expect(open.getByText("IDR 1500000.50")).toBeInTheDocument();
    expect(open.getByText("Terbuka")).toBeInTheDocument();
    expect(open.getByText("Berlaku")).toBeInTheDocument();

    const ended = within(rowOf("PKG-OLD"));
    expect(ended.getByText("CDM/CRM · VIP/Industri")).toBeInTheDocument();
    expect(ended.getByText("1+")).toBeInTheDocument();
    expect(ended.getByText("—")).toBeInTheDocument();
    expect(ended.getByText("Berakhir")).toBeInTheDocument();
    expect(ended.getByRole("button", { name: "Ubah" })).toBeDisabled();
    expect(ended.queryByRole("button", { name: "Akhiri" })).not.toBeInTheDocument();

    expect(within(rowOf("PKG-B")).getByText("Berlaku")).toBeInTheDocument();
  });

  it("a row with a pending change is badged and locked", () => {
    m.pendingIds = new Set([1]);
    render(<PackagesPanel vendorId={42} branchId={3} />);

    const row = within(rowOf("PKG-A"));
    expect(row.getByText("Menunggu approval")).toBeInTheDocument();
    expect(row.getByRole("button", { name: "Ubah" })).toBeDisabled();
    expect(row.getByRole("button", { name: "Akhiri" })).toBeDisabled();
  });

  it("lists pending creates for this branch only", () => {
    m.pendingCreates = [
      {
        id: 90,
        payload: {
          vendor_branch_id: 3,
          package_code: "PKG-NEW",
          machine_group: "ATM",
          price_class: "REGULAR",
        },
      },
      {
        id: 91,
        payload: {
          vendor_branch_id: 8,
          package_code: "PKG-OTHER",
          machine_group: "ATM",
          price_class: "REGULAR",
        },
      },
    ];
    render(<PackagesPanel vendorId={42} branchId={3} />);

    const list = screen.getByRole("region", { name: "Pengajuan baru menunggu approval" });
    expect(list).toHaveTextContent("PKG-NEW · ATM/REGULAR");
    expect(list).not.toHaveTextContent("PKG-OTHER");
  });

  it("ending a price asks first, submits it for approval and toasts the request number", async () => {
    m.disable.mockImplementation((_id, opts: { onSuccess: (r: unknown) => void }) =>
      opts.onSuccess({ change_request_id: 501, status: "pending" }),
    );
    const user = userEvent.setup();
    render(<PackagesPanel vendorId={42} branchId={3} />);

    await user.click(within(rowOf("PKG-A")).getByRole("button", { name: "Akhiri" }));
    const dialog = screen.getByRole("dialog");
    expect(dialog).toHaveTextContent('Akhiri harga khusus paket "PKG-A" per hari ini?');
    await user.click(within(dialog).getByRole("button", { name: "Akhiri" }));

    expect(m.disable).toHaveBeenCalledWith(1, expect.anything());
    expect(useToastStore.getState().toasts[0]).toMatchObject({
      type: "success",
      message: "Perubahan diajukan dan menunggu persetujuan (#501)",
    });
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("a failed end shows the server message", async () => {
    m.disable.mockImplementation((_id, opts: { onError: (e: Error) => void }) =>
      opts.onError(new Error("Masih ada perubahan pending")),
    );
    const user = userEvent.setup();
    render(<PackagesPanel vendorId={42} branchId={3} />);

    await user.click(within(rowOf("PKG-A")).getByRole("button", { name: "Akhiri" }));
    await user.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Akhiri" }));

    expect(useToastStore.getState().toasts[0]).toMatchObject({
      type: "error",
      message: "Masih ada perubahan pending",
    });
  });

  it("opens the form for a new price and for editing an open row", async () => {
    const user = userEvent.setup();
    render(<PackagesPanel vendorId={42} branchId={3} />);

    await user.click(screen.getByRole("button", { name: "Tambah Harga Khusus" }));
    expect(screen.getByTestId("form")).toHaveTextContent("create");

    await user.click(within(rowOf("PKG-B")).getByRole("button", { name: "Ubah" }));
    expect(screen.getByTestId("form")).toHaveTextContent("edit PKG-B");
  });

  it("shows loading, error and empty states", () => {
    m.query = { isLoading: true, isError: false, data: undefined };
    const { rerender } = render(<PackagesPanel vendorId={42} branchId={3} />);
    expect(screen.getByText("Memuat…")).toBeInTheDocument();

    m.query = { isLoading: false, isError: true, data: undefined };
    rerender(<PackagesPanel vendorId={42} branchId={3} />);
    expect(screen.getByRole("alert")).toHaveTextContent("Gagal memuat data paket cabang");

    m.query = { isLoading: false, isError: false, data: { packages: [], total: 0 } };
    rerender(<PackagesPanel vendorId={42} branchId={3} />);
    expect(screen.getByText("Cabang ini belum punya harga khusus")).toBeInTheDocument();
  });
});
