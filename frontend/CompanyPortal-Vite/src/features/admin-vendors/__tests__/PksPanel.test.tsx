/**
 * vendor-pks 11.5 (Req 9.4-9.8): PKS tab -- buttons per state, pending block, history, and the
 * hand-off into the two dialogs. Data hooks are mocked; dialogs, schemas and copy are real.
 */

import { useToastStore } from "@/lib/hooks/useToast";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

const m = vi.hoisted(() => ({
  query: {} as Record<string, unknown>,
  submit: vi.fn(),
  terminate: vi.fn(),
  unterminate: vi.fn(),
}));
vi.mock("../hooks", () => ({
  useVendorPks: () => m.query,
  usePksTypesList: () => ({
    data: [{ id: 1, code: "FLM", name: "First Line Maintenance", is_active: true, in_use: true }],
    isLoading: false,
  }),
  useSubmitVendorPks: () => ({ mutateAsync: m.submit, isPending: false }),
  useTerminateVendorPks: () => ({ mutateAsync: m.terminate, isPending: false }),
  useUnterminateVendorPks: () => ({ mutateAsync: m.unterminate, isPending: false }),
}));

import { PksPanel } from "../components/PksPanel";

const ACTIVE = {
  id: 7,
  vendor_id: 42,
  wod_number: "WOD/2026/001",
  pks_name: "PKS Cash Logistik 2026",
  effective_date: "2026-01-15",
  expiry_date: "2027-01-14",
  validity_months: 12,
  remaining_days: 120,
  terminated_at: null,
  termination_reason: null,
  replaced_at: null,
};
const OLD = {
  id: 3,
  vendor_id: 42,
  wod_number: "WOD/2024/099",
  pks_name: "PKS Lama",
  effective_date: "2024-01-01",
  expiry_date: "2024-12-31",
  validity_months: 12,
  remaining_days: -400,
  terminated_at: null,
  termination_reason: null,
  replaced_at: "2026-01-15T00:00:00Z",
};

function detail(over: Record<string, unknown> = {}) {
  const pks_status = (over.pks_status as string) ?? "aktif";
  const active = over.active !== undefined ? over.active : ACTIVE;
  const history = over.history !== undefined ? over.history : [OLD];
  const pending = over.pending !== undefined ? over.pending : null;

  const isCompletelyEmpty =
    pks_status === "belum_diisi" &&
    !active &&
    Array.isArray(history) &&
    history.length === 0 &&
    !pending;

  const defaultGroups = isCompletelyEmpty
    ? []
    : [
        {
          pks_type: { code: "FLM", name: "First Line Maintenance" },
          pks_status,
          active,
          history,
          pending,
        },
      ];

  return {
    isLoading: false,
    isError: false,
    data: {
      vendor_id: 42,
      kind: "FLM_VENDOR" as const,
      today: "2026-09-16",
      renewal_months: 6,
      groups: over.groups !== undefined ? over.groups : defaultGroups,
      ...over,
    },
  };
}

const btn = (name: string) => screen.queryByRole("button", { name });

beforeEach(() => {
  m.query = detail();
  m.submit.mockReset();
  m.terminate.mockReset();
  m.unterminate.mockReset();
  useToastStore.setState({ toasts: [] });
});

describe("PksPanel states", () => {
  it("active PKS: shows the card, history table and offers 'Catat PKS baru' + 'Terminate PKS'", () => {
    render(<PksPanel vendorId={42} vendor={{ is_active: true }} />);

    const card = screen.getByRole("region", { name: "PKS aktif" });
    expect(within(card).getByText("PKS Cash Logistik 2026")).toBeInTheDocument();
    expect(within(card).getByText("15/01/2026")).toBeInTheDocument();
    expect(within(card).getByText("14/01/2027")).toBeInTheDocument();
    expect(within(card).getByText("1 tahun")).toBeInTheDocument();
    expect(within(card).getByText("120 hari")).toBeInTheDocument();
    expect(within(card).getByText("Aktif")).toBeInTheDocument();
    expect(screen.getByText(/Need Renewal mulai 6 bulan/)).toBeInTheDocument();

    expect(screen.getByRole("table", { name: "Riwayat PKS" })).toHaveTextContent("PKS Lama");
    expect(btn("Catat PKS baru")).toBeEnabled();
    expect(btn("Terminate PKS")).toBeEnabled();
    expect(btn("Batalkan terminate")).toBeNull();
  });

  it("no PKS yet: empty message, only 'Catat PKS baru'", () => {
    m.query = detail({ pks_status: "belum_diisi", active: null, history: [] });
    render(<PksPanel vendorId={42} vendor={{ is_active: true }} />);

    expect(screen.getByText("Belum ada PKS yang dicatat.")).toBeInTheDocument();
    expect(screen.getByText("Belum ada riwayat.")).toBeInTheDocument();
    expect(btn("Catat PKS baru")).toBeEnabled();
    expect(btn("Terminate PKS")).toBeNull();
    expect(btn("Batalkan terminate")).toBeNull();
  });

  it("terminated PKS: shows reason and offers 'Batalkan terminate' + 'Catat PKS baru', no terminate", () => {
    m.query = detail({
      pks_status: "terminated",
      active: {
        ...ACTIVE,
        terminated_at: "2026-05-01T00:00:00Z",
        termination_reason: "Wanprestasi",
      },
    });
    render(<PksPanel vendorId={42} vendor={{ is_active: true }} />);

    const card = screen.getByRole("region", { name: "PKS aktif" });
    expect(card).toHaveTextContent("01/05/2026: Wanprestasi");
    expect(within(card).getByText("Terminated")).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "Catat PKS baru" })).toHaveLength(1);
    expect(btn("Batalkan terminate")).toBeEnabled();
    expect(btn("Terminate PKS")).toBeNull();
  });

  it("pending change: shows the pending block and disables terminate action for that type", () => {
    m.query = detail({
      pending: { op: "disable", maker_name: "Budi", payload: { reason: "Kontrak diputus" } },
    });
    render(<PksPanel vendorId={42} vendor={{ is_active: true }} />);

    const block = screen.getByRole("region", { name: "Menunggu persetujuan" });
    expect(block).toHaveTextContent("Terminate PKS");
    expect(block).toHaveTextContent("Budi");
    expect(block).toHaveTextContent("Kontrak diputus");
    expect(screen.getByText("Masih ada perubahan PKS yang menunggu persetujuan.")).toBeVisible();
    expect(btn("Terminate PKS")).toBeDisabled();
  });

  it("multi-type: terminate is disabled only for the type that has a pending change", () => {
    m.query = {
      isLoading: false,
      isError: false,
      data: {
        vendor_id: 42,
        kind: "FLM_VENDOR",
        today: "2026-09-16",
        renewal_months: 6,
        groups: [
          {
            pks_type: { code: "FLM", name: "First Line Maintenance" },
            pks_status: "aktif",
            active: { ...ACTIVE, id: 10, pks_name: "PKS FLM" },
            history: [],
            pending: { op: "disable", maker_name: "Budi", payload: { reason: "Putus kontrak" } },
          },
          {
            pks_type: { code: "SLM", name: "Second Line Maintenance" },
            pks_status: "aktif",
            active: { ...ACTIVE, id: 20, pks_name: "PKS SLM" },
            history: [],
            pending: null,
          },
        ],
      },
    };
    render(<PksPanel vendorId={42} vendor={{ is_active: true }} />);

    const flmCard = screen.getByRole("article", { name: "PKS FLM" });
    const slmCard = screen.getByRole("article", { name: "PKS SLM" });

    expect(within(flmCard).getByRole("button", { name: "Terminate PKS" })).toBeDisabled();
    expect(
      within(flmCard).getByText("Masih ada perubahan PKS yang menunggu persetujuan."),
    ).toBeInTheDocument();

    expect(within(slmCard).getByRole("button", { name: "Terminate PKS" })).toBeEnabled();
    expect(
      within(slmCard).queryByText("Masih ada perubahan PKS yang menunggu persetujuan."),
    ).toBeNull();
  });

  it("pending create shows the proposed PKS name", () => {
    m.query = detail({
      pending: { op: "create", maker_name: "Budi", payload: { pks_name: "PKS Usulan" } },
    });
    render(<PksPanel vendorId={42} vendor={{ is_active: true }} />);

    const block = screen.getByRole("region", { name: "Menunggu persetujuan" });
    expect(block).toHaveTextContent("Catat PKS baru");
    expect(block).toHaveTextContent("PKS Usulan");
  });

  it("inactive vendor: actions disabled with 'Vendor nonaktif' hint", () => {
    render(<PksPanel vendorId={42} vendor={{ is_active: false }} />);

    expect(screen.getByText("Vendor nonaktif. Aktifkan vendor dulu.")).toBeVisible();
    expect(btn("Catat PKS baru")).toBeDisabled();
    expect(btn("Terminate PKS")).toBeDisabled();
  });

  it("shows loading and error states", () => {
    m.query = { isLoading: true, isError: false, data: undefined };
    const { rerender } = render(<PksPanel vendorId={42} vendor={undefined} />);
    expect(screen.getByText("Memuat PKS...")).toBeInTheDocument();

    m.query = { isLoading: false, isError: true, error: { message: "boom" }, data: undefined };
    rerender(<PksPanel vendorId={42} vendor={undefined} />);
    expect(screen.getByRole("alert")).toHaveTextContent("boom");
  });
});

describe("PksPanel dialog hand-off", () => {
  it("'Catat PKS baru' opens the form with the replace notice when a PKS is active", async () => {
    const user = userEvent.setup();
    render(<PksPanel vendorId={42} vendor={{ is_active: true }} />);

    await user.click(screen.getByRole("button", { name: "Catat PKS baru" }));
    const dialog = screen.getByRole("dialog", { name: "Catat PKS baru" });
    expect(dialog).toHaveTextContent("PKS saat ini pindah ke riwayat setelah PKS baru disetujui.");
  });

  it("'Catat PKS baru' without an active PKS has no replace notice", async () => {
    m.query = detail({ pks_status: "belum_diisi", active: null, history: [] });
    const user = userEvent.setup();
    render(<PksPanel vendorId={42} vendor={{ is_active: true }} />);

    await user.click(screen.getByRole("button", { name: "Catat PKS baru" }));
    expect(screen.getByRole("dialog")).not.toHaveTextContent("pindah ke riwayat");
  });

  it("'Terminate PKS' submits the reason for the active PKS id", async () => {
    m.terminate.mockResolvedValue({ change_request_id: 55 });
    const user = userEvent.setup();
    render(<PksPanel vendorId={42} vendor={{ is_active: true }} />);

    await user.click(screen.getByRole("button", { name: "Terminate PKS" }));
    const dialog = screen.getByRole("dialog");
    expect(dialog).toHaveTextContent("Terminate PKS: PKS Cash Logistik 2026");
    await user.type(within(dialog).getByLabelText("Alasan"), "Kontrak diputus");
    await user.click(within(dialog).getByRole("button", { name: "Ajukan" }));

    await waitFor(() =>
      expect(m.terminate).toHaveBeenCalledWith({
        pksId: 7,
        payload: { reason: "Kontrak diputus" },
      }),
    );
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("'Batalkan terminate' uses the unterminate mutation", async () => {
    m.unterminate.mockResolvedValue({ change_request_id: 56 });
    m.query = detail({
      pks_status: "terminated",
      active: { ...ACTIVE, terminated_at: "2026-05-01" },
    });
    const user = userEvent.setup();
    render(<PksPanel vendorId={42} vendor={{ is_active: true }} />);

    await user.click(screen.getByRole("button", { name: "Batalkan terminate" }));
    const dialog = screen.getByRole("dialog");
    await user.type(within(dialog).getByLabelText("Alasan"), "Salah input");
    await user.click(within(dialog).getByRole("button", { name: "Ajukan" }));

    await waitFor(() =>
      expect(m.unterminate).toHaveBeenCalledWith({ pksId: 7, payload: { reason: "Salah input" } }),
    );
    expect(m.terminate).not.toHaveBeenCalled();
  });
});
