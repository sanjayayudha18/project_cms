import { useToastStore } from "@/lib/hooks/useToast";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ImportPreview } from "./api";
import { MasterDataIOPage } from "./components/MasterDataIOPage";

const downloadTemplate = vi.fn();
vi.mock("@tanstack/react-router", () => ({
  Link: ({
    to,
    params,
    children,
  }: { to: string; params?: Record<string, string>; children: React.ReactNode }) => (
    <a href={to.replace("$dataset", params?.dataset ?? "")}>{children}</a>
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
    useToastStore.setState({ toasts: [] });
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

  it("submits a clean file as one batch", async () => {
    dryRun.mockResolvedValue(preview({}));
    confirm.mockResolvedValue({ batch_id: 9, rows: 2, approval_request_id: 5, existing: false });
    await upload();

    await userEvent.click(await screen.findByRole("button", { name: "Ajukan untuk persetujuan" }));

    await waitFor(() => expect(confirm).toHaveBeenCalledOnce());
    await waitFor(() => expect(useToastStore.getState().toasts[0]?.message).toContain("batch #9"));
  });
});
