import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { BranchesPanel } from "../components/BranchesPanel";
import { PicsPanel } from "../components/PicsPanel";
import { VaultsPanel } from "../components/VaultsPanel";

// Server-side paging for vendor child lists (perbaikan-rbac G1): rows past
// the first 100 must stay reachable, e.g. ROH's 308 branches.

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children }: { children: React.ReactNode }) => <a href="/">{children}</a>,
}));

vi.mock("../../master-data/pending", () => ({
  usePendingEntityIds: () => new Set<number>(),
}));

const noopMutation = { mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false };

const useVendorBranchesMock = vi.fn();
const useVendorVaultsMock = vi.fn();
const useVendorPicsMock = vi.fn();

vi.mock("../hooks", () => ({
  useVendorBranches: (...args: unknown[]) => useVendorBranchesMock(...args),
  useVendorVaults: (...args: unknown[]) => useVendorVaultsMock(...args),
  useVendorPics: (...args: unknown[]) => useVendorPicsMock(...args),
  useCreateVendorBranch: () => noopMutation,
  useDisableVendorBranch: () => noopMutation,
  useEnableVendorBranch: () => noopMutation,
  useCreateVendorVault: () => noopMutation,
  useUpdateVendorVault: () => noopMutation,
  useDisableVendorVault: () => noopMutation,
  useEnableVendorVault: () => noopMutation,
  useCreateVendorPic: () => noopMutation,
  useUpdateVendorPic: () => noopMutation,
  useDisableVendorPic: () => noopMutation,
  useEnableVendorPic: () => noopMutation,
}));

function listResult(key: string, total: number) {
  return { isLoading: false, isError: false, data: { [key]: [], page: 1, page_size: 20, total } };
}

function lastParams(mock: ReturnType<typeof vi.fn>): Record<string, unknown> {
  return mock.mock.calls.at(-1)?.[1] as Record<string, unknown>;
}

beforeEach(() => {
  useVendorBranchesMock.mockReset().mockReturnValue(listResult("branches", 308));
  useVendorVaultsMock.mockReset().mockReturnValue(listResult("vaults", 45));
  useVendorPicsMock.mockReset().mockReturnValue({
    ...listResult("pics", 0),
    data: { pics: [], page: 1, page_size: 20, total: 0, warnings: [] },
  });
});

describe("BranchesPanel paging", () => {
  it("requests one server page and shows the server total, not a 100-row cap", () => {
    render(<BranchesPanel vendorId={1} />);

    expect(lastParams(useVendorBranchesMock)).toMatchObject({ page: 1, page_size: 20 });
    expect(screen.getByText("Halaman 1 dari 16 (308 cabang)")).toBeTruthy();
  });

  it("fetches the next server page on Berikutnya", async () => {
    render(<BranchesPanel vendorId={1} />);

    await userEvent.click(screen.getByRole("button", { name: "Berikutnya" }));

    expect(lastParams(useVendorBranchesMock)).toMatchObject({ page: 2 });
    expect(screen.getByRole("button", { name: "Sebelumnya" })).toBeEnabled();
  });

  it("sends the search to the server and goes back to page 1", async () => {
    render(<BranchesPanel vendorId={1} />);
    await userEvent.click(screen.getByRole("button", { name: "Berikutnya" }));

    await userEvent.type(screen.getByLabelText("Cari cabang"), "JKT");

    await waitFor(() => expect(lastParams(useVendorBranchesMock)).toMatchObject({ q: "JKT" }));
    expect(lastParams(useVendorBranchesMock)).toMatchObject({ page: 1 });
  });

  it("omits q when the search box is empty", () => {
    render(<BranchesPanel vendorId={1} />);
    expect(lastParams(useVendorBranchesMock)).not.toHaveProperty("q");
  });
});

describe("VaultsPanel paging", () => {
  it("pages within the selected branch", async () => {
    render(<VaultsPanel vendorId={1} branchId={7} />);
    expect(screen.getByText("Halaman 1 dari 3 (45 vault)")).toBeTruthy();

    await userEvent.click(screen.getByRole("button", { name: "Berikutnya" }));

    expect(lastParams(useVendorVaultsMock)).toMatchObject({ page: 2, branch_id: 7 });
  });
});

describe("PicsPanel scope", () => {
  it("vendor-wide asks only for PICs without a branch", () => {
    render(<PicsPanel vendorId={1} branchId={null} />);

    const params = lastParams(useVendorPicsMock);
    expect(params).toMatchObject({ vendor_wide_only: true, page: 1 });
    expect(params).not.toHaveProperty("branch_id");
  });

  it("branch scope asks for that branch only, never vendor-wide", () => {
    render(<PicsPanel vendorId={1} branchId={7} />);

    const params = lastParams(useVendorPicsMock);
    expect(params).toMatchObject({ branch_id: 7 });
    expect(params).not.toHaveProperty("vendor_wide_only");
  });

  it("disables both pager buttons when everything fits on one page", () => {
    render(<PicsPanel vendorId={1} branchId={7} />);

    expect(screen.getByRole("button", { name: "Sebelumnya" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Berikutnya" })).toBeDisabled();
  });
});
