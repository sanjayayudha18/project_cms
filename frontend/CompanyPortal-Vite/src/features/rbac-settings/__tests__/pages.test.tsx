/**
 * Page render tests (Task 16.1): each RBAC list page renders loading
 * (spinner + label), error (icon + text via NoticeBanner), and the populated
 * table with tabular-nums numeric cells and RoleBadge icons, using mocked
 * query hooks (Requirements 2.10, 2.11, 7.2, 9.2, 10.3, 10.5).
 */

import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

// ─── Mock useRbacQueries hooks ────────────────────────────────────────────────

const mockUseUserHierarchy = vi.fn();
const mockUseDelegations = vi.fn();
const mockUseLeaves = vi.fn();
const mockUsePolicies = vi.fn();

/** Mutation hooks only need an idle shape here; interactions live in forms.test.tsx. */
function idleMutation() {
  return { mutate: vi.fn(), isPending: false, isError: false, error: null };
}

vi.mock("../hooks/useRbacQueries", () => ({
  useUserHierarchy: () => mockUseUserHierarchy(),
  useDelegations: () => mockUseDelegations(),
  useLeaves: () => mockUseLeaves(),
  usePolicies: () => mockUsePolicies(),
  useSetHierarchy: () => idleMutation(),
  useCreateDelegation: () => idleMutation(),
  useRevokeDelegation: () => idleMutation(),
  useCreateLeave: () => idleMutation(),
  useCreatePolicy: () => idleMutation(),
  useUpdatePolicy: () => idleMutation(),
}));

// ─── Import components after mocks ───────────────────────────────────────────

import { RbacDelegationsPage } from "../components/RbacDelegationsPage";
import { RbacLeavesPage } from "../components/RbacLeavesPage";
import { RbacPoliciesPage } from "../components/RbacPoliciesPage";
import { RbacUsersPage } from "../components/RbacUsersPage";
import type {
  RbacDelegationsResponse,
  RbacLeavesResponse,
  RbacPoliciesResponse,
  RbacUsersResponse,
} from "../types";

// ─── Shared helpers ──────────────────────────────────────────────────────────

function loadingQuery() {
  return { data: undefined, isLoading: true, isError: false };
}

function errorQuery() {
  return { data: undefined, isLoading: false, isError: true };
}

function successQuery<T>(data: T) {
  return { data, isLoading: false, isError: false };
}

/** Asserts a StatusMessage error state: icon + text, never color alone. */
function expectErrorBanner(errorMessage: string) {
  expect(screen.getByText("Terjadi kesalahan")).toBeInTheDocument();
  expect(screen.getByText(errorMessage)).toBeInTheDocument();
  const banner = screen.getByRole("note");
  expect(banner.querySelector("svg")).toBeInTheDocument();
}

/** Asserts a StatusMessage loading state: spinner + label. */
function expectLoading(loadingLabel: string, container: HTMLElement) {
  expect(screen.getByText(loadingLabel)).toBeInTheDocument();
  expect(container.querySelector(".animate-spin")).toBeInTheDocument();
}

// ─── Test data ────────────────────────────────────────────────────────────────

const usersResponse: RbacUsersResponse = {
  users: [
    {
      id: 5,
      username: "dewi.lestari",
      full_name: "Dewi Lestari",
      supervisor_id: null,
      approval_level: 3,
      role: "ATM-SPV",
      auth_source: "ldap",
      vendor_id: null,
      vendor_name: null,
    },
  ],
};

const delegationsResponse: RbacDelegationsResponse = {
  delegations: [
    {
      id: 1,
      from_user_id: 10,
      to_user_id: 20,
      start_at: "2026-09-10T01:00:00Z",
      end_at: "2026-09-12T01:00:00Z",
      reason: "Cuti",
    },
  ],
};

const leavesResponse: RbacLeavesResponse = {
  leaves: [
    {
      id: 1,
      user_id: 9,
      start_at: "2026-09-10T01:00:00Z",
      end_at: "2026-09-12T01:00:00Z",
      reason: "Dinas luar",
    },
  ],
};

const policiesResponse: RbacPoliciesResponse = {
  policies: [
    {
      id: 1,
      document_type: "invoice",
      min_amount: "1000000.00",
      max_amount: "5000000.00",
      required_level: 2,
    },
  ],
};

// ─── RbacUsersPage ────────────────────────────────────────────────────────────

describe("RbacUsersPage", () => {
  it("renders loading state (spinner + label)", () => {
    mockUseUserHierarchy.mockReturnValue(loadingQuery());
    const { container } = render(<RbacUsersPage />);

    expectLoading("Memuat hierarki pengguna…", container);
  });

  it("renders error state (icon + text)", () => {
    mockUseUserHierarchy.mockReturnValue(errorQuery());
    render(<RbacUsersPage />);

    expectErrorBanner("Gagal memuat hierarki pengguna. Coba lagi.");
  });

  it("renders the populated table with tabular-nums numerics and RoleBadge icon", () => {
    mockUseUserHierarchy.mockReturnValue(successQuery(usersResponse));
    render(<RbacUsersPage />);

    const levelCell = screen.getByText("3").closest("td");
    expect(levelCell?.className).toContain("tabular-nums");

    // Requirement 9.2: role badge pairs a label with an icon.
    const badge = screen.getByText("ATM-SPV").closest("span");
    expect(badge?.querySelector("svg")).toBeInTheDocument();

    expect(screen.getByText("dewi.lestari")).toBeInTheDocument();
  });
});

// ─── RbacDelegationsPage ──────────────────────────────────────────────────────

describe("RbacDelegationsPage", () => {
  it("renders loading state (spinner + label)", () => {
    mockUseDelegations.mockReturnValue(loadingQuery());
    const { container } = render(<RbacDelegationsPage />);

    expectLoading("Memuat delegasi…", container);
  });

  it("renders error state (icon + text)", () => {
    mockUseDelegations.mockReturnValue(errorQuery());
    render(<RbacDelegationsPage />);

    expectErrorBanner("Gagal memuat delegasi. Coba lagi.");
  });

  it("renders the populated table with tabular-nums user ids and a revoke action", () => {
    mockUseDelegations.mockReturnValue(successQuery(delegationsResponse));
    render(<RbacDelegationsPage />);

    const fromCell = screen.getByText("10").closest("td");
    expect(fromCell?.className).toContain("tabular-nums");

    expect(screen.getByText("20")).toBeInTheDocument();
    expect(screen.getByText("Cuti")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /cabut/i })).toBeInTheDocument();
  });
});

// ─── RbacLeavesPage ───────────────────────────────────────────────────────────

describe("RbacLeavesPage", () => {
  it("renders loading state (spinner + label)", () => {
    mockUseLeaves.mockReturnValue(loadingQuery());
    const { container } = render(<RbacLeavesPage />);

    expectLoading("Memuat cuti pengguna…", container);
  });

  it("renders error state (icon + text)", () => {
    mockUseLeaves.mockReturnValue(errorQuery());
    render(<RbacLeavesPage />);

    expectErrorBanner("Gagal memuat cuti pengguna. Coba lagi.");
  });

  it("renders the populated table with a tabular-nums user id", () => {
    mockUseLeaves.mockReturnValue(successQuery(leavesResponse));
    render(<RbacLeavesPage />);

    const userCell = screen.getByText("9").closest("td");
    expect(userCell?.className).toContain("tabular-nums");
    expect(screen.getByText("Dinas luar")).toBeInTheDocument();
  });
});

// ─── RbacPoliciesPage ─────────────────────────────────────────────────────────

describe("RbacPoliciesPage", () => {
  it("renders loading state (spinner + label)", () => {
    mockUsePolicies.mockReturnValue(loadingQuery());
    const { container } = render(<RbacPoliciesPage />);

    expectLoading("Memuat kebijakan persetujuan…", container);
  });

  it("renders error state (icon + text)", () => {
    mockUsePolicies.mockReturnValue(errorQuery());
    render(<RbacPoliciesPage />);

    expectErrorBanner("Gagal memuat kebijakan persetujuan. Coba lagi.");
  });

  it("renders the populated table with right-aligned tabular-nums amounts", () => {
    mockUsePolicies.mockReturnValue(successQuery(policiesResponse));
    render(<RbacPoliciesPage />);

    // Requirement 7.2: amounts use tabular figures and right alignment, and
    // stay exact decimal strings end to end (never reformatted to a number).
    const minCell = screen.getByText("1000000.00").closest("td");
    expect(minCell?.className).toContain("text-right");
    expect(minCell?.className).toContain("tabular-nums");

    const maxCell = screen.getByText("5000000.00").closest("td");
    expect(maxCell?.className).toContain("text-right");
    expect(maxCell?.className).toContain("tabular-nums");

    expect(screen.getByText("invoice")).toBeInTheDocument();
    expect(screen.getByText("2")).toBeInTheDocument();
  });
});
