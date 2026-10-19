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
    vault: { vault_code: string } | null;
  }) =>
    open ? (
      <output data-testid="form">{vault ? `edit ${vault.vault_code}` : "create"}</output>
    ) : null,
}));

import { VaultsPanel } from "../components/VaultsPanel";

const ACTIVE = {
  id: 1,
  vault_code: "VLT-A",
  category: "CASH",
  max_capacity_amount: "5000000000.00",
  operating_hours: "08-17",
  is_active: true,
};
const INACTIVE = {
  id: 2,
  vault_code: "VLT-B",
  category: "ATM",
  max_capacity_amount: null,
  operating_hours: null,
  is_active: false,
};

const rowOf = (code: string) => screen.getByText(code).closest("tr") as HTMLElement;

beforeEach(() => {
  m.query = { isLoading: false, isError: false, data: { vaults: [ACTIVE, INACTIVE], total: 2 } };
  m.disable.mockReset();
  m.enable.mockReset();
  m.pendingIds = new Set();
  m.pendingCreates = [];
  useToastStore.setState({ toasts: [] });
});

describe("VaultsPanel", () => {
  it("lists vaults with category, capacity, hours ('—' when missing) and a status label", () => {
    render(<VaultsPanel vendorId={42} branchId={3} />);

    const a = within(rowOf("VLT-A"));
    expect(a.getByText("CASH")).toBeInTheDocument();
    expect(a.getByText("5000000000.00")).toBeInTheDocument();
    expect(a.getByText("Aktif")).toBeInTheDocument();
    const b = within(rowOf("VLT-B"));
    expect(b.getAllByText("—")).toHaveLength(2);
    expect(b.getByText("Nonaktif")).toBeInTheDocument();
  });

  it("lists only this branch's pending vault creates and locks a pending row", () => {
    m.pendingIds = new Set([2]);
    m.pendingCreates = [
      { id: 90, payload: { vendor_branch_id: 3, vault_code: "VLT-NEW" } },
      { id: 91, payload: { vendor_branch_id: 8, vault_code: "VLT-OTHER" } },
    ];
    render(<VaultsPanel vendorId={42} branchId={3} />);

    const list = screen.getByRole("region", { name: "Pengajuan baru menunggu approval" });
    expect(list).toHaveTextContent("VLT-NEW");
    expect(list).not.toHaveTextContent("VLT-OTHER");
    const b = within(rowOf("VLT-B"));
    expect(b.getByText("Menunggu approval")).toBeInTheDocument();
    expect(b.getByRole("button", { name: "Aktifkan" })).toBeDisabled();
  });

  it("disable → confirmation → submitted for approval", async () => {
    m.disable.mockImplementation((_id, opts: { onSuccess: (r: unknown) => void }) =>
      opts.onSuccess({ change_request_id: 802 }),
    );
    const user = userEvent.setup();
    render(<VaultsPanel vendorId={42} branchId={3} />);

    await user.click(within(rowOf("VLT-A")).getByRole("button", { name: "Nonaktifkan" }));
    const dialog = screen.getByRole("dialog");
    expect(dialog).toHaveTextContent('Nonaktifkan vault "VLT-A"?');
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

    await user.click(within(rowOf("VLT-B")).getByRole("button", { name: "Aktifkan" }));
    const dialog = screen.getByRole("dialog");
    expect(dialog).toHaveTextContent('Aktifkan kembali vault "VLT-B"?');
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
    await user.click(within(rowOf("VLT-A")).getByRole("button", { name: "Ubah" }));
    expect(screen.getByTestId("form")).toHaveTextContent("edit VLT-A");
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
