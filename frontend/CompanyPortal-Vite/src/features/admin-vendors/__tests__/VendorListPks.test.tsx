/**
 * vendor-pks-type Task 7.3 (Req 5.2, 5.3, 5.6):
 * Grouped PKS columns per active PKS type in VendorsTable and paired Type + Status filters.
 */

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { VendorFilterBar } from "../components/VendorFilterBar";
import { VendorsTable } from "../components/VendorsTable";
import type { AdminVendor, PksType } from "../types";

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children }: { children: React.ReactNode }) => <a href="/x">{children}</a>,
}));

const ACTIVE_PKS_TYPES: PksType[] = [
  { id: 1, code: "CIT", name: "Cash in Transit", is_active: true, created_at: "", updated_at: "" },
  {
    id: 2,
    code: "FLM",
    name: "First Line Maintenance",
    is_active: true,
    created_at: "",
    updated_at: "",
  },
];

const flmVendor: AdminVendor = {
  id: 1,
  code: "V1",
  name: "Vendor Satu",
  legal_name: "PT Vendor Satu",
  npwp: "01.234.567.8-901.000",
  contact_email: "v1@example.com",
  contact_phone: "021-1234567",
  hq_address: "Jakarta",
  is_active: true,
  deleted_at: null,
  kind: "FLM_VENDOR",
  director: { name: "Pak Direktur", phone: null, email: null },
  pks: [
    {
      pks_type: "CIT",
      pks_type_name: "Cash in Transit",
      pks_status: "aktif",
      remaining_days: 120,
    },
    {
      pks_type: "FLM",
      pks_type_name: "First Line Maintenance",
      pks_status: "expired",
      remaining_days: -12,
    },
  ],
};

const noPksVendor: AdminVendor = {
  id: 3,
  code: "V3",
  name: "Vendor Tiga",
  legal_name: "PT Vendor Tiga",
  npwp: "01.234.567.8-903.000",
  contact_email: "v3@example.com",
  contact_phone: "021-1234569",
  hq_address: "Surabaya",
  is_active: true,
  deleted_at: null,
  kind: "FLM_VENDOR",
  director: null,
  pks: [],
};

const internalVendor: AdminVendor = {
  id: 2,
  code: "ROH",
  name: "Vendor Internal",
  legal_name: "PT Vendor Internal",
  npwp: "01.234.567.8-902.000",
  contact_email: "internal@example.com",
  contact_phone: "021-1234568",
  hq_address: "Bandung",
  is_active: true,
  deleted_at: null,
  kind: "INTERNAL",
  director: null,
  pks: [],
};

function renderTable(vendors: AdminVendor[], pksTypes = ACTIVE_PKS_TYPES) {
  return render(
    <VendorsTable
      vendors={vendors}
      pksTypes={pksTypes}
      pendingIds={new Set()}
      onEdit={() => {}}
      onDisable={() => {}}
      onEnable={() => {}}
    />,
  );
}

const rowOf = (name: string) => screen.getByText(name).closest("tr") as HTMLElement;

describe("VendorsTable PKS type column groups (Req 5.2, 5.3)", () => {
  it("renders grouped column headers per active PKS type and director header, removing old single PKS columns", () => {
    renderTable([flmVendor]);

    expect(screen.getByRole("columnheader", { name: "Nama Direktur Utama" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "PKS CIT" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "PKS FLM" })).toBeInTheDocument();

    // Column headers: 1 vendor Status + 2 PKS Status sub-columns = 3 Status headers
    expect(screen.getAllByRole("columnheader", { name: "Status" })).toHaveLength(3);
    // 2 Monitoring sub-columns (one per active PKS type)
    expect(screen.getAllByRole("columnheader", { name: "Monitoring" })).toHaveLength(2);

    // Old single PKS headers must be removed
    expect(screen.queryByRole("columnheader", { name: "Nama PKS" })).toBeNull();
    expect(screen.queryByRole("columnheader", { name: "Tanggal Expired PKS" })).toBeNull();
    expect(screen.queryByRole("columnheader", { name: "Monitoring PKS" })).toBeNull();
  });

  it("FLM vendor with active and expired PKS: shows status badge and remaining days per type", () => {
    renderTable([flmVendor]);

    const row = within(rowOf("Vendor Satu"));
    expect(row.getByText("Pak Direktur")).toBeInTheDocument();

    // CIT has 'aktif' status and 120 days remaining
    expect(row.getByText("120 hari")).toBeInTheDocument();
    // FLM has 'expired' status and -12 days
    expect(row.getByText("lewat 12 hari")).toBeInTheDocument();
    expect(row.getByText("Expired")).toBeInTheDocument();
  });

  it("vendor with no PKS by type shows neutral 'Tidak ada' badge and '-' for monitoring", () => {
    renderTable([noPksVendor]);

    const row = within(rowOf("Vendor Tiga"));
    // Shows 'Tidak ada' for both active PKS types
    expect(row.getAllByText("Tidak ada")).toHaveLength(2);
    // Old label 'Belum diisi' is no longer used
    expect(row.queryByText("Belum diisi")).toBeNull();
  });

  it("INTERNAL vendor: shows dashes only and no PKS status badge", () => {
    renderTable([internalVendor]);

    const row = within(rowOf("Vendor Internal"));
    expect(row.queryByText("Tidak ada")).toBeNull();
    expect(row.queryByText("Belum diisi")).toBeNull();
    expect(row.getAllByText("-").length).toBeGreaterThanOrEqual(4);
  });
});

const SEARCH = { searchInput: "", onSearchInputChange: () => {} };

describe("VendorFilterBar Type PKS and Status PKS paired filter (Req 5.6)", () => {
  const base = { page: 1, page_size: 25, status: "active" as const, q: "" };

  it("lists active PKS types in 'Type PKS' and disables 'Status PKS' when type is empty", () => {
    render(
      <VendorFilterBar
        params={base}
        pksTypes={ACTIVE_PKS_TYPES}
        onParamsChange={() => {}}
        {...SEARCH}
      />,
    );

    const typeSelect = screen.getByLabelText("Type PKS");
    const typeLabels = within(typeSelect)
      .getAllByRole("option")
      .map((o) => o.textContent);
    expect(typeLabels).toEqual(["Semua", "CIT - Cash in Transit", "FLM - First Line Maintenance"]);

    const statusSelect = screen.getByLabelText("Status PKS");
    expect(statusSelect).toBeDisabled();
  });

  it("enables 'Status PKS' when pks_type is selected and lists status options with 'Tidak ada'", () => {
    render(
      <VendorFilterBar
        params={{ ...base, pks_type: "CIT" }}
        pksTypes={ACTIVE_PKS_TYPES}
        onParamsChange={() => {}}
        {...SEARCH}
      />,
    );

    const statusSelect = screen.getByLabelText("Status PKS");
    expect(statusSelect).not.toBeDisabled();

    const statusLabels = within(statusSelect)
      .getAllByRole("option")
      .map((o) => o.textContent);
    expect(statusLabels).toEqual([
      "Semua",
      "Aktif",
      "Need Renewal",
      "Expired",
      "Terminated",
      "Tidak ada",
    ]);
  });

  it("choosing Type enables Status; choosing Status dispatches both paired params with page: 1", async () => {
    const onParamsChange = vi.fn();
    const user = userEvent.setup();
    render(
      <VendorFilterBar
        params={base}
        pksTypes={ACTIVE_PKS_TYPES}
        onParamsChange={onParamsChange}
        {...SEARCH}
      />,
    );

    const typeSelect = screen.getByLabelText("Type PKS");
    await user.selectOptions(typeSelect, "CIT");

    const statusSelect = screen.getByLabelText("Status PKS");
    expect(statusSelect).not.toBeDisabled();

    await user.selectOptions(statusSelect, "need_renewal");
    expect(onParamsChange).toHaveBeenLastCalledWith({
      pks_type: "CIT",
      pks_status: "need_renewal",
      page: 1,
    });
  });

  it("clearing Type PKS clears both pks_type and pks_status with page: 1", async () => {
    const onParamsChange = vi.fn();
    const user = userEvent.setup();
    render(
      <VendorFilterBar
        params={{ ...base, pks_type: "CIT", pks_status: "aktif" }}
        pksTypes={ACTIVE_PKS_TYPES}
        onParamsChange={onParamsChange}
        {...SEARCH}
      />,
    );

    await user.selectOptions(screen.getByLabelText("Type PKS"), "");
    expect(onParamsChange).toHaveBeenLastCalledWith({
      pks_type: undefined,
      pks_status: undefined,
      page: 1,
    });
  });

  it("clearing Status PKS clears both pks_type and pks_status with page: 1", async () => {
    const onParamsChange = vi.fn();
    const user = userEvent.setup();
    render(
      <VendorFilterBar
        params={{ ...base, pks_type: "CIT", pks_status: "aktif" }}
        pksTypes={ACTIVE_PKS_TYPES}
        onParamsChange={onParamsChange}
        {...SEARCH}
      />,
    );

    await user.selectOptions(screen.getByLabelText("Status PKS"), "");
    expect(onParamsChange).toHaveBeenLastCalledWith({
      pks_type: undefined,
      pks_status: undefined,
      page: 1,
    });
  });
});
