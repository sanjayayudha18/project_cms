import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { createATMAssignment } from "../api";
import { ATMAssignmentsDialog } from "../components/ATMAssignmentsDialog";
import type { AdminATM } from "../types";

vi.mock("../api", () => ({
  listATMAssignments: async () => [
    {
      id: 1,
      atm_id: 3,
      vendor_package_id: 8,
      package_code: "PKG-A",
      source: "branch",
      effective_start_date: "2026-01-01",
      effective_end_date: null,
      is_active: true,
      vendor_id: 2,
      vendor_branch_id: 5,
    },
  ],
  createATMAssignment: vi.fn(async () => ({ change_request_id: 1, status: "pending" })),
  listATMPackageOptions: vi.fn(async () => ["PAKET 3", "PAKET 4"]),
}));
vi.mock("../../admin-vendors/api", () => ({
  listVendors: async () => ({
    vendors: [{ id: 2, code: "ABA", name: "Abacus" }],
    page: 1,
    page_size: 100,
    total: 1,
  }),
  listVendorChildren: async () => [
    { id: 5, branch_code: "ABACUS_002", branch_name: "Abacus Bandung", is_active: true },
  ],
  listVendorPackages: async () => ({
    packages: [
      {
        id: 8,
        package_code: "PKG-A",
        machine_group: "ATM",
        price_class: "REGULAR",
        atm_id: null,
      },
      {
        id: 9,
        package_code: "PKG-B",
        machine_group: "ATM",
        price_class: "REGULAR",
        atm_id: null,
      },
    ],
    page: 1,
    page_size: 100,
    total: 1,
  }),
}));

describe("ATMAssignmentsDialog", () => {
  it("lists periods with an open end and a text status; submit stays disabled while the form equals the current period", async () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <ATMAssignmentsDialog
          atm={{ id: 3, terminal_id: "T-003" } as AdminATM}
          onClose={() => {}}
        />
      </QueryClientProvider>,
    );
    expect(await screen.findByText("Terbuka")).toBeTruthy();
    expect(screen.getByText("Aktif")).toBeTruthy();
    expect((screen.getByRole("button", { name: "Ajukan" }) as HTMLButtonElement).disabled).toBe(
      true,
    );
  });

  it("prefills vendor, cabang and paket from the current period, has no date fields, and submits only the package", async () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <ATMAssignmentsDialog
          atm={{ id: 3, terminal_id: "T-003" } as AdminATM}
          onClose={() => {}}
        />
      </QueryClientProvider>,
    );
    const vendor = (await screen.findByLabelText("Vendor")) as HTMLSelectElement;
    await waitFor(() => expect(vendor.value).toBe("2"));
    const cabang = screen.getByLabelText("Cabang") as HTMLSelectElement;
    const paket = screen.getByLabelText("Paket") as HTMLSelectElement;
    await waitFor(() => expect(cabang.value).toBe("5"));
    await waitFor(() => expect(paket.value).toBe("8"));
    expect(screen.queryByLabelText("Mulai berlaku")).toBeNull();
    expect(screen.queryByLabelText(/Berakhir/)).toBeNull();

    const ajukan = screen.getByRole("button", { name: "Ajukan" }) as HTMLButtonElement;
    expect(ajukan.disabled).toBe(true); // same package as the running period
    fireEvent.change(paket, { target: { value: "9" } });
    expect(ajukan.disabled).toBe(false);
    fireEvent.click(ajukan);
    await waitFor(() =>
      expect(createATMAssignment).toHaveBeenCalledWith(3, {
        source: "branch",
        vendor_package_id: 9,
      }),
    );
  });

  it("requires a cabang in both modes: no 'Semua cabang' option, submit stays disabled until one is chosen", async () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <ATMAssignmentsDialog
          atm={{ id: 3, terminal_id: "T-003" } as AdminATM}
          onClose={() => {}}
        />
      </QueryClientProvider>,
    );
    const cabang = (await screen.findByLabelText("Cabang")) as HTMLSelectElement;
    expect(screen.queryByText("Semua cabang")).toBeNull();
    await waitFor(() => expect(cabang.value).toBe("5"));
    fireEvent.change(cabang, { target: { value: "" } });
    fireEvent.click(screen.getByLabelText(/Paket seluruh vendor/));
    await waitFor(() => expect(screen.getByRole("option", { name: "PAKET 4" })).toBeTruthy());
    fireEvent.change(screen.getByLabelText("Paket"), { target: { value: "PAKET 4" } });
    expect((screen.getByRole("button", { name: "Ajukan" }) as HTMLButtonElement).disabled).toBe(
      true,
    );
  });

  it("vendor-wide: picks a label from the ATM's options and submits vendor, cabang and label", async () => {
    vi.mocked(createATMAssignment).mockClear();
    render(
      <QueryClientProvider client={new QueryClient()}>
        <ATMAssignmentsDialog
          atm={{ id: 3, terminal_id: "T-003" } as AdminATM}
          onClose={() => {}}
        />
      </QueryClientProvider>,
    );
    const vendor = (await screen.findByLabelText("Vendor")) as HTMLSelectElement;
    await waitFor(() => expect(vendor.value).toBe("2"));
    const cabang = screen.getByLabelText("Cabang") as HTMLSelectElement;
    await waitFor(() => expect(cabang.value).toBe("5"));

    fireEvent.click(screen.getByLabelText(/Paket seluruh vendor/));
    // Switching the source clears the package until a new one is picked.
    const paket = screen.getByLabelText("Paket") as HTMLSelectElement;
    expect(paket.value).toBe("");
    const ajukan = screen.getByRole("button", { name: "Ajukan" }) as HTMLButtonElement;
    expect(ajukan.disabled).toBe(true);
    await waitFor(() => expect(screen.getByRole("option", { name: "PAKET 4" })).toBeTruthy());
    fireEvent.change(paket, { target: { value: "PAKET 4" } });
    expect(ajukan.disabled).toBe(false);
    fireEvent.click(ajukan);
    await waitFor(() =>
      expect(createATMAssignment).toHaveBeenCalledWith(3, {
        source: "vendor",
        vendor_id: 2,
        vendor_branch_id: 5,
        package: "PAKET 4",
      }),
    );
  });
});
