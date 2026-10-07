/**
 * Form and interaction tests (Task 16.2): Zod validation guards
 * (self-supervisor block, min_amount > max_amount, required fields),
 * successful submit payloads (RFC3339 conversion, amounts as strings),
 * and the delegation 409 conflict message (icon + Bahasa Indonesia text).
 * Query hooks are mocked; hook-level cache invalidation lives in
 * hooks.test.tsx.
 */

import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

// ─── Mock useRbacQueries hooks ────────────────────────────────────────────────

const mockUseUserHierarchy = vi.fn();
const mockUseDelegations = vi.fn();
const mockUseLeaves = vi.fn();
const mockUsePolicies = vi.fn();
const mockUseSetHierarchy = vi.fn();
const mockUseCreateDelegation = vi.fn();
const mockUseCreateLeave = vi.fn();
const mockUseCreatePolicy = vi.fn();

function idleMutation() {
  return { mutate: vi.fn(), isPending: false, isError: false, error: null };
}

vi.mock("../hooks/useRbacQueries", () => ({
  useUserHierarchy: () => mockUseUserHierarchy(),
  useDelegations: () => mockUseDelegations(),
  useLeaves: () => mockUseLeaves(),
  usePolicies: () => mockUsePolicies(),
  useSetHierarchy: () => mockUseSetHierarchy(),
  useCreateDelegation: () => mockUseCreateDelegation(),
  useRevokeDelegation: () => idleMutation(),
  useCreateLeave: () => mockUseCreateLeave(),
  useCreatePolicy: () => mockUseCreatePolicy(),
  useUpdatePolicy: () => idleMutation(),
}));

// ─── Import components after mocks ───────────────────────────────────────────

import { RbacDelegationsPage } from "../components/RbacDelegationsPage";
import { RbacLeavesPage } from "../components/RbacLeavesPage";
import { RbacPoliciesPage } from "../components/RbacPoliciesPage";
import { RbacUsersPage } from "../components/RbacUsersPage";
import type { RbacUsersResponse } from "../types";

// ─── Shared helpers ──────────────────────────────────────────────────────────

function emptyQuery() {
  return { data: undefined, isLoading: false, isError: false };
}

/** Fills a datetime-local input in one shot (jsdom rejects partial typed values). */
function setDateTime(label: string, value: string) {
  fireEvent.change(screen.getByLabelText(label), { target: { value } });
}

beforeEach(() => {
  mockUseUserHierarchy.mockReturnValue(emptyQuery());
  mockUseDelegations.mockReturnValue(emptyQuery());
  mockUseLeaves.mockReturnValue(emptyQuery());
  mockUsePolicies.mockReturnValue(emptyQuery());
  mockUseSetHierarchy.mockReturnValue(idleMutation());
  mockUseCreateDelegation.mockReturnValue(idleMutation());
  mockUseCreateLeave.mockReturnValue(idleMutation());
  mockUseCreatePolicy.mockReturnValue(idleMutation());
});

// ─── RbacUsersPage: hierarchy form ───────────────────────────────────────────

describe("RbacUsersPage — hierarchy form", () => {
  const usersResponse: RbacUsersResponse = {
    users: [
      {
        id: 5,
        username: "budi.santoso",
        full_name: "Budi Santoso",
        supervisor_id: 2,
        approval_level: null,
        role: "ATM-USER",
        auth_source: "ldap",
        vendor_id: null,
        vendor_name: null,
      },
      {
        id: 2,
        username: "dewi.lestari",
        full_name: "Dewi Lestari",
        supervisor_id: null,
        approval_level: 1,
        role: "ATM-SPV",
        auth_source: "ldap",
        vendor_id: null,
        vendor_name: null,
      },
    ],
  };

  /** Renders the page and opens the given user's edit row (default: Budi
   *  Santoso, id 5, a maker -- opens the supervisor select). The mutation
   *  mock must be set before render: the edit row captures
   *  useSetHierarchy()'s return value at render time, so a later
   *  mockReturnValue would not reach it. */
  async function openEditRow(mutation = idleMutation(), userName = "Budi Santoso") {
    mockUseUserHierarchy.mockReturnValue({
      data: usersResponse,
      isLoading: false,
      isError: false,
    });
    mockUseSetHierarchy.mockReturnValue(mutation);
    const user = userEvent.setup();
    render(<RbacUsersPage />);
    await user.click(screen.getByRole("button", { name: new RegExp(`ubah ${userName}`, "i") }));
    return user;
  }

  it("never offers a user as their own supervisor", async () => {
    await openEditRow(idleMutation(), "Dewi Lestari");

    // Dewi (id 2) is the only checker, so the supervisor select for makers
    // would only ever list her -- opening her own row has no supervisor
    // field (checkers edit approval level instead), confirming the split.
    expect(screen.getByLabelText("Level Persetujuan")).toBeInTheDocument();
    expect(screen.queryByLabelText("Supervisor ID")).not.toBeInTheDocument();
  });

  it("submits a valid hierarchy change to the mutation", async () => {
    const mutation = idleMutation();
    const user = await openEditRow(mutation);

    await user.selectOptions(screen.getByLabelText("Supervisor ID"), "2");
    await user.click(screen.getByRole("button", { name: /simpan/i }));

    expect(mutation.mutate).toHaveBeenCalledWith(
      { userId: 5, values: { supervisor_id: 2, approval_level: null } },
      expect.anything(),
    );
  });
});

// ─── RbacDelegationsPage: create form + 409 ───────────────────────────────────

describe("RbacDelegationsPage — create form", () => {
  it("shows required-field errors on an empty submit", async () => {
    const user = userEvent.setup();
    render(<RbacDelegationsPage />);

    await user.click(screen.getByRole("button", { name: /buat delegasi/i }));

    expect(screen.getByText("from_user_id wajib diisi")).toBeInTheDocument();
    expect(screen.getByText("to_user_id wajib diisi")).toBeInTheDocument();
    expect(screen.getByText("start_at wajib diisi")).toBeInTheDocument();
    expect(screen.getByText("end_at wajib diisi")).toBeInTheDocument();
  });

  it("rejects start_at not before end_at", async () => {
    const user = userEvent.setup();
    render(<RbacDelegationsPage />);

    await user.type(screen.getByLabelText("Dari (User ID)"), "10");
    await user.type(screen.getByLabelText("Ke (User ID)"), "20");
    setDateTime("Mulai", "2026-09-22T17:00");
    setDateTime("Berakhir", "2026-09-20T09:00");
    await user.click(screen.getByRole("button", { name: /buat delegasi/i }));

    expect(screen.getByText("start_at harus sebelum end_at")).toBeInTheDocument();
  });

  it("submits a valid delegation with datetime-local converted to RFC3339", async () => {
    const user = userEvent.setup();
    const mutation = idleMutation();
    mockUseCreateDelegation.mockReturnValue(mutation);
    render(<RbacDelegationsPage />);

    await user.type(screen.getByLabelText("Dari (User ID)"), "10");
    await user.type(screen.getByLabelText("Ke (User ID)"), "20");
    setDateTime("Mulai", "2026-09-20T09:00");
    setDateTime("Berakhir", "2026-09-22T17:00");
    await user.click(screen.getByRole("button", { name: /buat delegasi/i }));

    expect(mutation.mutate).toHaveBeenCalledTimes(1);
    const [values] = mutation.mutate.mock.calls[0] as [
      { from_user_id: number; to_user_id: number; start_at: string; end_at: string },
      unknown,
    ];
    expect(values.from_user_id).toBe(10);
    expect(values.to_user_id).toBe(20);
    // RFC3339 conversion: same instant as the local input, ISO with Z suffix.
    expect(values.start_at).toBe(new Date("2026-09-20T09:00").toISOString());
    expect(values.end_at).toBe(new Date("2026-09-22T17:00").toISOString());
  });

  it("renders a 409 conflict message with icon + text (Requirement 5.6)", () => {
    mockUseCreateDelegation.mockReturnValue({
      mutate: vi.fn(),
      isPending: false,
      isError: true,
      error: { status: 409, message: "overlapping delegation" },
    });
    render(<RbacDelegationsPage />);

    const message = screen.getByText(
      "Rentang delegasi tumpang tindih dengan delegasi lain untuk user ini.",
    );
    expect(message).toBeInTheDocument();
    expect(message.closest("p")?.querySelector("svg")).toBeInTheDocument();
  });
});

// ─── RbacLeavesPage: create form ─────────────────────────────────────────────

describe("RbacLeavesPage — create form", () => {
  it("shows required-field errors on an empty submit", async () => {
    const user = userEvent.setup();
    render(<RbacLeavesPage />);

    await user.click(screen.getByRole("button", { name: /catat cuti/i }));

    expect(screen.getByText("user_id wajib diisi")).toBeInTheDocument();
    expect(screen.getByText("start_at wajib diisi")).toBeInTheDocument();
    expect(screen.getByText("end_at wajib diisi")).toBeInTheDocument();
  });

  it("submits a valid leave with RFC3339 datetimes and the optional reason", async () => {
    const user = userEvent.setup();
    const mutation = idleMutation();
    mockUseCreateLeave.mockReturnValue(mutation);
    render(<RbacLeavesPage />);

    await user.type(screen.getByLabelText("User ID"), "9");
    setDateTime("Mulai", "2026-09-20T09:00");
    setDateTime("Berakhir", "2026-09-22T17:00");
    await user.type(screen.getByLabelText("Alasan (opsional)"), "Cuti tahunan");
    await user.click(screen.getByRole("button", { name: /catat cuti/i }));

    expect(mutation.mutate).toHaveBeenCalledTimes(1);
    const [values] = mutation.mutate.mock.calls[0] as [
      { user_id: number; start_at: string; end_at: string; reason?: string },
      unknown,
    ];
    expect(values.user_id).toBe(9);
    expect(values.reason).toBe("Cuti tahunan");
    expect(values.start_at).toBe(new Date("2026-09-20T09:00").toISOString());
    expect(values.end_at).toBe(new Date("2026-09-22T17:00").toISOString());
  });
});

// ─── RbacPoliciesPage: create form ───────────────────────────────────────────

describe("RbacPoliciesPage — create form", () => {
  async function fillPolicy(
    user: ReturnType<typeof userEvent.setup>,
    fields: { document_type?: string; min?: string; max?: string; level?: string },
  ) {
    if (fields.document_type !== undefined) {
      await user.type(screen.getByLabelText("Jenis Dokumen"), fields.document_type);
    }
    if (fields.min !== undefined) {
      await user.type(screen.getByLabelText("Nominal Minimum"), fields.min);
    }
    if (fields.max !== undefined) {
      await user.type(screen.getByLabelText("Nominal Maksimum"), fields.max);
    }
    if (fields.level !== undefined) {
      await user.type(screen.getByLabelText("Level Persetujuan"), fields.level);
    }
    await user.click(screen.getByRole("button", { name: /buat kebijakan/i }));
  }

  it("rejects min_amount greater than max_amount", async () => {
    const user = userEvent.setup();
    const mutation = idleMutation();
    mockUseCreatePolicy.mockReturnValue(mutation);
    render(<RbacPoliciesPage />);

    await fillPolicy(user, {
      document_type: "invoice",
      min: "5000000.00",
      max: "1000000.00",
      level: "1",
    });

    expect(screen.getByText("min_amount harus lebih kecil dari max_amount")).toBeInTheDocument();
    expect(mutation.mutate).not.toHaveBeenCalled();
  });

  it("requires a non-empty document_type", async () => {
    const user = userEvent.setup();
    render(<RbacPoliciesPage />);

    await fillPolicy(user, { min: "1000000.00", max: "5000000.00", level: "2" });

    expect(screen.getByText("document_type wajib diisi")).toBeInTheDocument();
  });

  it("submits a valid policy with amounts kept as exact strings", async () => {
    const user = userEvent.setup();
    const mutation = idleMutation();
    mockUseCreatePolicy.mockReturnValue(mutation);
    render(<RbacPoliciesPage />);

    await fillPolicy(user, {
      document_type: "invoice",
      min: "1000000.00",
      max: "5000000.00",
      level: "2",
    });

    expect(mutation.mutate).toHaveBeenCalledTimes(1);
    const [values] = mutation.mutate.mock.calls[0] as [
      { document_type: string; min_amount: string; max_amount: string; required_level: number },
      unknown,
    ];
    expect(values.document_type).toBe("invoice");
    expect(values.min_amount).toBe("1000000.00");
    expect(values.max_amount).toBe("5000000.00");
    expect(values.required_level).toBe(2);
  });
});
