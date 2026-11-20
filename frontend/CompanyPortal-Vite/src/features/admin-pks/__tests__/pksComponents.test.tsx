import { useAuthStore } from "@/lib/auth/store";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { PksFilterBar } from "../components/PksFilterBar";
import { PksFormDialog } from "../components/PksFormDialog";
import { PksListPage } from "../components/PksListPage";
import { PksReminderDialog } from "../components/PksReminderDialog";
import { PksTable } from "../components/PksTable";
import type { PksListItem } from "../types";

const mutateReminder = vi.fn();
const usePksListMock = vi.fn();

vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => vi.fn(),
  useSearch: () => ({}),
  Link: ({
    children,
    to,
    params,
  }: {
    children: React.ReactNode;
    to: string;
    params?: Record<string, string>;
  }) => {
    const href = params
      ? Object.entries(params).reduce((p, [k, v]) => p.replace(`$${k}`, v), to)
      : to;
    return <a href={href}>{children}</a>;
  },
}));

vi.mock("../hooks", () => ({
  usePksList: (...a: unknown[]) => usePksListMock(...a),
  useUpdatePksReminder: () => ({ mutateAsync: mutateReminder, isPending: false }),
  useSubmitVendorPks: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useTerminateVendorPks: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUnterminateVendorPks: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("../../admin-vendors/hooks", () => ({
  usePksTypesList: () => ({
    isLoading: false,
    data: [
      { code: "FLM", name: "Flm", is_active: true },
      { code: "CIT", name: "Cit", is_active: true },
    ],
  }),
  useVendorsList: () => ({
    isLoading: false,
    data: { vendors: [{ id: 1, code: "V1", name: "Vendor Satu" }] },
  }),
}));

vi.mock("../../exports", () => ({
  useExportFlow: () => ({
    request: vi.fn(),
    isRequesting: false,
    requestError: null,
    dialogProps: {},
  }),
  ExportProgressDialog: () => null,
  ReportExportNotice: () => null,
}));

const item = (over: Partial<PksListItem>): PksListItem => ({
  id: 1,
  vendor_id: 1,
  vendor_code: "V1",
  vendor_name: "Vendor Satu",
  pks_type: "FLM",
  pks_type_name: "Flm",
  wod_number: "WOD-1",
  pks_name: "PKS Satu",
  effective_date: "2026-01-01",
  expiry_date: "2027-01-01",
  renewal_days: 270,
  remaining_days: 100,
  status: "aktif",
  pending: null,
  ...over,
});

const ITEMS = [
  item({ id: 1 }),
  item({ id: 2, pks_name: "PKS Lama", status: "diganti", remaining_days: -50 }),
  item({ id: 3, pks_name: "PKS Pending", pending: { op: "terminate", change_id: 9 } }),
  item({
    id: null,
    pks_name: "PKS Baru",
    status: "menunggu_persetujuan",
    pending: { op: "create", change_id: 10 },
  }),
];

const setRole = (role: "ADMIN" | "ATM-SPV") =>
  useAuthStore.setState({
    user: {
      id: 1,
      username: "u",
      fullName: "U",
      email: "u@x",
      role,
      isKaryawan: true,
      vendorId: null,
    },
  });

beforeEach(() => {
  mutateReminder.mockReset();
  usePksListMock.mockReset();
  usePksListMock.mockReturnValue({
    isLoading: false,
    isError: false,
    data: { items: ITEMS, total: ITEMS.length, page: 1, page_size: 25 },
    refetch: vi.fn(),
  });
  setRole("ADMIN");
});

describe("PksTable", () => {
  it("renders the D12 columns, status badges with text and Masa Berlaku", () => {
    render(<PksTable items={ITEMS} />);
    const headers = screen.getAllByRole("columnheader").map((h) => h.textContent);
    expect(headers).toEqual([
      "Status",
      "Type",
      "Nama Vendor",
      "Judul PKS",
      "No. WOD",
      "Tgl Efektif",
      "Tgl Masa Akhir",
      "Masa Berlaku",
      "Reminder (hari)",
    ]);
    expect(screen.getAllByText("Aktif").length).toBeGreaterThan(0);
    expect(screen.getByText("Diganti")).toBeTruthy();
    expect(screen.getAllByText("Menunggu Persetujuan").length).toBeGreaterThan(0);
    expect(screen.getAllByText("100 hari").length).toBeGreaterThan(0);
  });

  it("the Judul cell is a real link to the vendor's PKS tab (keyboard reachable)", () => {
    render(<PksTable items={[item({})]} />);
    expect(screen.getByRole("link", { name: "PKS Satu" }).getAttribute("href")).toBe(
      "/settings/admin/vendors/1",
    );
  });

  it("hides Nama Vendor when scoped to a vendor and shows Aksi only with actions", () => {
    const { rerender } = render(<PksTable items={ITEMS} vendorCode="V1" actions={false} />);
    expect(screen.queryByRole("columnheader", { name: "Nama Vendor" })).toBeNull();
    expect(screen.queryByRole("columnheader", { name: "Aksi" })).toBeNull();
    rerender(<PksTable items={ITEMS} actions={() => <span>x</span>} />);
    expect(screen.getByRole("columnheader", { name: "Aksi" })).toBeTruthy();
  });

  it("shows the pending operation next to the status", () => {
    render(<PksTable items={[ITEMS[2]]} />);
    expect(screen.getByText("terminate")).toBeTruthy();
  });
});

describe("PksFilterBar", () => {
  const base = { page: 1, page_size: 25, status: "terkini" as const, q: "" };

  it("defaults Status to Terkini and reports a status change", async () => {
    const onParamsChange = vi.fn();
    render(
      <PksFilterBar
        params={base}
        searchInput=""
        onSearchInputChange={vi.fn()}
        onParamsChange={onParamsChange}
        typeOptions={[{ value: "FLM", label: "FLM" }]}
        vendorOptions={[]}
      />,
    );
    const status = screen.getByLabelText("Status") as HTMLSelectElement;
    expect(status.value).toBe("terkini");
    await userEvent.selectOptions(status, "expired");
    expect(onParamsChange).toHaveBeenCalledWith({ status: "expired" });
  });
});

describe("PksFormDialog", () => {
  it("defaults Reminder to 270 and requires vendor, type and WOD", async () => {
    render(<PksFormDialog onClose={vi.fn()} />);
    expect(
      (screen.getByLabelText("Reminder (hari sebelum expired)") as HTMLInputElement).value,
    ).toBe("270");
    await userEvent.click(screen.getByRole("button", { name: "Ajukan" }));
    expect(await screen.findByText("Vendor wajib dipilih")).toBeTruthy();
    expect(screen.getByText("Type PKS wajib dipilih")).toBeTruthy();
    expect(screen.getByText("No. WOD wajib diisi")).toBeTruthy();
  });

  it("Ganti locks Vendor and Type", () => {
    render(<PksFormDialog replace={item({})} onClose={vi.fn()} />);
    expect((screen.getByLabelText("Vendor") as HTMLSelectElement).disabled).toBe(true);
    expect((screen.getByLabelText("Type PKS") as HTMLSelectElement).disabled).toBe(true);
  });
});

describe("PksReminderDialog", () => {
  it("rejects a reminder longer than the PKS and saves a valid one", async () => {
    const user = userEvent.setup();
    mutateReminder.mockResolvedValue({});
    render(<PksReminderDialog item={item({ id: 5 })} onClose={vi.fn()} />);
    const input = screen.getByLabelText("Reminder (hari sebelum expired)");

    await user.clear(input);
    await user.type(input, "400");
    await user.click(screen.getByRole("button", { name: "Simpan" }));
    expect(await screen.findByText(/Tidak boleh melebihi masa PKS \(365 hari\)/)).toBeTruthy();
    expect(mutateReminder).not.toHaveBeenCalled();

    await user.clear(input);
    await user.type(input, "300");
    await user.click(screen.getByRole("button", { name: "Simpan" }));
    expect(mutateReminder).toHaveBeenCalledWith({ pksId: 5, renewalDays: 300 });
  });
});

describe("PksListPage role gating and row actions", () => {
  it("ATM-SPV is read-only: no Catat PKS, no Aksi column, export stays", () => {
    setRole("ATM-SPV");
    render(<PksListPage />);
    expect(screen.queryByRole("button", { name: /Catat PKS/ })).toBeNull();
    expect(screen.queryByRole("columnheader", { name: "Aksi" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Ubah reminder" })).toBeNull();
    expect(screen.getByRole("button", { name: /Ekspor CSV/ })).toBeTruthy();
  });

  it("ADMIN sees write actions: Ganti/Terminasi only on current rows, locked while pending", () => {
    render(<PksListPage />);
    expect(screen.getByRole("button", { name: /Catat PKS/ })).toBeTruthy();
    const ganti = screen.getAllByRole("button", { name: "Ganti" }) as HTMLButtonElement[];
    expect(ganti.map((b) => b.disabled)).toEqual([false, true]);
    expect(screen.getAllByRole("button", { name: "Terminasi" })).toHaveLength(2);
    // reminder on every real record (diganti included), not on the pending create
    expect(screen.getAllByRole("button", { name: "Ubah reminder" })).toHaveLength(3);
    expect(screen.getAllByText("Menunggu persetujuan perubahan lain").length).toBeGreaterThan(0);
  });

  it("the diganti row only offers Ubah reminder", () => {
    render(<PksListPage />);
    const row = screen.getByText("PKS Lama").closest("tr") as HTMLElement;
    expect(within(row).queryByRole("button", { name: "Ganti" })).toBeNull();
    expect(within(row).queryByRole("button", { name: "Terminasi" })).toBeNull();
    expect(within(row).getByRole("button", { name: "Ubah reminder" })).toBeTruthy();
  });
});
