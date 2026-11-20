import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import * as api from "../api";
import { AdminPksTypesPage } from "../components/AdminPksTypesPage";
import type { PksType } from "../types";

const mockToast = vi.fn();

vi.mock("../api", () => ({
  listPksTypes: vi.fn(),
  submitPksType: vi.fn(),
  updatePksType: vi.fn(),
  disablePksType: vi.fn(),
  enablePksType: vi.fn(),
}));

vi.mock("@/lib/hooks/useToast", () => ({
  useToast: () => ({
    toast: mockToast,
    dismiss: vi.fn(),
    dismissAll: vi.fn(),
    toasts: [],
  }),
}));

vi.mock("@/components/ui/BackLink", () => ({
  BackLink: () => <div data-testid="back-link" />,
}));

vi.mock("@/features/master-data/components/PendingApprovalBadge", () => ({
  PendingApprovalBadge: () => null,
}));

function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
      mutations: {
        retry: false,
      },
    },
  });
}

describe("AdminPksTypesPage", () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    vi.clearAllMocks();
    queryClient = createTestQueryClient();
  });

  const mockTypes: PksType[] = [
    {
      id: 1,
      code: "MAIN",
      name: "Maintenance Rutin",
      is_active: true,
      in_use: true,
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    },
    {
      id: 2,
      code: "FLM",
      name: "First Line Maintenance",
      is_active: false,
      in_use: false,
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    },
  ];

  it("renders loading state then list of types", async () => {
    vi.mocked(api.listPksTypes).mockResolvedValueOnce(mockTypes);

    render(
      <QueryClientProvider client={queryClient}>
        <AdminPksTypesPage />
      </QueryClientProvider>,
    );

    expect(screen.getByLabelText("Memuat data type PKS")).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText("Maintenance Rutin")).toBeInTheDocument();
      expect(screen.getByText("First Line Maintenance")).toBeInTheDocument();
    });
  });

  it("renders error state and retries", async () => {
    vi.mocked(api.listPksTypes)
      .mockRejectedValueOnce(new Error("Network error"))
      .mockResolvedValueOnce(mockTypes);

    render(
      <QueryClientProvider client={queryClient}>
        <AdminPksTypesPage />
      </QueryClientProvider>,
    );

    await waitFor(() => {
      expect(screen.getByText("Gagal memuat daftar type PKS")).toBeInTheDocument();
    });

    const retryButton = screen.getByRole("button", { name: /coba lagi/i });
    await userEvent.click(retryButton);

    await waitFor(() => {
      expect(screen.getByText("Maintenance Rutin")).toBeInTheDocument();
    });
  });

  it("filters list by search term and status", async () => {
    vi.mocked(api.listPksTypes).mockResolvedValueOnce(mockTypes);

    render(
      <QueryClientProvider client={queryClient}>
        <AdminPksTypesPage />
      </QueryClientProvider>,
    );

    await waitFor(() => {
      expect(screen.getByText("Maintenance Rutin")).toBeInTheDocument();
    });

    const searchInput = screen.getByRole("searchbox", {
      name: /cari berdasarkan kode atau nama type pks/i,
    });
    await userEvent.type(searchInput, "FLM");

    expect(screen.queryByText("Maintenance Rutin")).not.toBeInTheDocument();
    expect(screen.getByText("First Line Maintenance")).toBeInTheDocument();
  });

  it("opens create dialog and submits successfully with 202 maker-checker feedback", async () => {
    vi.mocked(api.listPksTypes).mockResolvedValue(mockTypes);
    vi.mocked(api.submitPksType).mockResolvedValueOnce({
      change_request_id: 123,
      status: "pending_approval",
      entity_type: "vendor_pks_type",
      op: "create",
    });

    render(
      <QueryClientProvider client={queryClient}>
        <AdminPksTypesPage />
      </QueryClientProvider>,
    );

    await waitFor(() => {
      expect(screen.getByText("Maintenance Rutin")).toBeInTheDocument();
    });

    const addButton = screen.getByRole("button", { name: "Tambah Type PKS" });
    await userEvent.click(addButton);

    expect(screen.getByText("Tambah Type PKS", { selector: "#dialog-title" })).toBeInTheDocument();

    const codeInput = screen.getByLabelText(/^kode/i);
    const nameInput = screen.getByLabelText(/^nama/i);

    await userEvent.type(codeInput, "CIT");
    await userEvent.type(nameInput, "Cash in Transit");

    const submitBtn = screen.getByRole("button", { name: "Simpan" });
    await userEvent.click(submitBtn);

    await waitFor(() => {
      expect(api.submitPksType).toHaveBeenCalledWith({
        code: "CIT",
        name: "Cash in Transit",
      });
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          type: "success",
          message: expect.stringContaining("menunggu persetujuan (#123)"),
        }),
      );
    });
  });

  it("opens confirmation dialog for enable action and shows success toast", async () => {
    vi.mocked(api.listPksTypes).mockResolvedValue(mockTypes);
    vi.mocked(api.enablePksType).mockResolvedValueOnce({
      change_request_id: 456,
      status: "pending_approval",
      entity_type: "vendor_pks_type",
      op: "enable",
    });

    render(
      <QueryClientProvider client={queryClient}>
        <AdminPksTypesPage />
      </QueryClientProvider>,
    );

    await waitFor(() => {
      expect(screen.getByText("First Line Maintenance")).toBeInTheDocument();
    });

    const flmCell = screen.getByText("First Line Maintenance");
    const flmRow = flmCell.closest("tr");
    expect(flmRow).not.toBeNull();
    const enableButton = within(flmRow as HTMLElement).getByRole("button", {
      name: "Aktifkan",
    });
    await userEvent.click(enableButton);

    const dialog = screen.getByRole("dialog");
    expect(
      within(dialog).getByText(/apakah anda yakin ingin mengaktifkan type pks "FLM"\?/i),
    ).toBeInTheDocument();

    const confirmButton = within(dialog).getByRole("button", {
      name: "Aktifkan",
    });
    await userEvent.click(confirmButton);

    await waitFor(() => {
      expect(api.enablePksType).toHaveBeenCalledWith(2);
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          type: "success",
          message: expect.stringContaining("menunggu persetujuan (#456)"),
        }),
      );
    });
  });
});
