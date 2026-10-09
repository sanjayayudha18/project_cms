/**
 * cit-acm-plan S7: penetapan vault UI. Hooks in ./api are mocked (no live
 * backend); the helpers (formatDenoms, labels) stay real.
 */

import { useAuthStore } from "@/lib/auth/store";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { RequestVaultPanel } from "./RequestVaultPanel";
import { VaultPlanDetail } from "./VaultPlanDetail";
import { VaultPlanList } from "./VaultPlanList";
import type { VaultPlanDetail as Plan } from "./api";

const mutate = vi.fn();
const decide = vi.fn();
const usePlan = vi.fn();
const listFilter = vi.fn();

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
    useVaultCandidates: (_p: number, _t: string, urgent: boolean) => ({
      isLoading: false,
      data: [
        {
          vendor_branch_id: 9,
          branch_code: "BJK-01",
          branch_name: "Bijak Bekasi",
          vendor_name: "Bijak",
          region_code: "JKT",
          category: "CASH",
          tier: 1,
          saldo_known: true,
          saldo: { "100000": "900000000" },
          capacity: { "100000": "100000" },
          capacity_warning: true,
        },
        ...(urgent
          ? [
              {
                vendor_branch_id: 11,
                branch_code: "SBY-01",
                branch_name: "Surabaya",
                vendor_name: "Bijak",
                region_code: "SBY",
                category: "CASH",
                tier: 3,
                saldo_known: false,
                saldo: null,
                capacity: null,
                capacity_warning: true,
              },
            ]
          : []),
      ],
    }),
    useRequestVaultReview: () => ({
      data: {
        plans: [
          {
            id: 1,
            acm_area_id: 2,
            acm_area_name: "Jabo",
            status: "acm_approved",
            rejection_reason: null,
          },
        ],
        assignments: [
          {
            vault_plan_id: 1,
            terminal_id: "T001",
            replenish_branch: { id: 5, code: "BJK-02" },
            vault_branch: { id: 9, code: "BJK-01", name: "Bijak Bekasi", vendor_name: "Bijak" },
            tier: 1,
            is_urgent: false,
            urgent_reason: null,
            saldo_snapshot: { "100000": "900000000" },
            capacity_snapshot: { "100000": "1500000" },
            capacity_warning: false,
          },
        ],
      },
    }),
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

function plan(overrides: Partial<Plan> = {}): Plan {
  return {
    id: 7,
    vendor_request_id: 42,
    request_number: "REP-BJK-JKT-20261009-001",
    replenish_date: "2026-10-09",
    request_status: "vault_assignment",
    vault_rejection_reason: null,
    acm_area_id: 2,
    acm_area_name: "Jabo",
    status: "draft",
    submitted_by: null,
    approved_by: null,
    rejection_reason: null,
    atms: [
      {
        terminal_id: "T001",
        replenish_branch: BRANCH,
        order: { "100000": "1500000" },
        assignment: null,
      },
    ],
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

beforeEach(() => {
  mutate.mockReset();
  decide.mockReset();
});

describe("VaultPlanDetail", () => {
  it("ACM-USER picks a tiered candidate, sees the capacity warning and saves", async () => {
    setRole(1, "ACM-USER");
    usePlan.mockReturnValue({ isLoading: false, isError: false, data: plan() });
    render(<VaultPlanDetail />);

    expect(screen.getByText("100K 1.500.000")).toBeInTheDocument();
    // Submit needs every ATM saved first.
    expect(screen.getByRole("button", { name: "Kirim untuk Approval" })).toBeDisabled();

    // FR4.2: the top Tier 1 candidate is pre-selected for an unassigned ATM.
    expect(screen.getByLabelText("Branch vault untuk T001")).toHaveValue("9");
    expect(screen.getByText("Kapasitas kurang dari order")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));

    expect(mutate).toHaveBeenCalledWith(
      {
        op: "save",
        id: 7,
        assignments: [
          { terminal_id: "T001", vault_branch_id: 9, is_urgent: false, urgent_reason: null },
        ],
      },
      expect.anything(),
    );
  });

  it("urgent unlocks tier 3 and needs a 10-500 char reason before saving", async () => {
    setRole(1, "ACM-USER");
    usePlan.mockReturnValue({ isLoading: false, isError: false, data: plan() });
    render(<VaultPlanDetail />);

    await userEvent.click(screen.getByLabelText(/Urgent/));
    await userEvent.selectOptions(screen.getByLabelText("Branch vault untuk T001"), "11");
    const save = screen.getByRole("button", { name: "Simpan" });
    expect(save).toBeDisabled();

    await userEvent.type(screen.getByLabelText("Alasan urgent untuk T001"), "vault lokal kosong");
    expect(save).toBeEnabled();
    await userEvent.click(save);
    expect(mutate.mock.calls[0][0].assignments[0]).toEqual({
      terminal_id: "T001",
      vault_branch_id: 11,
      is_urgent: true,
      urgent_reason: "vault lokal kosong",
    });
  });

  it("ACM-SPV approves a submitted plan but not their own submission", async () => {
    setRole(2, "ACM-SPV");
    usePlan.mockReturnValue({
      isLoading: false,
      isError: false,
      data: plan({ status: "pending_acm_approval", submitted_by: 1 }),
    });
    const { unmount } = render(<VaultPlanDetail />);
    expect(screen.queryByLabelText("Branch vault untuk T001")).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Setujui" }));
    expect(mutate.mock.calls[0][0]).toEqual({ op: "approve", id: 7 });
    unmount();

    setRole(1, "ACM-SPV");
    render(<VaultPlanDetail />);
    expect(screen.queryByRole("button", { name: "Setujui" })).not.toBeInTheDocument();
  });

  it("shows the ATM-SPV rejection reason on a plan sent back to draft", () => {
    setRole(1, "ACM-USER");
    usePlan.mockReturnValue({
      isLoading: false,
      isError: false,
      data: plan({ vault_rejection_reason: "vault terlalu jauh" }),
    });
    render(<VaultPlanDetail />);
    expect(screen.getByText("vault terlalu jauh")).toBeInTheDocument();
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
  it("lists assignments and lets the checker approve", async () => {
    render(<RequestVaultPanel requestId={42} canReview />);
    expect(screen.getByText("100K 1.500.000")).toBeInTheDocument();
    expect(screen.getByText("100K 900.000.000")).toBeInTheDocument(); // saldo snapshot
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
    expect(decide.mock.calls[0][0]).toEqual({
      id: 42,
      approve: false,
      reason: "kapasitas kurang",
    });
  });

  it("hides actions when review is not open", () => {
    render(<RequestVaultPanel requestId={42} canReview={false} />);
    expect(screen.queryByRole("button", { name: "Setujui Penetapan" })).not.toBeInTheDocument();
  });
});
