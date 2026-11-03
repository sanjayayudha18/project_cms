import { useToastStore } from "@/lib/hooks/useToast";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ImportApiError, type ImportPreview } from "./api";
import { MasterDataIOPage } from "./components/MasterDataIOPage";

const downloadTemplate = vi.fn();
const router = vi.hoisted(() => ({ search: {} as Record<string, unknown>, navigate: vi.fn() }));
vi.mock("@tanstack/react-router", () => ({
  useSearch: () => router.search,
  useNavigate: () => router.navigate,
  Link: ({
    to,
    params,
    search,
    children,
  }: {
    to: string;
    params?: Record<string, string>;
    search?: Record<string, unknown>;
    children: React.ReactNode;
  }) => (
    <a
      href={
        to.replace("$dataset", params?.dataset ?? "") +
        (search ? `?${new URLSearchParams(search as Record<string, string>)}` : "")
      }
    >
      {children}
    </a>
  ),
}));
vi.mock("@/features/exports", async (orig) => ({
  ...(await orig<typeof import("@/features/exports")>()),
  downloadMasterDataTemplate: (...a: unknown[]) => downloadTemplate(...a),
}));

const dryRun = vi.fn();
const confirm = vi.fn();
vi.mock("./api", async (orig) => ({
  ...(await orig<typeof import("./api")>()),
  dryRunImport: (...a: unknown[]) => dryRun(...a),
  confirmImport: (...a: unknown[]) => confirm(...a),
}));

const preview = (over: Partial<ImportPreview>): ImportPreview => ({
  entity: "vendors",
  total_rows: 2,
  valid_rows: 2,
  creates: 1,
  updates: 1,
  unchanged: 0,
  errors: [],
  errors_truncated: false,
  data_updates: 1,
  enables: 0,
  disables: 0,
  changes: [
    { row: 2, op: "create", key: "VND07", fields: [{ field: "name", before: "", after: "PT A" }] },
    { row: 3, op: "update", key: "VND01", fields: [{ field: "name", before: "B", after: "C" }] },
  ],
  ...over,
});

async function upload(): Promise<void> {
  render(
    <QueryClientProvider client={new QueryClient()}>
      <MasterDataIOPage />
    </QueryClientProvider>,
  );
  const file = new File(["id,code\n,V1\n"], "v.csv", { type: "text/csv" });
  await userEvent.upload(screen.getByLabelText("File CSV impor"), file);
  await userEvent.click(screen.getByRole("button", { name: "Periksa file" }));
}

describe("MasterDataIOPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    router.search = {};
    useToastStore.setState({ toasts: [] });
  });

  it("shows the new title, description and the Impor baru tab by default", () => {
    renderPage();
    expect(screen.getByRole("heading", { name: "Impor CSV Data Master" })).toBeTruthy();
    expect(screen.getByText(/Data baru berubah setelah pengajuan disetujui/)).toBeTruthy();
    expect(screen.getByRole("tab", { name: "Impor baru" }).getAttribute("aria-selected")).toBe(
      "true",
    );
    expect(screen.getByRole("tab", { name: "Riwayat impor" }).getAttribute("aria-selected")).toBe(
      "false",
    );
    expect(screen.getByRole("tabpanel", { name: "Impor baru" })).toBeTruthy();
    expect(screen.getByLabelText("File CSV impor")).toBeTruthy();
  });

  it("shows the history placeholder when ?tab=history and hides the import form", () => {
    router.search = { tab: "history" };
    renderPage();
    expect(screen.getByRole("tab", { name: "Riwayat impor" }).getAttribute("aria-selected")).toBe(
      "true",
    );
    expect(screen.getByRole("tabpanel", { name: "Riwayat impor" })).toBeTruthy();
    expect(screen.queryByLabelText("File CSV impor")).toBeNull();
  });

  it("falls back to Impor baru for an unknown ?tab value", () => {
    router.search = { tab: "bogus" };
    renderPage();
    expect(screen.getByRole("tab", { name: "Impor baru" }).getAttribute("aria-selected")).toBe(
      "true",
    );
  });

  it("writes the chosen tab to the URL and drops the default one", async () => {
    renderPage();
    await userEvent.click(screen.getByRole("tab", { name: "Riwayat impor" }));
    expect(router.navigate).toHaveBeenLastCalledWith({ to: ".", search: { tab: "history" } });

    router.search = { tab: "history" };
    renderPage();
    await userEvent.click(screen.getAllByRole("tab", { name: "Impor baru" })[1]);
    expect(router.navigate).toHaveBeenLastCalledWith({ to: ".", search: {} });
  });

  it("moves between tabs with the arrow keys and uses a roving tabindex", async () => {
    renderPage();
    const first = screen.getByRole("tab", { name: "Impor baru" });
    const second = screen.getByRole("tab", { name: "Riwayat impor" });
    expect(first.getAttribute("tabindex")).toBe("0");
    expect(second.getAttribute("tabindex")).toBe("-1");

    first.focus();
    await userEvent.keyboard("{ArrowRight}");
    expect(router.navigate).toHaveBeenLastCalledWith({ to: ".", search: { tab: "history" } });
    expect(document.activeElement).toBe(second);

    await userEvent.keyboard("{ArrowLeft}");
    expect(router.navigate).toHaveBeenLastCalledWith({ to: ".", search: {} });
    expect(document.activeElement).toBe(first);
  });

  const renderPage = () =>
    render(
      <QueryClientProvider client={new QueryClient()}>
        <MasterDataIOPage />
      </QueryClientProvider>,
    );

  it("downloads the template and links to the export preview of the chosen entity", async () => {
    renderPage();
    await userEvent.click(screen.getByRole("button", { name: "Unduh templat" }));
    await waitFor(() => expect(downloadTemplate).toHaveBeenCalledWith("vendors"));
    expect(screen.queryByRole("button", { name: "Minta ekspor" })).toBeNull();
    expect(screen.queryByLabelText("Status data ekspor")).toBeNull();
    const link = () => screen.getByRole("link", { name: "Ekspor data ini" });
    expect(link().getAttribute("href")).toBe("/exports/vendors");
    await userEvent.selectOptions(screen.getByLabelText("Entitas"), "atms");
    expect(link().getAttribute("href")).toBe("/exports/atms");
  });

  it("blocks submit and lists row errors when the dry-run finds problems", async () => {
    dryRun.mockResolvedValue(
      preview({ valid_rows: 1, errors: [{ row: 3, field: "code", message: "wajib diisi" }] }),
    );
    await upload();

    expect(await screen.findByText("wajib diisi")).toBeTruthy();
    expect(
      (screen.getByRole("button", { name: "Ajukan untuk persetujuan" }) as HTMLButtonElement)
        .disabled,
    ).toBe(true);
  });

  it("marks the current step and resets to step 2 when the entity changes", async () => {
    dryRun.mockResolvedValue(preview({}));
    await upload();

    await screen.findByRole("button", { name: "Ajukan untuk persetujuan" });
    expect(
      screen
        .getByText(/3\. Tinjau dan ajukan/)
        .closest("li")
        ?.getAttribute("aria-current"),
    ).toBe("step");

    await userEvent.selectOptions(screen.getByLabelText("Entitas"), "atms");

    expect(screen.queryByRole("button", { name: "Ajukan untuk persetujuan" })).toBeNull();
    expect(
      screen
        .getByText(/2\. Unggah dan periksa/)
        .closest("li")
        ?.getAttribute("aria-current"),
    ).toBe("step");
  });

  it("asks for confirmation before sending, then shows the pending result panel", async () => {
    dryRun.mockResolvedValue(preview({}));
    confirm.mockResolvedValue({ batch_id: 9, rows: 2, approval_request_id: null, existing: false });
    await upload();

    await userEvent.click(await screen.findByRole("button", { name: "Ajukan untuk persetujuan" }));
    expect(confirm).not.toHaveBeenCalled();
    expect(screen.getByText(/Data belum berubah sampai pengajuan disetujui\./)).toBeTruthy();
    expect(screen.getByText(/File: v\.csv/)).toBeTruthy();

    await userEvent.click(screen.getByRole("button", { name: "Ajukan" }));

    expect(await screen.findByText("#9")).toBeTruthy();
    expect(confirm).toHaveBeenCalledOnce();
    expect(screen.getByText("Menunggu persetujuan")).toBeTruthy();
    expect(screen.getByRole("link", { name: "Lihat di riwayat" }).getAttribute("href")).toContain(
      "tab=history&batch=9",
    );

    await userEvent.click(screen.getByRole("button", { name: "Impor file lain" }));
    expect(screen.queryByText("#9")).toBeNull();
    expect(screen.queryByRole("button", { name: "Ajukan untuk persetujuan" })).toBeNull();
  });

  it("does not claim a pending status for an existing batch", async () => {
    dryRun.mockResolvedValue(preview({}));
    confirm.mockResolvedValue({ batch_id: 4, rows: 2, approval_request_id: 1, existing: true });
    await upload();

    await userEvent.click(await screen.findByRole("button", { name: "Ajukan untuk persetujuan" }));
    await userEvent.click(screen.getByRole("button", { name: "Ajukan" }));

    expect(await screen.findByText(/sudah diajukan sebelumnya/)).toBeTruthy();
    expect(screen.queryByText("Menunggu persetujuan")).toBeNull();
  });

  it("returns to the error table when the submit is rejected with 422", async () => {
    dryRun.mockResolvedValue(preview({}));
    confirm.mockRejectedValue(
      new ImportApiError("validasi gagal", 422, [{ row: 2, field: "code", message: "duplikat" }]),
    );
    await upload();

    await userEvent.click(await screen.findByRole("button", { name: "Ajukan untuk persetujuan" }));
    await userEvent.click(screen.getByRole("button", { name: "Ajukan" }));

    expect(await screen.findByText("duplikat")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Ajukan" })).toBeNull();
    expect(
      (screen.getByRole("button", { name: "Ajukan untuk persetujuan" }) as HTMLButtonElement)
        .disabled,
    ).toBe(true);
  });

  it("toasts other submit errors and keeps the check result", async () => {
    dryRun.mockResolvedValue(preview({}));
    confirm.mockRejectedValue(new ImportApiError("Ada perubahan tertunda", 409));
    await upload();

    await userEvent.click(await screen.findByRole("button", { name: "Ajukan untuk persetujuan" }));
    await userEvent.click(screen.getByRole("button", { name: "Ajukan" }));

    await waitFor(() =>
      expect(useToastStore.getState().toasts[0]?.message).toBe("Ada perubahan tertunda"),
    );
    expect(screen.queryByRole("button", { name: "Ajukan" })).toBeNull();
    expect(screen.getByRole("button", { name: "Ajukan untuk persetujuan" })).toBeTruthy();
  });

  it("keeps the step 1 rules note and starts on step 2", () => {
    renderPage();
    expect(screen.getByText(/pemisah koma atau titik koma/)).toBeInTheDocument();
    expect(screen.getByText(/maks\. 5 MiB dan 2\.000 baris/)).toBeInTheDocument();
    expect(screen.getByText(/pemisah koma atau titik koma/)).toHaveTextContent(
      /Kolom id kosong = data baru; terisi = ubah/,
    );
    expect(screen.getByText(/Excel menghapus angka nol di depan/)).toBeInTheDocument();
    expect(
      screen.getByText("Baris tidak bisa mengubah data dan status aktif sekaligus."),
    ).toBeInTheDocument();
    expect(screen.getByText(/2\. Unggah dan periksa/).closest("li")).toHaveAttribute(
      "aria-current",
      "step",
    );
  });

  it("shows the seven summary numbers, with Ubah taken from data_updates", async () => {
    dryRun.mockResolvedValue(
      preview({
        total_rows: 12,
        creates: 3,
        updates: 6,
        data_updates: 2,
        enables: 1,
        disables: 3,
        unchanged: 4,
        errors: [],
      }),
    );
    await upload();
    await screen.findByRole("button", { name: "Ajukan untuk persetujuan" });

    const stat = (label: string) => screen.getByText(label, { selector: "dt" }).parentElement;
    expect(stat("Total baris")).toHaveTextContent("12");
    expect(stat("Baru")).toHaveTextContent("3");
    expect(stat("Ubah")).toHaveTextContent("2");
    expect(stat("Aktifkan")).toHaveTextContent("1");
    expect(stat("Nonaktifkan")).toHaveTextContent("3");
    expect(stat("Tanpa perubahan")).toHaveTextContent("4");
    expect(stat("Kesalahan")).toHaveTextContent("0");
  });

  it("hides the change table and notes truncation when the check has errors", async () => {
    dryRun.mockResolvedValue(
      preview({
        errors: [{ row: 3, field: "code", message: "wajib diisi" }],
        errors_truncated: true,
      }),
    );
    await upload();

    expect(await screen.findByText("wajib diisi")).toBeInTheDocument();
    expect(screen.getByText(/Hanya sebagian kesalahan yang ditampilkan/)).toBeInTheDocument();
    expect(screen.queryByLabelText("Operasi")).toBeNull();
    expect(screen.queryByText("VND07")).toBeNull();
  });

  it("disables submit when there are no changes to submit", async () => {
    dryRun.mockResolvedValue(preview({ changes: [] }));
    await upload();

    const submit = await screen.findByRole("button", { name: "Ajukan untuk persetujuan" });
    expect(submit).toBeDisabled();
  });

  it("states the entity, file, counts and the no-change-yet sentence in the dialog", async () => {
    dryRun.mockResolvedValue(preview({ creates: 1, data_updates: 2, enables: 3, disables: 4 }));
    await upload();

    await userEvent.click(await screen.findByRole("button", { name: "Ajukan untuk persetujuan" }));

    expect(screen.getByText("Ajukan impor untuk persetujuan?")).toBeInTheDocument();
    expect(
      screen.getByText(
        /File: v\.csv\. Perubahan: 1 baru, 2 ubah, 3 aktifkan, 4 nonaktifkan\. Data belum berubah sampai pengajuan disetujui\./,
      ),
    ).toBeInTheDocument();
  });

  it("sends nothing when the dialog is dismissed", async () => {
    dryRun.mockResolvedValue(preview({}));
    await upload();

    await userEvent.click(await screen.findByRole("button", { name: "Ajukan untuk persetujuan" }));
    await userEvent.click(screen.getByRole("button", { name: "Batal" }));

    expect(confirm).not.toHaveBeenCalled();
  });

  it("does not submit twice while the request is pending", async () => {
    dryRun.mockResolvedValue(preview({}));
    confirm.mockReturnValue(new Promise(() => {}));
    await upload();

    await userEvent.click(await screen.findByRole("button", { name: "Ajukan untuk persetujuan" }));
    const submit = screen.getByRole("button", { name: "Ajukan" });
    await userEvent.click(submit);
    await userEvent.click(submit);

    expect(confirm).toHaveBeenCalledOnce();
  });

  it("drops the old result and returns to step 2 when another file is chosen", async () => {
    dryRun.mockResolvedValue(preview({}));
    confirm.mockResolvedValue({ batch_id: 9, rows: 2, approval_request_id: null, existing: false });
    await upload();
    await userEvent.click(await screen.findByRole("button", { name: "Ajukan untuk persetujuan" }));
    await userEvent.click(screen.getByRole("button", { name: "Ajukan" }));
    expect(await screen.findByText("#9")).toBeInTheDocument();

    await userEvent.upload(
      screen.getByLabelText("File CSV impor"),
      new File(["id,code\n,V2\n"], "w.csv", { type: "text/csv" }),
    );

    expect(screen.queryByText("#9")).toBeNull();
    expect(screen.queryByRole("button", { name: "Ajukan untuk persetujuan" })).toBeNull();
    expect(screen.getByText(/2\. Unggah dan periksa/).closest("li")).toHaveAttribute(
      "aria-current",
      "step",
    );
  });
});
