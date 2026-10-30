import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const apiMock = vi.hoisted(() => ({
  previewExport: vi.fn(),
  requestExport: vi.fn(),
  getExportJob: vi.fn(),
}));
vi.mock("../api", async (orig) => ({ ...(await orig<typeof import("../api")>()), ...apiMock }));

vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    Link: ({ to, children, ...rest }: { to: string; children: React.ReactNode }) => (
      <a href={to} {...rest}>
        {children}
      </a>
    ),
  };
});

vi.mock("@/features/admin-vendors/hooks", () => ({
  useVendorsList: () => ({ data: { vendors: [{ id: 3, name: "Vendor Tiga", code: "V3" }] } }),
  useVendorBranches: () => ({
    data: { branches: [{ id: 9, branch_name: "Cabang Sembilan", branch_code: "B9" }] },
  }),
}));

import { type DatasetId, findMasterDataset, parseFilterSearch } from "../datasets";
import type { ExportPreview } from "../types";
import { ExportPreviewPage } from "./ExportPreviewPage";

const COLUMNS = ["id", "vault_code", "min_capacity_amount", "is_active"];
const PAGE1: ExportPreview = {
  columns: COLUMNS,
  rows: [
    ["1", "V-01", "1000000", "true"],
    ["2", "V-02", "2000000", "false"],
  ],
  total: 1240,
  next_after: 2,
  limit: 50,
};
const PAGE2: ExportPreview = {
  columns: COLUMNS,
  rows: [["3", "V-03", "3000000", "true"]],
  total: 1240,
  next_after: null,
  limit: 50,
};

/** Serves PAGE1 for the first cursor and PAGE2 for `after = 2`. */
const servePages = (_params: unknown, after?: number) =>
  Promise.resolve(after === 2 ? PAGE2 : PAGE1);

const queuedJob = { id: 5, status: "queued", file_name: "f.csv", row_count: 0, kind: "cabang" };

/** Stands in for the route: owns the URL search and feeds the page its Applied_Filter. */
function Harness({ id, initial = {} }: { id: DatasetId; initial?: Record<string, unknown> }) {
  const [search, setSearch] = useState<Record<string, unknown>>(initial);
  const dataset = findMasterDataset(id);
  if (!dataset) throw new Error(`unknown dataset ${id}`);
  return (
    <ExportPreviewPage
      dataset={dataset}
      applied={parseFilterSearch(id, search)}
      onApplySearch={setSearch}
    />
  );
}

function renderPage(id: DatasetId = "vendor-vaults", initial?: Record<string, unknown>) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <Harness id={id} initial={initial} />
    </QueryClientProvider>,
  );
}

const exportButton = () => screen.getByRole("button", { name: /Ekspor\s+[\d.]+\s+baris/ });

beforeEach(() => {
  for (const m of Object.values(apiMock)) m.mockReset();
  apiMock.previewExport.mockImplementation(servePages);
  apiMock.getExportJob.mockResolvedValue(queuedJob);
});

describe("ExportPreviewPage table", () => {
  it("shows the CSV columns as-is, rows, total and caption", async () => {
    renderPage();
    await screen.findByText("V-01");
    const headers = screen.getAllByRole("columnheader").map((h) => h.textContent);
    expect(headers).toEqual(COLUMNS);
    expect(screen.getByText(/^Total/)).toHaveTextContent("Total 1.240 baris");
    expect(screen.getByRole("heading", { name: "Pratinjau Vault" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Kembali ke Export Data/ })).toHaveAttribute(
      "href",
      "/exports",
    );
  });

  it("renders is_active as icon + text badge, and money right-aligned in monospace", async () => {
    renderPage();
    await screen.findByText("V-01");
    const table = within(screen.getByRole("table"));
    expect(table.getByText("Aktif")).toBeInTheDocument();
    expect(table.getByText("Nonaktif")).toBeInTheDocument();
    const money = screen.getByText("1000000");
    expect(money).toHaveClass("font-mono");
    expect(money.closest("td")).toHaveClass("text-right", "tabular-nums");
    const header = screen.getByRole("columnheader", { name: "min_capacity_amount" });
    expect(header).toHaveClass("text-right");
    expect(screen.getByRole("columnheader", { name: "vault_code" })).toHaveClass("text-left");
  });

  it("pages with cursors: Berikutnya sends next_after, Sebelumnya goes back", async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("V-01");
    expect(screen.getByRole("button", { name: "Sebelumnya" })).toBeDisabled();

    await user.click(screen.getByRole("button", { name: "Berikutnya" }));
    await screen.findByText("V-03");
    expect(apiMock.previewExport.mock.calls.some((c) => c[1] === 2 && c[2] === 50)).toBe(true);
    expect(screen.getByText("Halaman 2")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Berikutnya" })).toBeDisabled();

    await user.click(screen.getByRole("button", { name: "Sebelumnya" }));
    await screen.findByText("V-01");
    expect(screen.getByText("Halaman 1")).toBeInTheDocument();
  });

  it("can be paged from the keyboard", async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("V-01");
    screen.getByRole("button", { name: "Berikutnya" }).focus();
    await user.keyboard("{Enter}");
    await screen.findByText("V-03");
  });

  it("applying a filter resets to the first page with the new filter", async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("V-01");
    await user.click(screen.getByRole("button", { name: "Berikutnya" }));
    await screen.findByText("V-03");

    await user.selectOptions(screen.getByLabelText("Status"), "disabled");
    await user.click(screen.getByRole("button", { name: "Terapkan" }));

    await waitFor(() => expect(screen.getByText("Halaman 1")).toBeInTheDocument());
    const last = apiMock.previewExport.mock.calls.at(-1);
    expect(last?.[0]).toMatchObject({ status: "disabled" });
    expect(last?.[1]).toBe(0);
  });

  it("shows the empty state and disables export when nothing matches", async () => {
    apiMock.previewExport.mockResolvedValue({ ...PAGE1, rows: [], total: 0, next_after: null });
    renderPage();
    await screen.findByText("Tidak ada data yang cocok dengan filter ini.");
    expect(exportButton()).toBeDisabled();
  });

  it("shows an error banner and 'Coba lagi' reloads the page", async () => {
    const user = userEvent.setup();
    apiMock.previewExport.mockRejectedValueOnce(new Error("boom"));
    renderPage();
    await screen.findByText("Pratinjau gagal dimuat");
    await user.click(screen.getByRole("button", { name: "Coba lagi" }));
    await screen.findByText("V-01");
    expect(screen.queryByText("Pratinjau gagal dimuat")).not.toBeInTheDocument();
  });
});

describe("ExportPreviewPage filters", () => {
  it("labels every filter and shows the cabang picker only once a vendor is chosen", async () => {
    const user = userEvent.setup();
    renderPage("package-prices", { vendorId: 3 });
    await screen.findByText("V-01");
    for (const label of ["Cari vendor", "Vendor", "Cabang", "Status", "Tingkat harga"]) {
      expect(screen.getByLabelText(label)).toBeInTheDocument();
    }
    await user.selectOptions(screen.getByLabelText("Vendor"), "");
    expect(screen.queryByLabelText("Cabang")).not.toBeInTheDocument();
  });

  it("asks for a vendor first and makes no API call while a required vendor is missing", async () => {
    renderPage("package-prices");
    expect(await screen.findByText(/Pilih vendor dulu/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Terapkan" })).toBeDisabled();
    expect(apiMock.previewExport).not.toHaveBeenCalled();
    expect(exportButton()).toBeDisabled();
  });
});

describe("ExportPreviewPage export", () => {
  it("blocks export while the draft differs from the applied filter", async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("V-01");
    expect(exportButton()).toBeEnabled();

    await user.selectOptions(screen.getByLabelText("Status"), "disabled");
    expect(exportButton()).toBeDisabled();
    expect(screen.getByText("Terapkan filter dulu")).toBeInTheDocument();
  });

  it("exports the applied filter (not the draft) and opens the progress dialog", async () => {
    const user = userEvent.setup();
    apiMock.requestExport.mockResolvedValue({ job: queuedJob, created: true });
    renderPage("vendor-vaults", { status: "disabled" });
    await screen.findByText("V-01");

    await user.click(exportButton());
    expect(apiMock.requestExport.mock.calls[0]?.[0]).toEqual({
      kind: "master_data_global",
      entity: "vendor-vaults",
      status: "disabled",
    });
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Ekspor data")).toBeInTheDocument();
  });

  it("shows a 429 inline without opening the dialog", async () => {
    const user = userEvent.setup();
    apiMock.requestExport.mockRejectedValue({
      status: 429,
      code: "too_many_exports",
      message: "too many",
    });
    renderPage();
    await screen.findByText("V-01");

    await user.click(exportButton());
    expect(await screen.findByText(/Terlalu banyak ekspor/)).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("disables export and explains why above one million rows", async () => {
    apiMock.previewExport.mockResolvedValue({ ...PAGE1, total: 1_000_001 });
    renderPage();
    await screen.findByText("V-01");
    expect(exportButton()).toBeDisabled();
    expect(screen.getByText(/Lebih dari 1\.000\.000 baris/)).toBeInTheDocument();
  });

  it("tells the user the file is fetched when the export is processed", async () => {
    renderPage();
    await screen.findByText("V-01");
    expect(
      screen.getByText(
        "File diambil saat ekspor diproses; perubahan data setelah tabel dimuat bisa ikut masuk.",
      ),
    ).toBeInTheDocument();
  });
});
