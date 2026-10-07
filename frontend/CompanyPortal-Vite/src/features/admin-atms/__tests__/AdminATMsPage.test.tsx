import { useToastStore } from "@/lib/hooks/useToast";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AdminATMsPage } from "../components/AdminATMsPage";
import type { AdminATM } from "../types";

const ACTIVE_ATM: AdminATM = {
  id: 9,
  terminal_id: "TATM009",
  location_id: 10,
  location_name: "Jakarta Pusat",
  machine_type: "ATM",
  brand: "NCR",
  model: "SelfServ",
  operation_hours: "24 Hours",
  deployment_type: "ONSITE",
  capacity_amount: null,
  low_threshold_amount: null,
  critical_threshold_amount: null,
  blacklisted: false,
  escrow_account: null,
  priority_class: "VIP",
  is_active: true,
  created_at: null,
  updated_at: null,
  deleted_at: null,
};

const useATMsListMock = vi.fn();
const disableMutate = vi.fn();
const navigateMock = vi.fn();
let currentSearch: Record<string, unknown> = {};

vi.mock("../hooks", () => ({
  useATMsList: (...args: unknown[]) => useATMsListMock(...args),
  useDisableATM: () => ({ mutate: disableMutate, isPending: false }),
  useEnableATM: () => ({ mutate: vi.fn(), isPending: false }),
  useCreateATM: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpdateATM: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useLocationOptions: () => ({ data: { locations: [] } }),
}));

vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    useNavigate: () => navigateMock,
    useSearch: () => currentSearch,
  };
});

beforeEach(() => {
  useATMsListMock.mockReset();
  disableMutate.mockReset();
  navigateMock.mockReset();
  currentSearch = {};
  useToastStore.setState({ toasts: [] });
  useATMsListMock.mockReturnValue({
    data: { atms: [ACTIVE_ATM], page: 1, page_size: 25, total: 1 },
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  });
});

describe("AdminATMsPage", () => {
  it("renders Status and Prioritas badges with icon + label", () => {
    render(<AdminATMsPage />);

    // Scoped to the table: "Aktif" also appears as a Status filter option.
    const table = within(screen.getByRole("table"));
    expect(table.getByText("Aktif")).toBeInTheDocument();
    expect(table.getByText("VIP")).toBeInTheDocument();
  });

  it("changing a filter resets page to 1 and navigates with the new params", async () => {
    currentSearch = { page: 3 };
    const user = userEvent.setup();
    render(<AdminATMsPage />);

    await user.selectOptions(screen.getByLabelText("Status"), "disabled");

    expect(navigateMock).toHaveBeenCalledWith({
      to: ".",
      search: expect.objectContaining({ status: "disabled" }),
    });
    const call = navigateMock.mock.calls.find((c) => c[0].search?.status === "disabled");
    expect(call?.[0].search.page).toBeUndefined(); // page 1 is the default, omitted from the URL
  });

  it("disable confirmation flow: confirm calls the mutation and shows a success toast", async () => {
    const user = userEvent.setup();
    disableMutate.mockImplementation((_id, { onSuccess }) => {
      onSuccess({ change_request_id: 21, status: "pending", entity_type: "atm", op: "disable" });
    });

    render(<AdminATMsPage />);

    await user.click(screen.getByRole("button", { name: "Nonaktifkan" }));
    const dialog = screen.getByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: "Nonaktifkan" }));

    expect(disableMutate).toHaveBeenCalledWith(ACTIVE_ATM.id, expect.anything());
    const toasts = useToastStore.getState().toasts;
    expect(toasts).toHaveLength(1);
    expect(toasts[0]).toMatchObject({
      type: "success",
      message: "Perubahan diajukan dan menunggu persetujuan (#21)",
    });
  });

  it("disable confirmation flow: cancel does not call the mutation", async () => {
    const user = userEvent.setup();
    render(<AdminATMsPage />);

    await user.click(screen.getByRole("button", { name: "Nonaktifkan" }));
    const dialog = screen.getByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: "Batal" }));

    expect(disableMutate).not.toHaveBeenCalled();
  });

  it("shows a loading skeleton instead of the table while loading", () => {
    useATMsListMock.mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      refetch: vi.fn(),
    });
    render(<AdminATMsPage />);

    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("shows an inline error with a retry button that calls refetch", async () => {
    const refetch = vi.fn();
    useATMsListMock.mockReturnValue({ data: undefined, isLoading: false, isError: true, refetch });
    const user = userEvent.setup();
    render(<AdminATMsPage />);

    expect(screen.getByText("Gagal memuat daftar ATM")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Coba Lagi" }));
    expect(refetch).toHaveBeenCalled();
  });

  it("shows an empty state when the filtered list has no rows", () => {
    useATMsListMock.mockReturnValue({
      data: { atms: [], page: 1, page_size: 25, total: 0 },
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    });
    render(<AdminATMsPage />);

    expect(
      screen.getByText("Tidak ada ATM yang sesuai dengan filter saat ini"),
    ).toBeInTheDocument();
  });

  it("pagination: Sebelumnya is disabled on page 1, Berikutnya navigates to the next page", async () => {
    // totalPages is derived from the URL state's page_size, not the response's --
    // set both consistently so page 1 of a 2-item, 1-per-page list has a next page.
    currentSearch = { page_size: 1 };
    useATMsListMock.mockReturnValue({
      data: { atms: [ACTIVE_ATM], page: 1, page_size: 1, total: 2 },
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    });
    const user = userEvent.setup();
    render(<AdminATMsPage />);

    expect(screen.getByRole("button", { name: "Sebelumnya" })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Berikutnya" }));
    expect(navigateMock).toHaveBeenCalledWith({ to: ".", search: { page: 2, page_size: 1 } });
  });

  it("clicking Tambah ATM opens the create form dialog", async () => {
    const user = userEvent.setup();
    render(<AdminATMsPage />);

    await user.click(screen.getByRole("button", { name: "Tambah ATM" }));
    expect(screen.getByRole("dialog", { name: "Tambah ATM" })).toBeInTheDocument();
  });

  it("clicking Ubah opens the edit form dialog pre-filled with the row's data", async () => {
    const user = userEvent.setup();
    render(<AdminATMsPage />);

    await user.click(screen.getByRole("button", { name: "Ubah" }));
    const dialog = screen.getByRole("dialog", { name: "Ubah ATM" });
    expect(within(dialog).getByLabelText("Terminal ID")).toHaveValue(ACTIVE_ATM.terminal_id);
  });
});

vi.mock("../components/ATMAssignmentsDialog", () => ({ ATMAssignmentsDialog: () => null }));
vi.mock("../../master-data/pending", () => ({ usePendingEntityIds: () => new Set<number>() }));
