/**
 * cit-vault-select S5: penetapan vault UI (vault rows per request). Hooks in
 * ./api are mocked (no live backend); the helpers (formatDenoms, labels) and
 * ./allocation rules stay real.
 */

import { useAuthStore } from "@/lib/auth/store";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { RequestVaultPanel } from "./RequestVaultPanel";
import { VaultPlanDetail } from "./VaultPlanDetail";
import { VaultPlanList } from "./VaultPlanList";
import { areaLabel } from "./allocation";
import type { VaultPlanDetail as Plan, VaultAllocationView } from "./api";

const mutate = vi.fn();
const decide = vi.fn();
const usePlan = vi.fn();
const listFilter = vi.fn();
const review = vi.fn();

vi.mock("./api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("./api")>();
  return {
    ...actual,
    useVaultPlan: () => usePlan(),
    useVaultPlans: (filter: unknown) => {
      listFilter(filter);
      return { isLoading: false, isError: false, data: [] };
    },
    useVaultPlanMutation: () => ({ mutate, isPending: false }),
    useVaultCandidates: () => ({
      isLoading: false,
      isError: false,
      data: [
        {
          vendor_branch_id: 9,
          branch_code: "BJK-01",
          branch_name: "Bijak Bekasi",
          vendor_id: 1,
          vendor_name: "Bijak",
          region_code: "JKT",
          tier: 1,
          vaults: [
            {
              vendor_vault_id: 91,
              vault_code: "V-BJK-1",
              category: "CASH",
              tier: 1,
              saldo_known: true,
              saldo: { "100000": "900000000" },
              capacity: { "100000": "1000000" },
            },
            {
              vendor_vault_id: 92,
              vault_code: "V-BJK-2",
              category: "ATM_CASH",
              tier: 1,
              saldo_known: true,
              saldo: { "100000": "900000000" },
              capacity: { "100000": "5000000" },
            },
          ],
        },
        {
          vendor_branch_id: 11,
          branch_code: "SBY-01",
          branch_name: "Surabaya",
          vendor_id: 1,
          vendor_name: "Bijak",
          region_code: "SBY",
          tier: 3,
          vaults: [
            {
              vendor_vault_id: 111,
              vault_code: "V-SBY-1",
              category: "CASH",
              tier: 3,
              saldo_known: false,
              saldo: null,
              capacity: null,
            },
          ],
        },
      ],
    }),
    useRequestVaultReview: () => review(),
    useRequestVaultDecision: () => ({ mutate: decide, isPending: false }),
  };
});

vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    useParams: () => ({ id: "7" }),
    Link: ({ children }: { children?: ReactNode }) => <a href="/cit/vault-plans">{children}</a>,
  };
});

const BRANCH = {
  id: 5,
  code: "BJK-02",
  name: "Bijak Jakarta",
  vendor_id: 1,
  vendor_name: "Bijak",
  region_code: "JKT",
};

function allocation(overrides: Partial<VaultAllocationView> = {}): VaultAllocationView {
  return {
    id: 1,
    vendor_vault_id: 91,
    vault_code: "V-BJK-1",
    vault_category: "CASH",
    branch: { ...BRANCH, id: 9, code: "BJK-01", name: "Bijak Bekasi" },
    tier: 1,
    is_urgent: false,
    urgent_reason: null,
    amounts: { "100000": "1500000" },
    saldo_snapshot: { "100000": "900000000" },
    capacity_snapshot: { "100000": "1000000" },
    capacity_warning: true,
    ticket_number: null,
    ...overrides,
  };
}

function plan(overrides: Partial<Plan> = {}): Plan {
  return {
    id: 7,
    vendor_request_id: 42,
    request_number: "REP-BJK-JKT-20261013-001",
    replenish_date: "2026-10-13",
    region_code: "JKT",
    request_status: "vault_assignment",
    vault_rejection_reason: null,
    rejected_by_vendor: false,
    acm_area_id: 2,
    acm_area_name: "Jabo",
    status: "draft",
    submitted_by: null,
    approved_by: null,
    rejection_reason: null,
    buffer_reason: null,
    atms: [{ terminal_id: "T001", replenish_branch: BRANCH, order: { "100000": "1500000" } }],
    order_totals: { "100000": "1500000" },
    allocations: [],
    vault_tickets: [],
    ...overrides,
  };
}

function setRole(id: number, role: string) {
  useAuthStore.setState({
    user: {
      id,
      username: "u",
      fullName: "U",
      email: "u@x",
      role: role as never,
      isKaryawan: true,
      vendorId: null,
    },
  });
}

function showPlan(p: Plan) {
  usePlan.mockReturnValue({ isLoading: false, isError: false, data: p });
  return render(<VaultPlanDetail />);
}

beforeEach(() => {
  mutate.mockReset();
  decide.mockReset();
});

describe("VaultPlanDetail editor", () => {
  it("adds and removes vault rows, checks multiples, and a short denom blocks submit", async () => {
    setRole(1, "ACM-USER");
    showPlan(plan());
    const submit = screen.getByRole("button", { name: "Kirim untuk Approval" });
    expect(submit).toBeDisabled();
    expect(screen.getByText("Lengkapi setiap baris vault.")).toBeInTheDocument();

    await userEvent.selectOptions(screen.getByLabelText("Branch vault baris 1"), "9");
    await userEvent.selectOptions(screen.getByLabelText("Vault baris 1"), "91");
    const amount1 = screen.getByLabelText("Nominal 100K baris 1");
    await userEvent.type(amount1, "1000050");
    expect(screen.getByText("Harus kelipatan Rp100.000")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Simpan" })).toBeDisabled();

    await userEvent.clear(amount1);
    await userEvent.type(amount1, "01000000");
    expect(amount1).toHaveValue("1000000"); // leading zero dropped, matches the server's value
    expect(screen.getByText("Kurang")).toBeInTheDocument();
    expect(screen.getByText("Alokasi 100K masih kurang dari order.")).toBeInTheDocument();
    expect(submit).toBeDisabled();

    // Second row: the vault already used on row 1 is not offered, the only free one is picked.
    await userEvent.click(screen.getByRole("button", { name: "Tambah Vault" }));
    await userEvent.selectOptions(screen.getByLabelText("Branch vault baris 2"), "9");
    expect(screen.getByLabelText("Vault baris 2")).toHaveValue("92");
    expect(within(screen.getByLabelText("Vault baris 2")).queryByText(/V-BJK-1/)).toBeNull();
    await userEvent.type(screen.getByLabelText("Nominal 100K baris 2"), "500000");
    expect(screen.getByText("Sesuai")).toBeInTheDocument();
    // Saved state differs from the draft, so submit waits for Simpan.
    expect(screen.getByText("Simpan perubahan sebelum mengirim.")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));
    expect(mutate.mock.calls[0][0]).toEqual({
      op: "save",
      id: 7,
      bufferReason: null,
      allocations: [
        {
          vendor_vault_id: 91,
          amounts: { "100000": "1000000" },
          is_urgent: false,
          urgent_reason: null,
        },
        {
          vendor_vault_id: 92,
          amounts: { "100000": "500000" },
          is_urgent: false,
          urgent_reason: null,
        },
      ],
    });

    await userEvent.click(screen.getByRole("button", { name: "Hapus baris 2" }));
    expect(screen.queryByLabelText("Vault baris 2")).not.toBeInTheDocument();
    expect(screen.getByText("Kurang")).toBeInTheDocument();
  });

  it("an excess is a buffer: the reason field appears and is required to submit", async () => {
    setRole(1, "ACM-USER");
    showPlan(plan({ allocations: [allocation({ amounts: { "100000": "2000000" } })] }));
    expect(screen.getByLabelText("Vault baris 1")).toHaveValue("91");
    expect(screen.getByText("Buffer")).toBeInTheDocument();
    expect(screen.getByText("Kapasitas vault kurang dari alokasi")).toBeInTheDocument();
    expect(screen.getByText("Isi alasan buffer (10-500 karakter).")).toBeInTheDocument();

    await userEvent.type(screen.getByLabelText(/Alasan buffer/), "antisipasi penarikan gajian");
    expect(screen.getByText("Simpan perubahan sebelum mengirim.")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));
    expect(mutate.mock.calls[0][0].bufferReason).toBe("antisipasi penarikan gajian");
  });

  it("a saved, covered plan can be submitted and shows its vault ticket", async () => {
    setRole(1, "ACM-USER");
    showPlan(plan({ allocations: [allocation({ ticket_number: "V-BJK-1_20261013_001" })] }));
    expect(screen.getByText("Tiket V-BJK-1_20261013_001")).toBeInTheDocument();
    expect(screen.queryByLabelText(/Alasan buffer/)).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Kirim untuk Approval" }));
    expect(mutate.mock.calls[0][0]).toEqual({ op: "submit", id: 7 });
  });

  it("urgent unlocks tier 3 and needs a 10-500 char reason; un-urgent drops the tier 3 branch", async () => {
    setRole(1, "ACM-USER");
    showPlan(plan());
    const branch = screen.getByLabelText("Branch vault baris 1");
    expect(within(branch).queryByText(/SBY-01/)).toBeNull();

    await userEvent.click(screen.getByLabelText(/Urgent/));
    await userEvent.selectOptions(branch, "11");
    expect(screen.getByLabelText("Vault baris 1")).toHaveValue("111");
    await userEvent.type(screen.getByLabelText("Nominal 100K baris 1"), "1500000");
    expect(screen.getByText("Saldo vault tidak diketahui")).toBeInTheDocument();
    const save = screen.getByRole("button", { name: "Simpan" });
    expect(save).toBeDisabled();

    await userEvent.type(screen.getByLabelText("Alasan urgent baris 1"), "vault lokal kosong");
    expect(save).toBeEnabled();
    await userEvent.click(save);
    expect(mutate.mock.calls[0][0].allocations[0]).toEqual({
      vendor_vault_id: 111,
      amounts: { "100000": "1500000" },
      is_urgent: true,
      urgent_reason: "vault lokal kosong",
    });

    await userEvent.click(screen.getByLabelText(/Urgent/));
    expect(branch).toHaveValue("");
  });
});

describe("VaultPlanDetail decisions", () => {
  it("ACM-SPV sees the saved vaults read-only and approves, but not their own submission", async () => {
    setRole(2, "ACM-SPV");
    const { unmount } = showPlan(
      plan({
        status: "pending_acm_approval",
        submitted_by: 1,
        allocations: [allocation({ ticket_number: "V-BJK-1_20261013_001" })],
      }),
    );
    expect(screen.queryByLabelText("Vault baris 1")).not.toBeInTheDocument();
    expect(screen.getByText("V-BJK-1_20261013_001")).toBeInTheDocument();
    expect(screen.getByText("Peringatan kapasitas")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Setujui" }));
    expect(mutate.mock.calls[0][0]).toEqual({ op: "approve", id: 7 });
    unmount();

    setRole(1, "ACM-SPV");
    render(<VaultPlanDetail />);
    expect(screen.queryByRole("button", { name: "Setujui" })).not.toBeInTheDocument();
  });

  it("shows the ATM-SPV rejection reason on a plan sent back to draft", () => {
    setRole(1, "ACM-USER");
    showPlan(plan({ vault_rejection_reason: "vault terlalu jauh" }));
    expect(screen.getByText("vault terlalu jauh")).toBeInTheDocument();
  });

  // cit-send-vendor FR4.3: a vault vendor's rejection is labelled as such, not as ACM-SPV.
  it("labels a plan sent back by a vault vendor", () => {
    setRole(1, "ACM-USER");
    showPlan(plan({ rejection_reason: "saldo vault tidak cukup", rejected_by_vendor: true }));
    expect(screen.getByText(/Ditolak vendor \(branch vault\)/)).toBeInTheDocument();
    expect(screen.getByText("saldo vault tidak cukup")).toBeInTheDocument();
    expect(screen.queryByText("Ditolak ACM-SPV")).not.toBeInTheDocument();
  });
});

describe("VaultPlanList", () => {
  it("sends status and replenish date range to the list query", async () => {
    render(<VaultPlanList />);
    expect(listFilter).toHaveBeenLastCalledWith({ status: "draft", from: "", to: "" });
    await userEvent.type(screen.getByLabelText("Replenish dari"), "2026-10-01");
    expect(listFilter).toHaveBeenLastCalledWith({ status: "draft", from: "2026-10-01", to: "" });
  });
});

describe("RequestVaultPanel", () => {
  beforeEach(() => {
    review.mockReturnValue({
      data: plan({
        status: "acm_approved",
        allocations: [allocation({ ticket_number: "V-BJK-1_20261013_001" })],
      }),
    });
  });

  it("lists the request's vaults with order coverage and lets the checker approve", async () => {
    render(<RequestVaultPanel requestId={42} canReview />);
    expect(screen.getByText("V-BJK-1_20261013_001")).toBeInTheDocument();
    expect(screen.getByText("Sesuai")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Setujui Penetapan" }));
    expect(decide.mock.calls[0][0]).toEqual({ id: 42, approve: true, reason: undefined });
  });

  it("reject requires a reason", async () => {
    render(<RequestVaultPanel requestId={42} canReview />);
    await userEvent.click(screen.getByRole("button", { name: "Tolak Penetapan" }));
    const confirm = screen.getByRole("button", { name: "Kembalikan ke ACM" });
    expect(confirm).toBeDisabled();
    await userEvent.type(screen.getByLabelText("Alasan Penolakan"), "kapasitas kurang");
    await userEvent.click(confirm);
    expect(decide.mock.calls[0][0]).toEqual({ id: 42, approve: false, reason: "kapasitas kurang" });
  });

  it("hides actions when review is not open, and renders nothing without a plan", () => {
    const { unmount } = render(<RequestVaultPanel requestId={42} canReview={false} />);
    expect(screen.queryByRole("button", { name: "Setujui Penetapan" })).not.toBeInTheDocument();
    unmount();
    review.mockReturnValue({ data: null });
    const { container } = render(<RequestVaultPanel requestId={42} canReview />);
    expect(container).toBeEmptyDOMElement();
  });
});

describe("areaLabel", () => {
  it("does not double the Area prefix", () => {
    expect(areaLabel("Jabo")).toBe("Area Jabo");
    expect(areaLabel("Area ACM Dev")).toBe("Area ACM Dev");
    expect(areaLabel("area timur")).toBe("area timur");
    expect(areaLabel("Areal Barat")).toBe("Area Areal Barat");
  });
});
