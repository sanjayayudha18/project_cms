/**
 * region-management Task 9.3: schema, form server-error mapping, URL state
 * round-trip, and table a11y (status = icon + label, code tabular-nums).
 */

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { RegionFormDialog } from "../components/RegionFormDialog";
import { RegionsTable } from "../components/RegionsTable";
import { type AdminRegion, createRegionSchema, updateRegionSchema } from "../types";
import { omitDefaults, parseParams } from "../useAdminRegionsUrlState";

const REGION: AdminRegion = {
  id: 7,
  code: "JKT",
  region: "Jakarta",
  is_active: true,
  location_count: 12,
  created_at: null,
  updated_at: null,
  deleted_at: null,
};

const createMutateAsync = vi.fn();
const updateMutateAsync = vi.fn();

vi.mock("../hooks", () => ({
  useCreateRegion: () => ({ mutateAsync: createMutateAsync, isPending: false }),
  useUpdateRegion: () => ({ mutateAsync: updateMutateAsync, isPending: false }),
}));

beforeEach(() => {
  createMutateAsync.mockReset();
  updateMutateAsync.mockReset();
});

describe("region schemas", () => {
  it.each([
    [{ code: "JKT01", region: "Jakarta" }, true],
    [{ code: "  jkt  ", region: " Jakarta " }, true],
    [{ code: "", region: "Jakarta" }, false],
    [{ code: "JK-T", region: "Jakarta" }, false],
    [{ code: "A".repeat(21), region: "Jakarta" }, false],
    [{ code: "JKT", region: "   " }, false],
    [{ code: "JKT", region: "x".repeat(101) }, false],
  ])("createRegionSchema(%j) valid=%s", (input, valid) => {
    expect(createRegionSchema.safeParse(input).success).toBe(valid);
  });

  it("updateRegionSchema only carries the name (code is immutable)", () => {
    const parsed = updateRegionSchema.parse({ code: "NEW", region: "Jakarta" });
    expect(parsed).toEqual({ region: "Jakarta" });
  });
});

describe("RegionFormDialog", () => {
  it("maps a 409 conflict to the code field and keeps the dialog open", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    createMutateAsync.mockRejectedValue({ status: 409, message: "kode region sudah dipakai" });
    render(<RegionFormDialog open onClose={onClose} region={null} />);

    await user.type(screen.getByLabelText("Code"), "JKT");
    await user.type(screen.getByLabelText("Nama"), "Jakarta");
    await user.click(screen.getByRole("button", { name: "Simpan" }));

    expect(await screen.findByText("kode region sudah dipakai")).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("blocks invalid input client-side without calling the API", async () => {
    const user = userEvent.setup();
    render(<RegionFormDialog open onClose={vi.fn()} region={null} />);

    await user.type(screen.getByLabelText("Code"), "JK-T");
    await user.click(screen.getByRole("button", { name: "Simpan" }));

    expect(await screen.findByText("Hanya huruf dan angka")).toBeInTheDocument();
    expect(createMutateAsync).not.toHaveBeenCalled();
  });

  it("edit mode disables code and submits only the name", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    updateMutateAsync.mockResolvedValue({ ...REGION, region: "Jakarta Raya" });
    render(<RegionFormDialog open onClose={onClose} region={REGION} />);

    expect(screen.getByLabelText("Code")).toBeDisabled();
    const name = screen.getByLabelText("Nama");
    await user.clear(name);
    await user.type(name, "Jakarta Raya");
    await user.click(screen.getByRole("button", { name: "Simpan" }));

    expect(updateMutateAsync).toHaveBeenCalledWith({ id: 7, payload: { region: "Jakarta Raya" } });
    expect(onClose).toHaveBeenCalled();
  });
});

describe("admin regions URL state", () => {
  it("omits defaults and round-trips non-default filters", () => {
    expect(omitDefaults(parseParams({}))).toEqual({});
    const params = { page: 3, page_size: 50, status: "inactive" as const, q: "jak" };
    expect(parseParams(omitDefaults(params))).toEqual(params);
  });
});

describe("RegionsTable", () => {
  it("shows status as icon + label and code/count with tabular-nums", () => {
    render(
      <RegionsTable
        regions={[REGION, { ...REGION, id: 8, code: "BDG", is_active: false }]}
        onEdit={vi.fn()}
        onDisable={vi.fn()}
        onEnable={vi.fn()}
      />,
    );

    expect(screen.getByText("Aktif")).toBeInTheDocument();
    expect(screen.getByText("Nonaktif")).toBeInTheDocument();
    expect(screen.getByText("JKT")).toHaveClass("tabular-nums");

    const bdgRow = screen.getByText("BDG").closest("tr") as HTMLElement;
    expect(within(bdgRow).getByRole("button", { name: "Aktifkan" })).toBeInTheDocument();
  });
});
