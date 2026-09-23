import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ATMAssignmentsDialog } from "../components/ATMAssignmentsDialog";
import type { AdminATM } from "../types";

vi.mock("../api", () => ({
  listATMAssignments: async () => [
    {
      id: 1,
      atm_id: 3,
      vendor_package_id: 8,
      package_code: "PKG-A",
      effective_start_date: "2026-01-01",
      effective_end_date: null,
      is_active: true,
    },
  ],
  createATMAssignment: vi.fn(),
}));
vi.mock("../../admin-vendors/api", () => ({
  listVendors: async () => ({ vendors: [], page: 1, page_size: 100, total: 0 }),
  listVendorChildren: async () => [],
}));

describe("ATMAssignmentsDialog", () => {
  it("lists periods with an open end and a text status; submit is disabled until a package and start date are chosen", async () => {
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
});
