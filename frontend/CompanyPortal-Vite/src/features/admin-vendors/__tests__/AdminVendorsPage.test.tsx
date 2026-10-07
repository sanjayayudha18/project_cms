import { useToastStore } from "@/lib/hooks/useToast";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AdminVendorsPage } from "../components/AdminVendorsPage";
import type { AdminVendor } from "../types";

const ACTIVE_VENDOR: AdminVendor = {
  id: 9,
  code: "VDR009",
  name: "PT Vendor Aktif",
  contact_email: "aktif@vendor.com",
  contact_phone: "021-555-0199",
  legal_name: "PT Vendor Satu",
  npwp: "012345678901000",
  hq_address: "Jl. Gatot Subroto",
  is_active: true,
  deleted_at: null,
};

const useVendorsListMock = vi.fn();
const disableMutate = vi.fn();

vi.mock("../hooks", () => ({
  useVendorsList: (...args: unknown[]) => useVendorsListMock(...args),
  useDisableVendor: () => ({ mutate: disableMutate, isPending: false }),
  useEnableVendor: () => ({ mutate: vi.fn(), isPending: false }),
  useCreateVendor: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpdateVendor: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    useNavigate: () => vi.fn(),
    useSearch: () => ({}),
    Link: ({ children }: { children: React.ReactNode }) => <a href="/x">{children}</a>,
  };
});

beforeEach(() => {
  useVendorsListMock.mockReset();
  disableMutate.mockReset();
  useToastStore.setState({ toasts: [] });
  useVendorsListMock.mockReturnValue({
    data: { vendors: [ACTIVE_VENDOR], page: 1, page_size: 25, total: 1 },
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  });
});

describe("AdminVendorsPage — linked-active-users warning on disable (Task 9, Req 8.5)", () => {
  it("shows a warning toast (not a plain success) when the disable response carries a warning", async () => {
    const user = userEvent.setup();
    disableMutate.mockImplementation((_id, { onSuccess }) => {
      onSuccess({
        change_request_id: 12,
        status: "pending",
        entity_type: "vendor",
        op: "disable",
        warning: "Vendor masih memiliki 3 pengguna aktif yang terhubung",
        linked_active_users: 3,
      });
    });

    render(<AdminVendorsPage />);

    await user.click(screen.getByRole("button", { name: "Nonaktifkan" }));
    const dialog = screen.getByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: "Nonaktifkan" }));

    expect(disableMutate).toHaveBeenCalledWith(ACTIVE_VENDOR.id, expect.anything());
    const toasts = useToastStore.getState().toasts;
    expect(toasts).toHaveLength(1);
    expect(toasts[0]).toMatchObject({
      type: "warning",
      message:
        "Perubahan diajukan dan menunggu persetujuan (#12). Vendor masih memiliki 3 pengguna aktif yang terhubung",
    });
  });

  it("shows a plain success toast when there is no linked-users warning", async () => {
    const user = userEvent.setup();
    disableMutate.mockImplementation((_id, { onSuccess }) => {
      onSuccess({ change_request_id: 12, status: "pending", entity_type: "vendor", op: "disable" });
    });

    render(<AdminVendorsPage />);

    await user.click(screen.getByRole("button", { name: "Nonaktifkan" }));
    const dialog = screen.getByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: "Nonaktifkan" }));

    const toasts = useToastStore.getState().toasts;
    expect(toasts).toHaveLength(1);
    expect(toasts[0]).toMatchObject({
      type: "success",
      message: "Perubahan diajukan dan menunggu persetujuan (#12)",
    });
  });
});
vi.mock("../../master-data/pending", () => ({ usePendingEntityIds: () => new Set<number>() }));
