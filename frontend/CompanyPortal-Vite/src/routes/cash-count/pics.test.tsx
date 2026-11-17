import { CashCountPicsPage } from "@/features/cash-count-pics/CashCountPicsPage";
import { useAuthStore } from "@/lib/auth/store";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { get, post, toast } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), toast: vi.fn() }));

vi.mock("@/lib/api/client", () => ({ api: { get, post, put: vi.fn() } }));
vi.mock("@/lib/hooks/useToast", () => ({ useToast: () => ({ toast }) }));
vi.mock("@/features/exports", () => ({
  ExportProgressDialog: () => null,
  isTooManyExports: () => false,
  useExportFlow: () => ({ request: vi.fn(), isRequesting: false, dialogProps: {} }),
}));

const PIC = {
  id: 5,
  pic_role: "COORDINATOR",
  user_id: 11,
  username: "budi",
  full_name: "Budi",
  region_code: "JKT1",
  area_id: null,
  area_code: null,
  area_name: null,
  is_active: true,
};

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <CashCountPicsPage />
    </QueryClientProvider>,
  );
}

describe("CashCountPicsPage - Mapping PIC (AC-5.1-10, AC-5.1-11)", () => {
  beforeEach(() => {
    for (const f of [get, post, toast]) f.mockReset();
    get.mockImplementation(async (url: string) =>
      url.startsWith("/cash-count/areas") ? { data: { areas: [] } } : { data: { pics: [PIC] } },
    );
    useAuthStore.setState({ user: { role: "CC-CWM" } as never });
  });

  it("disable -> 202 marks the row 'Menunggu approval' and toasts the pending message", async () => {
    post.mockResolvedValue({
      data: {
        change_request_id: 9,
        status: "pending",
        entity_type: "cash_count_pics",
        op: "disable",
      },
    });
    const user = userEvent.setup();
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Nonaktifkan" }));

    expect(await screen.findByText("Menunggu approval")).toBeInTheDocument();
    expect(post).toHaveBeenCalledWith("/cash-count/pics/5/disable");
    expect(toast).toHaveBeenCalledWith({
      type: "success",
      message: expect.stringContaining("menunggu persetujuan (#9)"),
    });
    expect(screen.getByRole("button", { name: "Nonaktifkan" })).toBeDisabled();
  });

  it("add mapping -> 422 shows the API message", async () => {
    post.mockRejectedValue({ status: 422, message: "User bukan CC-COORDINATOR aktif" });
    const user = userEvent.setup();
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Tambah Mapping" }));
    await user.type(screen.getByLabelText("ID user"), "99");
    await user.type(screen.getByLabelText("Region"), "jkt2");
    await user.click(screen.getByRole("button", { name: "Ajukan" }));

    await waitFor(() =>
      expect(toast).toHaveBeenCalledWith({
        type: "error",
        message: "User bukan CC-COORDINATOR aktif",
      }),
    );
    expect(post).toHaveBeenCalledWith("/cash-count/pics", {
      pic_role: "COORDINATOR",
      user_id: 99,
      region_code: "JKT2",
    });
  });

  it("hides write actions for ADMIN", async () => {
    useAuthStore.setState({ user: { role: "ADMIN" } as never });
    renderPage();
    expect(await screen.findByText("budi")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Tambah Mapping" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Unggah CSV" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Nonaktifkan" })).not.toBeInTheDocument();
  });

  it("replace-all CSV preview calls out the implicit disable rows (AC-5.1-11)", async () => {
    post.mockResolvedValue({
      data: {
        entity: "cash-count-pics",
        total_rows: 1,
        valid_rows: 1,
        creates: 1,
        updates: 1,
        unchanged: 0,
        errors: null,
        errors_truncated: false,
        data_updates: 0,
        enables: 0,
        disables: 1,
        changes: [
          { row: 2, op: "create", key: "COORDINATOR:andi:JKT3", fields: null },
          { row: 0, op: "disable", key: "COORDINATOR:budi:JKT1", fields: null },
        ],
      },
    });
    const user = userEvent.setup();
    renderPage();
    await user.click(await screen.findByRole("button", { name: "Unggah CSV" }));
    const file = new File(["pic_role,username,region_code,area_code\n"], "pics.csv", {
      type: "text/csv",
    });
    await user.upload(screen.getByLabelText("File CSV mapping PIC"), file);
    await user.click(screen.getByRole("button", { name: "Periksa file" }));

    const warning = await screen.findByRole("region", {
      name: "Mapping yang akan dinonaktifkan",
    });
    expect(within(warning).getByText(/1 mapping aktif tidak ada di file/)).toBeInTheDocument();
    expect(within(warning).getByText("COORDINATOR:budi:JKT1")).toBeInTheDocument();
    expect(post).toHaveBeenCalledWith(
      "/cash-count/import/cash-count-pics/dry-run",
      expect.any(FormData),
    );
    expect(screen.getByRole("button", { name: "Ajukan untuk persetujuan" })).toBeEnabled();
  });
});
