/**
 * fe-coverage C5: vendor PIC list (vendor-wide or per branch). Data hooks and pending-change
 * lookups mocked; the PIC form is stubbed (VaultPicFormDialogs.test.tsx covers it).
 * Paging/scope params are covered by ChildPanelsPaging.test.tsx.
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
  useVendorPics: () => m.query,
  useDisableVendorPic: () => ({ mutate: m.disable, isPending: false }),
  useEnableVendorPic: () => ({ mutate: m.enable, isPending: false }),
}));
vi.mock("../../master-data/pending", () => ({
  usePendingEntityIds: () => m.pendingIds,
  usePendingCreates: () => m.pendingCreates,
}));
vi.mock("../components/VendorPicFormDialog", () => ({
  VendorPicFormDialog: ({ open, pic }: { open: boolean; pic: { name: string } | null }) =>
    open ? <output data-testid="form">{pic ? `edit ${pic.name}` : "create"}</output> : null,
}));

import { PicsPanel } from "../components/PicsPanel";

const ACTIVE = {
  id: 1,
  name: "Andi",
  position: "Supervisor",
  phone: null,
  email: "andi@vendor.co.id",
  is_notification_recipient: true,
  is_active: true,
};
const INACTIVE = {
  id: 2,
  name: "Sari",
  position: null,
  phone: "0812",
  email: null,
  is_notification_recipient: false,
  is_active: false,
};

const rowOf = (name: string) => screen.getByText(name).closest("tr") as HTMLElement;

beforeEach(() => {
  m.query = {
    isLoading: false,
    isError: false,
    data: { pics: [ACTIVE, INACTIVE], total: 2, warnings: [] },
  };
  m.disable.mockReset();
  m.enable.mockReset();
  m.pendingIds = new Set();
  m.pendingCreates = [];
  useToastStore.setState({ toasts: [] });
});

describe("PicsPanel", () => {
  it("lists PICs with '—' for missing fields, a recipient badge and a status label", () => {
    render(<PicsPanel vendorId={42} branchId={3} />);

    const andi = within(rowOf("Andi"));
    expect(andi.getByText("Supervisor")).toBeInTheDocument();
    expect(andi.getByText("Penerima notifikasi")).toBeInTheDocument();
    expect(andi.getByText("Aktif")).toBeInTheDocument();
    const sari = within(rowOf("Sari"));
    expect(sari.getAllByText("—").length).toBeGreaterThanOrEqual(2);
    expect(sari.getByText("Nonaktif")).toBeInTheDocument();
  });

  it("shows server warnings (e.g. no notification recipient)", () => {
    m.query = {
      isLoading: false,
      isError: false,
      data: { pics: [], total: 0, warnings: ["Belum ada penerima notifikasi"] },
    };
    render(<PicsPanel vendorId={42} branchId={null} />);

    expect(screen.getByText("Belum ada penerima notifikasi")).toBeInTheDocument();
    expect(screen.getByText("Belum ada PIC vendor-wide")).toBeInTheDocument();
  });

  it("branch scope: empty message and only this branch's pending creates", () => {
    m.query = { isLoading: false, isError: false, data: { pics: [], total: 0 } };
    m.pendingCreates = [
      { id: 90, payload: { vendor_branch_id: 3, name: "PIC Baru" } },
      { id: 91, payload: { vendor_branch_id: 8, name: "PIC Lain" } },
    ];
    render(<PicsPanel vendorId={42} branchId={3} />);

    expect(screen.getByText("Cabang ini belum punya PIC")).toBeInTheDocument();
    const list = screen.getByRole("region", { name: "Pengajuan baru menunggu approval" });
    expect(list).toHaveTextContent("PIC Baru");
    expect(list).not.toHaveTextContent("PIC Lain");
  });

  it("vendor-wide scope: only this vendor's pending creates without a branch", () => {
    m.pendingCreates = [
      { id: 92, payload: { vendor_id: 42, vendor_branch_id: null, name: "PIC Pusat" } },
      { id: 93, payload: { vendor_id: 42, vendor_branch_id: 3, name: "PIC Cabang" } },
      { id: 94, payload: { vendor_id: 7, vendor_branch_id: null, name: "PIC Vendor Lain" } },
    ];
    render(<PicsPanel vendorId={42} branchId={null} />);

    const list = screen.getByRole("region", { name: "Pengajuan baru menunggu approval" });
    expect(list).toHaveTextContent("PIC Pusat");
    expect(list).not.toHaveTextContent("PIC Cabang");
    expect(list).not.toHaveTextContent("PIC Vendor Lain");
  });

  it("a PIC with a pending change is badged and its actions are locked", () => {
    m.pendingIds = new Set([1]);
    render(<PicsPanel vendorId={42} branchId={3} />);

    const row = within(rowOf("Andi"));
    expect(row.getByText("Menunggu approval")).toBeInTheDocument();
    expect(row.getByRole("button", { name: "Ubah" })).toBeDisabled();
    expect(row.getByRole("button", { name: "Nonaktifkan" })).toBeDisabled();
  });

  it("disabling submits for approval and toasts the request number", async () => {
    m.disable.mockImplementation((_id, opts: { onSuccess: (r: unknown) => void }) =>
      opts.onSuccess({ change_request_id: 801 }),
    );
    const user = userEvent.setup();
    render(<PicsPanel vendorId={42} branchId={3} />);

    await user.click(within(rowOf("Andi")).getByRole("button", { name: "Nonaktifkan" }));
    const dialog = screen.getByRole("dialog");
    expect(dialog).toHaveTextContent('Nonaktifkan PIC "Andi"?');
    await user.click(within(dialog).getByRole("button", { name: "Nonaktifkan" }));

    expect(m.disable).toHaveBeenCalledWith(1, expect.anything());
    expect(useToastStore.getState().toasts[0]).toMatchObject({
      type: "success",
      message: "Perubahan diajukan dan menunggu persetujuan (#801)",
    });
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("enabling an inactive PIC; a failure shows the server message", async () => {
    m.enable.mockImplementation((_id, opts: { onError: (e: Error) => void }) =>
      opts.onError(new Error("Masih ada perubahan pending")),
    );
    const user = userEvent.setup();
    render(<PicsPanel vendorId={42} branchId={3} />);

    await user.click(within(rowOf("Sari")).getByRole("button", { name: "Aktifkan" }));
    const dialog = screen.getByRole("dialog");
    expect(dialog).toHaveTextContent('Aktifkan kembali PIC "Sari"?');
    await user.click(within(dialog).getByRole("button", { name: "Aktifkan" }));

    expect(m.enable).toHaveBeenCalledWith(2, expect.anything());
    expect(useToastStore.getState().toasts[0]).toMatchObject({
      type: "error",
      message: "Masih ada perubahan pending",
    });
  });

  it("opens the form for a new PIC and for editing one", async () => {
    const user = userEvent.setup();
    render(<PicsPanel vendorId={42} branchId={3} />);

    await user.click(screen.getByRole("button", { name: "Tambah PIC" }));
    expect(screen.getByTestId("form")).toHaveTextContent("create");

    await user.click(within(rowOf("Sari")).getByRole("button", { name: "Ubah" }));
    expect(screen.getByTestId("form")).toHaveTextContent("edit Sari");
  });

  it("shows loading and error states", () => {
    m.query = { isLoading: true, isError: false, data: undefined };
    const { rerender } = render(<PicsPanel vendorId={42} branchId={3} />);
    expect(screen.getByText("Memuat…")).toBeInTheDocument();

    m.query = { isLoading: false, isError: true, data: undefined };
    rerender(<PicsPanel vendorId={42} branchId={3} />);
    expect(screen.getByRole("alert")).toHaveTextContent("Gagal memuat data PIC");
  });
});
