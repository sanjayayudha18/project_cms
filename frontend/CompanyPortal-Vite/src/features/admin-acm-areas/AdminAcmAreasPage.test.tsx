import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { AdminAcmAreasPage } from "./AdminAcmAreasPage";

const mutate = vi.fn();
vi.mock("./api", () => ({
  useAcmAreas: () => ({
    isLoading: false,
    isError: false,
    data: [
      {
        id: 1,
        name: "Jabodetabek",
        is_active: true,
        deleted_at: null,
        region_count: 2,
        member_count: 1,
      },
    ],
  }),
  useAcmAreaWarnings: () => ({
    data: [
      {
        vendor_request_id: 41,
        request_number: "REP-BJK-SBY-20261013-001",
        region_code: "SBY",
        replenish_date: "2026-10-13",
      },
      {
        vendor_request_id: 42,
        request_number: "REP-BJK-20261001-002",
        region_code: null,
        replenish_date: "2026-10-01",
      },
    ],
  }),
  useAcmArea: () => ({
    isLoading: false,
    data: {
      id: 1,
      name: "Jabodetabek",
      is_active: true,
      region_codes: ["JKT", "OLD"],
      members: [],
    },
  }),
  useAcmRegionOptions: () => ({
    data: [
      { region_code: "JKT", branch_count: 3, acm_area_id: 1, acm_area_name: "Jabodetabek" },
      { region_code: "MKS", branch_count: 1, acm_area_id: 2, acm_area_name: "Makassar" },
      { region_code: "SBY", branch_count: 2, acm_area_id: null, acm_area_name: null },
    ],
  }),
  useAcmEligibleUsers: () => ({
    data: [{ id: 21, username: "budi", full_name: "Budi", role: "ACM-USER" }],
  }),
  useAcmAreaMutation: () => ({ mutate, isPending: false }),
}));
vi.mock("@/lib/hooks/useToast", () => ({ useToast: () => ({ toast: vi.fn() }) }));

describe("AdminAcmAreasPage", () => {
  it("shows areas and the requests waiting for an area (FR1.4)", () => {
    render(<AdminAcmAreasPage />);
    expect(screen.getByText("Jabodetabek")).toBeTruthy();
    expect(screen.getByText("2 request menunggu area ACM")).toBeTruthy();
    expect(screen.getByText("REP-BJK-SBY-20261013-001")).toBeTruthy();
    expect(screen.getByText(/tidak ada \(nomor format lama\)/)).toBeTruthy();
  });

  it("creates an area with a trimmed name", () => {
    render(<AdminAcmAreasPage />);
    fireEvent.change(screen.getByLabelText("Nama area baru"), {
      target: { value: "  Makassar  " },
    });
    fireEvent.click(screen.getByRole("button", { name: "Tambah Area" }));
    expect(mutate).toHaveBeenCalledWith({ op: "create", name: "Makassar" }, expect.anything());
  });

  it("region held by another area cannot be picked; saving sends the chosen codes", () => {
    render(<AdminAcmAreasPage />);
    fireEvent.click(screen.getByRole("button", { name: "Kelola" }));
    expect(screen.queryByRole("button", { name: "Simpan Cabang" })).toBeNull();

    const held = screen.getByRole("checkbox", { name: /MKS/ }) as HTMLInputElement;
    expect(held.disabled).toBe(true);
    expect(screen.getByText(/sudah di area Makassar/)).toBeTruthy();
    // A code of this area no longer used by an active branch stays visible and checked.
    expect((screen.getByRole("checkbox", { name: /OLD/ }) as HTMLInputElement).checked).toBe(true);

    fireEvent.click(screen.getByRole("checkbox", { name: /SBY/ }));
    fireEvent.click(screen.getByRole("checkbox", { name: /OLD/ }));
    fireEvent.click(screen.getByRole("button", { name: "Simpan Region" }));
    expect(mutate).toHaveBeenCalledWith(
      { op: "regions", id: 1, region_codes: ["JKT", "SBY"] },
      expect.anything(),
    );
  });

  it("saves members from the ACM user list", () => {
    render(<AdminAcmAreasPage />);
    fireEvent.click(screen.getByRole("button", { name: "Kelola" }));
    fireEvent.click(screen.getByRole("checkbox", { name: /Budi/ }));
    fireEvent.click(screen.getByRole("button", { name: "Simpan Anggota" }));
    expect(mutate).toHaveBeenCalledWith(
      { op: "members", id: 1, user_ids: [21] },
      expect.anything(),
    );
  });
});
