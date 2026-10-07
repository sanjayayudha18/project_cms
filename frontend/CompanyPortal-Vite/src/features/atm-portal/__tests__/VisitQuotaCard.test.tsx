/**
 * VisitQuotaCard (atm-visit-quota FR6.5): kuota display, "Paket tidak
 * dikenali", checker-only reset (two-step) and visit cancel (reason
 * required), hidden on 403. Hooks are mocked — no live backend here.
 */

import { useAuthStore } from "@/lib/auth/store";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { VisitQuotaCard } from "../components/VisitQuotaCard";
import type { AtmVisitQuota } from "../visitQuota";

const useAtmVisitQuotaMock = vi.fn();
const resetSpy = vi.fn();
const cancelSpy = vi.fn();

vi.mock("../visitQuota", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../visitQuota")>();
  return {
    ...actual,
    useAtmVisitQuota: (...args: unknown[]) => useAtmVisitQuotaMock(...args),
    useResetAtmQuota: () => ({ mutateAsync: resetSpy, isPending: false }),
    useCancelAtmVisit: () => ({ mutateAsync: cancelSpy, isPending: false }),
  };
});

const QUOTA: AtmVisitQuota = {
  terminal_id: "T1",
  has_quota: true,
  package_code: "PAKET 5",
  quota_total: 5,
  remaining: -1,
  sisa: 0,
  kelebihan: 1,
  reset_at: "2026-10-01T01:00:00Z",
  reset_by: { id: 2, full_name: "SPV Satu" },
  current_package_code: "PAKET 5",
  current_quota: 5,
  visits: [
    {
      id: 11,
      vendor_request_id: 7,
      request_number: "REP-TAG-20261001-001",
      quota_known: true,
      is_over_quota: true,
      created_at: "2026-10-01T02:00:00Z",
      cancelled_at: null,
      cancelled_by_name: null,
      cancel_reason: null,
    },
  ],
};

function mockQuota(data: AtmVisitQuota | undefined, error: { status: number } | null = null) {
  useAtmVisitQuotaMock.mockReturnValue({
    data,
    isLoading: false,
    isError: error !== null,
    error,
  });
}

function setRole(role: string) {
  useAuthStore.setState({
    user: {
      id: 9,
      username: "u",
      fullName: "U",
      email: "u@x.com",
      role: role as never,
      isKaryawan: true,
      vendorId: null,
    },
  });
}

beforeEach(() => {
  useAtmVisitQuotaMock.mockReset();
  resetSpy.mockReset().mockResolvedValue(undefined);
  cancelSpy.mockReset().mockResolvedValue(undefined);
});

describe("VisitQuotaCard", () => {
  it("shows sisa/kuota, kelebihan and the over-quota visit (text badge)", () => {
    mockQuota(QUOTA);
    setRole("ATM-USER");
    render(<VisitQuotaCard terminalId="T1" />);

    expect(screen.getByText("0 / 5")).toBeInTheDocument();
    expect(screen.getByText("PAKET 5")).toBeInTheDocument();
    expect(screen.getByText("Kelebihan kuota")).toBeInTheDocument();
    // ATM-USER: read only.
    expect(screen.queryByRole("button", { name: /Reset Kuota/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Batalkan" })).not.toBeInTheDocument();
  });

  it("unknown package: explains it and offers no reset", () => {
    mockQuota({
      ...QUOTA,
      has_quota: false,
      current_package_code: null,
      current_quota: null,
      visits: [],
    });
    setRole("ATM-SPV");
    render(<VisitQuotaCard terminalId="T1" />);

    expect(screen.getByText(/Paket tidak dikenali/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Reset Kuota/ })).not.toBeInTheDocument();
  });

  it("checker resets in two steps", async () => {
    mockQuota(QUOTA);
    setRole("ATM-SPV");
    render(<VisitQuotaCard terminalId="T1" />);

    await userEvent.click(screen.getByRole("button", { name: /Reset Kuota/ }));
    expect(resetSpy).not.toHaveBeenCalled();
    expect(screen.getByText("Reset sisa ke 5?")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Ya, Reset" }));
    expect(resetSpy).toHaveBeenCalledWith("T1");
  });

  it("checker cancels a visit only with a reason", async () => {
    mockQuota(QUOTA);
    setRole("BRANCH-ATM-SPV");
    render(<VisitQuotaCard terminalId="T1" />);

    await userEvent.click(screen.getByRole("button", { name: "Batalkan" }));
    const confirm = screen.getByRole("button", { name: "Batalkan Kunjungan" });
    expect(confirm).toBeDisabled();
    await userEvent.type(screen.getByLabelText("Alasan Pembatalan Kunjungan"), "ATM gagal diisi");
    await userEvent.click(confirm);
    expect(cancelSpy).toHaveBeenCalledWith({ visitId: 11, reason: "ATM gagal diisi" });
  });

  it("renders nothing when the role is refused (403)", () => {
    mockQuota(undefined, { status: 403 });
    setRole("VENDOR");
    const { container } = render(<VisitQuotaCard terminalId="T1" />);
    expect(container).toBeEmptyDOMElement();
  });
});
