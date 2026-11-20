/**
 * fe-coverage C5: a branch's vault list. Data hooks and pending-change lookups mocked; the
 * vault form is stubbed (VaultPicFormDialogs.test.tsx covers it).
 */

import { useToastStore } from "@/lib/hooks/useToast";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

const m = vi.hoisted(() => ({
  query: {} as Record<string, unknown>,
  disable: vi.fn(),
  enable: vi.fn(),
  pendingIds: new Set<number>(),
  pendingCreates: [] as { id: number; payload: Record<string, unknown> }[],
}));
vi.mock("../hooks", () => ({
  useVendorVaults: () => m.query,
  useDisableVendorVault: () => ({ mutate: m.disable, isPending: false }),
  useEnableVendorVault: () => ({ mutate: m.enable, isPending: false }),
}));
vi.mock("../../master-data/pending", () => ({
  usePendingEntityIds: () => m.pendingIds,
  usePendingCreates: () => m.pendingCreates,
}));
vi.mock("../components/VendorVaultFormDialog", () => ({
  VendorVaultFormDialog: ({
    open,
    vault,
  }: {
    open: boolean;
    vault: { vault_name?: string; vault_code: string } | null;
  }) =>
    open ? (
      <output data-testid="form">
        {vault ? `edit ${vault.vault_name || vault.vault_code}` : "create"}
      </output>
    ) : null,
}));

import { VaultsPanel } from "../components/VaultsPanel";

const ACTIVE = {
  id: 1,
  vault_code: "ABA_Bali_V001",
  vault_name: "ABA_Bali_V001",
  region: "Bali",
  category: "CASH",
  max_balance: "5000000000.00",
  cis_limit_amount: "2500000000.00",
  escrow_cash: "001234567890",
  escrow_atm: "998877665544",
  operating_hours: "08-17",
  is_active: true,
};
const INACTIVE = {
  id: 2,
  vault_code: "ABA_Bali_V002",
  vault_name: "ABA_Bali_V002",
  region: null,
  category: "ATM",
  max_balance: null,
  cis_limit_amount: null,
  escrow_cash: null,
  escrow_atm: null,
  operating_hours: null,
  is_active: false,
};

const rowOf = (name: string) => screen.getByText(name).closest("tr") as HTMLElement;

beforeEach(() => {
  m.query = { isLoading: false, isError: false, data: { vaults: [ACTIVE, INACTIVE], total: 2 } };
  m.disable.mockReset();
  m.enable.mockReset();
  m.pendingIds = new Set();
  m.pendingCreates = [];
  useToastStore.setState({ toasts: [] });
});

describe("VaultsPanel", () => {
  it("lists vaults with vault_name, category, region, max_balance, cis_limit_amount, escrow, hours and status", () => {
    render(<VaultsPanel vendorId={42} branchId={3} />);

    const a = within(rowOf("ABA_Bali_V001"));
    expect(a.getByText("CASH")).toBeInTheDocument();
    expect(a.getByText("Bali")).toBeInTheDocument();
    expect(a.getByText("5000000000.00")).toBeInTheDocument();
    expect(a.getByText("2500000000.00")).toBeInTheDocument();
    expect(a.getByText("001234567890")).toBeInTheDocument();
    expect(a.getByText("998877665544")).toBeInTheDocument();
    expect(a.getByText("08-17")).toBeInTheDocument();
    expect(a.getByText("Aktif")).toBeInTheDocument();
    const b = within(rowOf("ABA_Bali_V002"));
    expect(b.getAllByText("—")).toHaveLength(6);
    expect(b.getByText("Nonaktif")).toBeInTheDocument();
  });

  it("lists only this branch's pending vault creates and locks a pending row", () => {
    m.pendingIds = new Set([2]);
    m.pendingCreates = [
      { id: 90, payload: { vendor_branch_id: 3, vault_name: "ABA_Bali_V003" } },
      { id: 91, payload: { vendor_branch_id: 8, vault_name: "ABA_Jakarta_V001" } },
    ];
    render(<VaultsPanel vendorId={42} branchId={3} />);

    const list = screen.getByRole("region", { name: "Pengajuan baru menunggu approval" });
    expect(list).toHaveTextContent("ABA_Bali_V003");
    expect(list).not.toHaveTextContent("ABA_Jakarta_V001");
    const b = within(rowOf("ABA_Bali_V002"));
    expect(b.getByText("Menunggu approval")).toBeInTheDocument();
    expect(b.getByRole("button", { name: "Aktifkan" })).toBeDisabled();
  });

  it("disable → confirmation → submitted for approval", async () => {
    m.disable.mockImplementation((_id, opts: { onSuccess: (r: unknown) => void }) =>
      opts.onSuccess({ change_request_id: 802 }),
    );
    const user = userEvent.setup();
    render(<VaultsPanel vendorId={42} branchId={3} />);

    await user.click(within(rowOf("ABA_Bali_V001")).getByRole("button", { name: "Nonaktifkan" }));
    const dialog = screen.getByRole("dialog");
    expect(dialog).toHaveTextContent('Nonaktifkan vault "ABA_Bali_V001"?');
    await user.click(within(dialog).getByRole("button", { name: "Nonaktifkan" }));

    expect(m.disable).toHaveBeenCalledWith(1, expect.anything());
    expect(useToastStore.getState().toasts[0]).toMatchObject({
      type: "success",
      message: "Perubahan diajukan dan menunggu persetujuan (#802)",
    });
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("enable failure shows the server message", async () => {
    m.enable.mockImplementation((_id, opts: { onError: (e: Error) => void }) =>
      opts.onError(new Error("Masih ada perubahan pending")),
    );
    const user = userEvent.setup();
    render(<VaultsPanel vendorId={42} branchId={3} />);

    await user.click(within(rowOf("ABA_Bali_V002")).getByRole("button", { name: "Aktifkan" }));
    const dialog = screen.getByRole("dialog");
    expect(dialog).toHaveTextContent('Aktifkan kembali vault "ABA_Bali_V002"?');
    await user.click(within(dialog).getByRole("button", { name: "Aktifkan" }));

    expect(useToastStore.getState().toasts[0]).toMatchObject({
      type: "error",
      message: "Masih ada perubahan pending",
    });
  });

  it("opens the form for a new vault and for editing one", async () => {
    const user = userEvent.setup();
    render(<VaultsPanel vendorId={42} branchId={3} />);

    await user.click(screen.getByRole("button", { name: "Tambah Vault" }));
    expect(screen.getByTestId("form")).toHaveTextContent("create");
    await user.click(within(rowOf("ABA_Bali_V001")).getByRole("button", { name: "Ubah" }));
    expect(screen.getByTestId("form")).toHaveTextContent("edit ABA_Bali_V001");
  });

  it("shows loading, error and empty states", () => {
    m.query = { isLoading: true, isError: false, data: undefined };
    const { rerender } = render(<VaultsPanel vendorId={42} branchId={3} />);
    expect(screen.getByText("Memuat…")).toBeInTheDocument();

    m.query = { isLoading: false, isError: true, data: undefined };
    rerender(<VaultsPanel vendorId={42} branchId={3} />);
    expect(screen.getByRole("alert")).toHaveTextContent("Gagal memuat data vault");

    m.query = { isLoading: false, isError: false, data: { vaults: [], total: 0 } };
    rerender(<VaultsPanel vendorId={42} branchId={3} />);
    expect(screen.getByText("Cabang ini belum punya vault")).toBeInTheDocument();
  });
});
