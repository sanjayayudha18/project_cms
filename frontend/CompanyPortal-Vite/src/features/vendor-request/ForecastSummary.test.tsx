import { useAuthStore } from "@/lib/auth/store";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ForecastSummary, summaryGroupKey } from "./ForecastSummary";
import type { ForecastSummaryResponse } from "./types";

const resetVendorSpy = vi.fn();
vi.mock("@/features/atm-portal/visitQuota", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/features/atm-portal/visitQuota")>();
  return {
    ...actual,
    useResetVendorQuota: () => ({ mutateAsync: resetVendorSpy, isPending: false }),
  };
});

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

const DATA: ForecastSummaryResponse = {
  forecast_date: "2027-01-15",
  totals: {
    atm_count: 7,
    requested_atm_count: 3,
    unrequested_atm_count: 4,
    unassigned_atm_count: 1,
    amount_replenish: 7_000_000,
    unrequested_amount_replenish: 4_000_000,
  },
  groups: [
    {
      vendor_id: 1,
      flm_vendor: "TAG",
      flm_vendor_region: "Jawa Barat",
      atm_count: 3,
      requested_atm_count: 0,
      unrequested_atm_count: 3,
      amount_replenish: 3_000_000,
      unrequested_amount_replenish: 3_000_000,
    },
    {
      vendor_id: 2,
      flm_vendor: "ROH",
      flm_vendor_region: "Jawa Timur",
      atm_count: 3,
      requested_atm_count: 3,
      unrequested_atm_count: 0,
      amount_replenish: 3_000_000,
      unrequested_amount_replenish: 0,
    },
    {
      vendor_id: 0,
      flm_vendor: "",
      flm_vendor_region: "",
      atm_count: 1,
      requested_atm_count: 0,
      unrequested_atm_count: 1,
      amount_replenish: 1_000_000,
      unrequested_amount_replenish: 1_000_000,
    },
  ],
};

function renderSummary(props: Partial<React.ComponentProps<typeof ForecastSummary>> = {}) {
  const onSelectGroup = props.onSelectGroup ?? vi.fn();
  render(
    <ForecastSummary
      data={"data" in props ? props.data : DATA}
      isLoading={props.isLoading ?? false}
      isError={props.isError ?? false}
      onRetry={props.onRetry ?? vi.fn()}
      activeKey={props.activeKey ?? null}
      onSelectGroup={onSelectGroup}
    />,
  );
  return { onSelectGroup };
}

function rowOf(buttonName: string): HTMLElement {
  const row = screen.getByRole("button", { name: buttonName }).closest("tr");
  if (!row) throw new Error(`no row for ${buttonName}`);
  return row;
}

describe("ForecastSummary", () => {
  it("shows the four KPI totals with explicit Rp amounts", () => {
    renderSummary();

    expect(screen.getByText("Perlu isi")).toBeInTheDocument();
    expect(screen.getByText("Rp 7.000.000")).toBeInTheDocument();
    expect(screen.getByText("Rp 4.000.000")).toBeInTheDocument();
    expect(screen.getByText("Tanpa vendor aktif", { selector: "p" })).toBeInTheDocument();
  });

  it("labels every status with text, not colour alone", () => {
    renderSummary();

    expect(
      within(rowOf("Lihat ATM TAG Jawa Barat")).getByText("Belum lengkap"),
    ).toBeInTheDocument();
    expect(within(rowOf("Lihat ATM ROH Jawa Timur")).getByText("Selesai")).toBeInTheDocument();
    expect(
      within(rowOf("Lihat ATM Tanpa vendor aktif")).getByText("Perlu master data"),
    ).toBeInTheDocument();
  });

  it("renders the group amount right-aligned as tabular-nums IDR", () => {
    renderSummary();

    const cell = within(rowOf("Lihat ATM TAG Jawa Barat")).getByText("Rp 3.000.000");
    expect(cell).toHaveClass("text-right", "tabular-nums");
  });

  it("calls onSelectGroup with the clicked group", async () => {
    const user = userEvent.setup();
    const { onSelectGroup } = renderSummary();

    await user.click(screen.getByRole("button", { name: "Lihat ATM Tanpa vendor aktif" }));

    expect(onSelectGroup).toHaveBeenCalledWith(DATA.groups[2]);
  });

  it("marks only the active group with aria-current", () => {
    renderSummary({ activeKey: summaryGroupKey(DATA.groups[1]) });

    expect(rowOf("Lihat ATM ROH Jawa Timur")).toHaveAttribute("aria-current", "true");
    expect(rowOf("Lihat ATM TAG Jawa Barat")).not.toHaveAttribute("aria-current");
  });

  it("shows an empty message when the date has no recommendations", () => {
    renderSummary({ data: { ...DATA, groups: [] } });

    expect(
      screen.getByText("Tidak ada rekomendasi forecast untuk tanggal ini"),
    ).toBeInTheDocument();
  });

  it("shows a retry action on error", async () => {
    const user = userEvent.setup();
    const onRetry = vi.fn();
    renderSummary({ isError: true, onRetry });

    await user.click(screen.getByRole("button", { name: "Coba Lagi" }));
    expect(onRetry).toHaveBeenCalled();
  });
});

describe("ForecastSummary — vendor branch without region (review R1)", () => {
  it('labels the region "(tanpa region)" and names the action accordingly', () => {
    const group = { ...DATA.groups[0], flm_vendor_region: "" };
    renderSummary({ data: { ...DATA, groups: [group] } });

    const row = rowOf("Lihat ATM TAG tanpa region");
    expect(within(row).getByText("(tanpa region)")).toBeInTheDocument();
  });
});

describe("summaryGroupKey", () => {
  it('maps the no-vendor group to "" and others to vendor|region', () => {
    expect(summaryGroupKey({ flm_vendor: "", flm_vendor_region: "" })).toBe("");
    expect(summaryGroupKey({ flm_vendor: "TAG", flm_vendor_region: "Jawa Barat" })).toBe(
      "TAG|Jawa Barat",
    );
  });
});

describe("ForecastSummary — Reset Kuota Vendor (atm-visit-quota FR6.6)", () => {
  it("is offered only after picking one vendor, only to a checker, in two steps", async () => {
    resetVendorSpy.mockReset().mockResolvedValue({ reset_count: 3, skipped_count: 1 });
    setRole("ATM-SPV");
    renderSummary();

    expect(screen.queryByRole("button", { name: /Reset Kuota Vendor/ })).not.toBeInTheDocument();
    await userEvent.selectOptions(screen.getByLabelText("Vendor"), "TAG");
    await userEvent.click(screen.getByRole("button", { name: /Reset Kuota Vendor/ }));
    expect(resetVendorSpy).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: "Ya, Reset" }));
    expect(resetVendorSpy).toHaveBeenCalledWith(1); // TAG's vendor_id in DATA
  });

  it("is hidden from a non-checker", async () => {
    setRole("ATM-USER");
    renderSummary();
    await userEvent.selectOptions(screen.getByLabelText("Vendor"), "TAG");
    expect(screen.queryByRole("button", { name: /Reset Kuota Vendor/ })).not.toBeInTheDocument();
  });
});
