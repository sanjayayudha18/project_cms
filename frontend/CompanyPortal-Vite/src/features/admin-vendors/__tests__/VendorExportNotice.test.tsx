import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { AdminVendorsPage } from "../components/AdminVendorsPage";

vi.mock("../hooks", () => ({
  useVendorsList: () => ({
    data: { vendors: [], page: 1, page_size: 25, total: 0 },
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
  useDisableVendor: () => ({ mutate: vi.fn(), isPending: false }),
  useEnableVendor: () => ({ mutate: vi.fn(), isPending: false }),
  useCreateVendor: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpdateVendor: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

// Real ReportExportNotice; only the export flow is stubbed with a request error.
vi.mock("../../exports", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../exports")>();
  return {
    ...actual,
    useExportFlow: () => ({
      request: vi.fn(),
      isRequesting: false,
      requestError: new Error("Permintaan ekspor gagal"),
      dialogProps: {},
    }),
    ExportProgressDialog: () => null,
  };
});

vi.mock("../../master-data/pending", () => ({ usePendingEntityIds: () => new Set<number>() }));

vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    useNavigate: () => vi.fn(),
    useSearch: () => ({}),
    Link: ({ children }: { children: React.ReactNode }) => <a href="/x">{children}</a>,
  };
});

describe("AdminVendorsPage — export notice (Req 3.1–3.3)", () => {
  it("hides 'Lihat Ekspor Saya' but still shows the export request error", () => {
    render(<AdminVendorsPage />);

    expect(screen.queryByText(/Lihat Ekspor Saya/i)).toBeNull();
    expect(screen.getByText(/Permintaan ekspor gagal/)).toBeInTheDocument();
  });
});
