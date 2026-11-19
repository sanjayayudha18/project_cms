/**
 * settings-menu-review 4.2 (Req 2.6, 2.7, 6.2): "Jenis vendor" filter on the admin Vendors list.
 */

import { api } from "@/lib/api/client";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { listVendors } from "../api";
import { AdminVendorsPage } from "../components/AdminVendorsPage";

const useVendorsListMock = vi.fn();
const navigateMock = vi.fn();
const exportRequestMock = vi.fn();
let searchState: Record<string, unknown> = {};

vi.mock("../hooks", () => ({
  useVendorsList: (...args: unknown[]) => useVendorsListMock(...args),
  usePksTypesList: () => ({ data: [], isLoading: false }),
  useDisableVendor: () => ({ mutate: vi.fn(), isPending: false }),
  useEnableVendor: () => ({ mutate: vi.fn(), isPending: false }),
  useCreateVendor: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpdateVendor: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("../../exports", () => ({
  useExportFlow: () => ({
    request: exportRequestMock,
    isRequesting: false,
    requestError: null,
    dialogProps: {},
  }),
  ExportProgressDialog: () => null,
  ReportExportNotice: () => null,
}));

vi.mock("../../master-data/pending", () => ({ usePendingEntityIds: () => new Set<number>() }));

vi.mock("@/lib/api/client", () => ({ api: { get: vi.fn() } }));

vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    useNavigate: () => navigateMock,
    useSearch: () => searchState,
    Link: ({ children }: { children: React.ReactNode }) => <a href="/x">{children}</a>,
  };
});

beforeEach(() => {
  useVendorsListMock.mockReset();
  navigateMock.mockReset();
  exportRequestMock.mockReset();
  searchState = {};
  useVendorsListMock.mockReturnValue({
    data: { vendors: [], page: 1, page_size: 25, total: 0 },
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  });
});

describe("Jenis vendor filter", () => {
  it("offers Semua, Vendor FLM and ROH", () => {
    render(<AdminVendorsPage />);

    const options = within(screen.getByLabelText("Jenis vendor"))
      .getAllByRole("option")
      .map((o) => o.textContent);
    expect(options).toEqual(["Semua", "Vendor FLM", "ROH"]);
  });

  it("choosing ROH puts kind=INTERNAL in the URL and resets page to 1", async () => {
    searchState = { page: 3 };
    const user = userEvent.setup();
    render(<AdminVendorsPage />);

    await user.selectOptions(screen.getByLabelText("Jenis vendor"), "INTERNAL");

    // page 1 is the default, so it is omitted from the search
    expect(navigateMock).toHaveBeenLastCalledWith({ to: ".", search: { kind: "INTERNAL" } });
  });

  it("passes kind from the URL to the list request", () => {
    searchState = { kind: "INTERNAL" };
    render(<AdminVendorsPage />);

    expect(useVendorsListMock).toHaveBeenCalledWith(expect.objectContaining({ kind: "INTERNAL" }));
  });

  it("PKS export does not carry kind (Q9)", async () => {
    searchState = { kind: "INTERNAL", q: "abc" };
    const user = userEvent.setup();
    render(<AdminVendorsPage />);

    await user.click(screen.getByRole("button", { name: /Ekspor CSV/ }));

    expect(exportRequestMock).toHaveBeenCalledTimes(1);
    const arg = exportRequestMock.mock.calls[0][0];
    expect(arg.report).toBe("vendor-pks");
    expect(arg.params).not.toHaveProperty("kind");
  });
});

describe("listVendors", () => {
  it("sends kind only when set", async () => {
    vi.mocked(api.get).mockResolvedValue({ data: { vendors: [] } });

    await listVendors({ page: 1, page_size: 25, kind: "INTERNAL" });
    expect(vi.mocked(api.get).mock.calls[0][0]).toContain("kind=INTERNAL");

    await listVendors({ page: 1, page_size: 25 });
    expect(vi.mocked(api.get).mock.calls[1][0]).not.toContain("kind=");
  });
});
