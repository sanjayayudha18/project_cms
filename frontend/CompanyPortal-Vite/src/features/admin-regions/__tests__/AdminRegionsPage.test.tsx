/**
 * fe-coverage C2: Admin Regions screen with its real URL-state hook and filter bar.
 * Router, data hooks and toast are mocked; the form dialog is stubbed (own tests).
 */

import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { AdminRegion } from "../types";

const nav = vi.hoisted(() => ({ navigate: vi.fn(), search: {} as Record<string, unknown> }));
vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return { ...actual, useNavigate: () => nav.navigate, useSearch: () => nav.search };
});

const data = vi.hoisted(() => ({
  list: {} as Record<string, unknown>,
  listParams: [] as unknown[],
  refetch: vi.fn(),
  disable: vi.fn(),
  enable: vi.fn(),
  toast: vi.fn(),
}));
vi.mock("../hooks", () => ({
  useRegionsList: (params: unknown) => {
    data.listParams.push(params);
    return { ...data.list, refetch: data.refetch };
  },
  useDisableRegion: () => ({ mutate: data.disable, isPending: false }),
  useEnableRegion: () => ({ mutate: data.enable, isPending: false }),
}));
vi.mock("@/lib/hooks/useToast", () => ({ useToast: () => ({ toast: data.toast }) }));
vi.mock("../components/RegionFormDialog", () => ({
  RegionFormDialog: ({ open, region }: { open: boolean; region: AdminRegion | null }) =>
    open ? <div data-testid="form">{region ? `edit ${region.code}` : "create"}</div> : null,
}));

import { AdminRegionsPage } from "../components/AdminRegionsPage";

const region = (over: Partial<AdminRegion>): AdminRegion => ({
  id: 1,
  code: "JKT",
  region: "Jakarta",
  is_active: true,
  location_count: 3,
  created_at: null,
  updated_at: null,
  deleted_at: null,
  ...over,
});

beforeEach(() => {
  nav.navigate.mockReset();
  nav.search = {};
  data.listParams = [];
  data.refetch.mockReset();
  data.disable.mockReset();
  data.enable.mockReset();
  data.toast.mockReset();
  data.list = {
    isLoading: false,
    isError: false,
    data: {
      regions: [region({}), region({ id: 2, code: "BDG", region: null, is_active: false })],
      total: 45,
    },
  };
});

describe("AdminRegionsPage", () => {
  it("queries the list with URL defaults (page 1, 20 rows, active, no search)", () => {
    render(<AdminRegionsPage />);

    expect(data.listParams[0]).toEqual({ page: 1, page_size: 20, status: "active", q: undefined });
  });

  it("shows loading, error (with retry) and empty states", async () => {
    data.list = { isLoading: true, isError: false, data: undefined };
    const { rerender } = render(<AdminRegionsPage />);
    expect(screen.queryByRole("table")).not.toBeInTheDocument();

    data.list = { isLoading: false, isError: true, data: undefined };
    rerender(<AdminRegionsPage />);
    expect(screen.getByText("Gagal memuat daftar region")).toBeInTheDocument();
    await userEvent.setup().click(screen.getByRole("button", { name: "Coba Lagi" }));
    expect(data.refetch).toHaveBeenCalled();

    data.list = { isLoading: false, isError: false, data: { regions: [], total: 0 } };
    rerender(<AdminRegionsPage />);
    expect(
      screen.getByText("Tidak ada region yang sesuai dengan filter saat ini"),
    ).toBeInTheDocument();
    expect(screen.getByText("Halaman 1 dari 1 (0 region)")).toBeInTheDocument();
  });

  it("pages through results via the URL", async () => {
    const user = userEvent.setup();
    render(<AdminRegionsPage />);

    expect(screen.getByText("Halaman 1 dari 3 (45 region)")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Sebelumnya" })).toBeDisabled();

    await user.click(screen.getByRole("button", { name: "Berikutnya" }));
    expect(nav.navigate).toHaveBeenLastCalledWith({ to: ".", search: { page: 2 } });
  });

  it("on the last page, Berikutnya is disabled and Sebelumnya goes back", async () => {
    nav.search = { page: 3, status: "all" };
    render(<AdminRegionsPage />);

    expect(screen.getByRole("button", { name: "Berikutnya" })).toBeDisabled();
    await userEvent.setup().click(screen.getByRole("button", { name: "Sebelumnya" }));
    expect(nav.navigate).toHaveBeenLastCalledWith({ to: ".", search: { page: 2, status: "all" } });
  });

  it("changing the status filter resets to page 1", async () => {
    nav.search = { page: 2 };
    render(<AdminRegionsPage />);

    await userEvent.setup().selectOptions(screen.getByLabelText("Status"), "inactive");

    expect(nav.navigate).toHaveBeenLastCalledWith({ to: ".", search: { status: "inactive" } });
  });

  it("search is debounced into the URL and resets to page 1", async () => {
    nav.search = { page: 2 };
    render(<AdminRegionsPage />);

    await userEvent.setup().type(screen.getByLabelText("Cari"), "band");

    expect(nav.navigate).not.toHaveBeenCalled();
    await waitFor(() =>
      expect(nav.navigate).toHaveBeenLastCalledWith({ to: ".", search: { q: "band" } }),
    );
  });

  it("disabling asks for confirmation, then toasts success", async () => {
    data.disable.mockImplementation((_id, opts: { onSuccess: () => void }) => opts.onSuccess());
    const user = userEvent.setup();
    render(<AdminRegionsPage />);

    await user.click(screen.getByRole("button", { name: "Nonaktifkan" }));
    const dialog = screen.getByRole("dialog");
    expect(dialog).toHaveTextContent(
      'Nonaktifkan region "JKT"? Region dengan lokasi aktif tidak dapat dinonaktifkan.',
    );
    await user.click(within(dialog).getByRole("button", { name: "Nonaktifkan" }));

    expect(data.disable).toHaveBeenCalledWith(1, expect.anything());
    expect(data.toast).toHaveBeenCalledWith({
      type: "success",
      message: 'Region "JKT" berhasil diubah',
    });
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("a blocked disable (409) shows the server message as an error toast", async () => {
    data.disable.mockImplementation((_id, opts: { onError: (e: Error) => void }) =>
      opts.onError(new Error("Region masih dipakai lokasi aktif")),
    );
    const user = userEvent.setup();
    render(<AdminRegionsPage />);

    await user.click(screen.getByRole("button", { name: "Nonaktifkan" }));
    await user.click(
      within(screen.getByRole("dialog")).getByRole("button", { name: "Nonaktifkan" }),
    );

    expect(data.toast).toHaveBeenCalledWith({
      type: "error",
      message: "Region masih dipakai lokasi aktif",
    });
  });

  it("enabling an inactive region uses the enable mutation", async () => {
    const user = userEvent.setup();
    render(<AdminRegionsPage />);

    await user.click(screen.getByRole("button", { name: "Aktifkan" }));
    const dialog = screen.getByRole("dialog");
    expect(dialog).toHaveTextContent('Aktifkan kembali region "BDG"?');
    await user.click(within(dialog).getByRole("button", { name: "Aktifkan" }));

    expect(data.enable).toHaveBeenCalledWith(2, expect.anything());
    expect(data.disable).not.toHaveBeenCalled();
  });

  it("cancel closes the confirmation without calling the API", async () => {
    const user = userEvent.setup();
    render(<AdminRegionsPage />);

    await user.click(screen.getByRole("button", { name: "Nonaktifkan" }));
    await user.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Batal" }));

    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(data.disable).not.toHaveBeenCalled();
  });

  it("opens the form in create mode, and in edit mode for a row", async () => {
    const user = userEvent.setup();
    render(<AdminRegionsPage />);

    await user.click(screen.getByRole("button", { name: "Tambah Region" }));
    expect(screen.getByTestId("form")).toHaveTextContent("create");

    await act(async () => {
      screen.getAllByRole("button", { name: "Ubah" })[1]?.click();
    });
    expect(screen.getByTestId("form")).toHaveTextContent("edit BDG");
  });
});
