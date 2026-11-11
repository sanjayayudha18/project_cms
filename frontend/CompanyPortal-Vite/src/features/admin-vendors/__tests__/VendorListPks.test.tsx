/**
 * vendor-pks 11.5 (Req 1.10, 9.1, 9.2): PKS columns in VendorsTable and the "Status PKS" filter.
 */

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { VendorFilterBar } from "../components/VendorFilterBar";
import { VendorsTable } from "../components/VendorsTable";
import type { AdminVendor } from "../types";

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children }: { children: React.ReactNode }) => <a href="/x">{children}</a>,
}));

const flm = {
  id: 1,
  code: "V1",
  name: "Vendor Satu",
  is_active: true,
  kind: "FLM_VENDOR",
  director: { name: "Pak Direktur", phone: null, email: null },
  pks: {
    id: 7,
    pks_name: "PKS 2026",
    effective_date: "2026-01-15",
    expiry_date: "2027-01-14",
    validity_months: 12,
    remaining_days: 120,
  },
  pks_status: "aktif",
} as AdminVendor;

const internal = {
  id: 2,
  code: "ROH",
  name: "Vendor Internal",
  is_active: true,
  kind: "INTERNAL",
  director: null,
  pks: null,
  pks_status: null,
} as AdminVendor;

const noPks = {
  ...flm,
  id: 3,
  code: "V3",
  name: "Vendor Tiga",
  director: null,
  pks: null,
  pks_status: "belum_diisi",
} as AdminVendor;

const expired = {
  ...flm,
  id: 4,
  code: "V4",
  name: "Vendor Empat",
  pks: { ...flm.pks, remaining_days: -12, expiry_date: "2026-03-01" },
  pks_status: "expired",
} as AdminVendor;

function renderTable(vendors: AdminVendor[]) {
  return render(
    <VendorsTable
      vendors={vendors}
      pendingIds={new Set()}
      onEdit={() => {}}
      onDisable={() => {}}
      onEnable={() => {}}
    />,
  );
}
const rowOf = (name: string) => screen.getByText(name).closest("tr") as HTMLElement;

describe("VendorsTable PKS columns", () => {
  it("has the five new column headers", () => {
    renderTable([flm]);

    for (const h of [
      "Nama Direktur Utama",
      "Nama PKS",
      "Tanggal Expired PKS",
      "Monitoring PKS",
      "Status PKS",
    ]) {
      expect(screen.getByRole("columnheader", { name: h })).toBeInTheDocument();
    }
  });

  it("FLM vendor with PKS: director, PKS name, dd/mm/yyyy expiry, days left and status text", () => {
    renderTable([flm]);

    const row = within(rowOf("Vendor Satu"));
    expect(row.getByText("Pak Direktur")).toBeInTheDocument();
    expect(row.getByText("PKS 2026")).toBeInTheDocument();
    expect(row.getByText("14/01/2027")).toBeInTheDocument();
    expect(row.getByText("120 hari")).toBeInTheDocument();
    // vendor status badge + PKS status badge, both text-labelled
    expect(row.getAllByText("Aktif")).toHaveLength(2);
  });

  it("no PKS recorded: dashes and the 'Belum diisi' label", () => {
    renderTable([noPks]);

    const row = within(rowOf("Vendor Tiga"));
    expect(row.getByText("Belum diisi")).toBeInTheDocument();
    expect(row.getAllByText("-").length).toBeGreaterThanOrEqual(4);
  });

  it("expired PKS shows 'lewat N hari' with an Expired label", () => {
    renderTable([expired]);

    const row = within(rowOf("Vendor Empat"));
    expect(row.getByText("lewat 12 hari")).toBeInTheDocument();
    expect(row.getByText("Expired")).toBeInTheDocument();
    expect(row.getByText("01/03/2026")).toBeInTheDocument();
  });

  it("INTERNAL vendor (pks_status null): only dashes in the PKS columns", () => {
    renderTable([internal]);

    const row = within(rowOf("Vendor Internal"));
    expect(row.getAllByText("-").length).toBeGreaterThanOrEqual(5);
    expect(row.queryByText("Belum diisi")).toBeNull();
  });
});

const SEARCH = { searchInput: "", onSearchInputChange: () => {} };

describe("VendorFilterBar Status PKS", () => {
  const base = { page: 1, page_size: 25, status: "active" as const, q: "" };

  it("lists the five PKS statuses plus 'Semua'", () => {
    render(<VendorFilterBar params={base} onParamsChange={() => {}} {...SEARCH} />);

    const select = screen.getByLabelText("Status PKS");
    const labels = within(select)
      .getAllByRole("option")
      .map((o) => o.textContent);
    expect(labels).toEqual([
      "Semua",
      "Aktif",
      "Need Renewal",
      "Expired",
      "Terminated",
      "Belum diisi",
    ]);
  });

  it("choosing a status reports it; choosing 'Semua' clears it", async () => {
    const onParamsChange = vi.fn();
    const user = userEvent.setup();
    const { rerender } = render(
      <VendorFilterBar params={base} onParamsChange={onParamsChange} {...SEARCH} />,
    );

    await user.selectOptions(screen.getByLabelText("Status PKS"), "need_renewal");
    expect(onParamsChange).toHaveBeenLastCalledWith({ pks_status: "need_renewal" });

    rerender(
      <VendorFilterBar
        params={{ ...base, pks_status: "need_renewal" }}
        onParamsChange={onParamsChange}
        {...SEARCH}
      />,
    );
    expect(screen.getByLabelText("Status PKS")).toHaveValue("need_renewal");
    await user.selectOptions(screen.getByLabelText("Status PKS"), "");
    expect(onParamsChange).toHaveBeenLastCalledWith({ pks_status: undefined });
  });
});
