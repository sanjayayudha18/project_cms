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
        branch_count: 2,
        member_count: 1,
      },
    ],
  }),
  useAcmAreaWarnings: () => ({
    data: [
      {
        vendor_branch_id: 9,
        branch_code: "BJK-01",
        branch_name: "Bijak Bekasi",
        vendor_name: "Bijak",
        request_count: 3,
      },
    ],
  }),
  useAcmArea: () => ({
    isLoading: false,
    data: {
      id: 1,
      name: "Jabodetabek",
      is_active: true,
      branches: [
        {
          vendor_branch_id: 5,
          branch_code: "BJK-02",
          branch_name: "Bijak Jakarta",
          region_code: "JKT",
          branch_is_active: true,
          vendor_name: "Bijak",
        },
      ],
      members: [],
    },
  }),
  useAcmBranchOptions: () => ({
    data: [
      {
        vendor_branch_id: 5,
        branch_code: "BJK-02",
        branch_name: "Bijak Jakarta",
        region_code: "JKT",
        vendor_name: "Bijak",
        acm_area_id: 1,
        acm_area_name: "Jabodetabek",
      },
      {
        vendor_branch_id: 6,
        branch_code: "ABA-01",
        branch_name: "Aba Makassar",
        region_code: "MKS",
        vendor_name: "Aba",
        acm_area_id: 2,
        acm_area_name: "Makassar",
      },
      {
        vendor_branch_id: 7,
        branch_code: "ABA-02",
        branch_name: "Aba Depok",
        region_code: "JKT",
        vendor_name: "Aba",
        acm_area_id: null,
        acm_area_name: null,
      },
    ],
  }),
  useAcmEligibleUsers: () => ({
    data: [{ id: 21, username: "budi", full_name: "Budi", role: "ACM-USER" }],
  }),
  useAcmAreaMutation: () => ({ mutate, isPending: false }),
}));
vi.mock("@/lib/hooks/useToast", () => ({ useToast: () => ({ toast: vi.fn() }) }));

describe("AdminAcmAreasPage", () => {
  it("shows areas and the branches-without-area warning", () => {
    render(<AdminAcmAreasPage />);
    expect(screen.getByText("Jabodetabek")).toBeTruthy();
    expect(screen.getByText("1 cabang belum punya area ACM")).toBeTruthy();
    expect(screen.getByText(/Bijak Bekasi/)).toBeTruthy();
  });

  it("creates an area with a trimmed name", () => {
    render(<AdminAcmAreasPage />);
    fireEvent.change(screen.getByLabelText("Nama area baru"), {
      target: { value: "  Makassar  " },
    });
    fireEvent.click(screen.getByRole("button", { name: "Tambah Area" }));
    expect(mutate).toHaveBeenCalledWith({ op: "create", name: "Makassar" }, expect.anything());
  });

  it("branch held by another area cannot be picked; saving sends the chosen ids", () => {
    render(<AdminAcmAreasPage />);
    fireEvent.click(screen.getByRole("button", { name: "Kelola" }));

    const held = screen.getByRole("checkbox", { name: /Aba Makassar/ }) as HTMLInputElement;
    expect(held.disabled).toBe(true);
    expect(screen.getByText(/sudah di area Makassar/)).toBeTruthy();

    fireEvent.click(screen.getByRole("checkbox", { name: /Aba Depok/ }));
    fireEvent.click(screen.getByRole("button", { name: "Simpan Cabang" }));
    expect(mutate).toHaveBeenCalledWith(
      { op: "branches", id: 1, vendor_branch_ids: [5, 7] },
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
