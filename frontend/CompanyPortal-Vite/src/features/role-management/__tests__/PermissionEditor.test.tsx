import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { PermissionEditor } from "../components/PermissionEditor";
import type { CatalogEntry, RoleWithPermissions } from "../types";

const updateMutateAsync = vi.fn();

vi.mock("../hooks/useRoleQueries", () => ({
  useUpdateRolePermissions: () => ({ mutateAsync: updateMutateAsync, isPending: false }),
}));

const CATALOG: CatalogEntry[] = [
  { id: 1, parent_id: null, key: "dashboard", label: "Dashboard", kind: "menu", sort_order: 0 },
  { id: 2, parent_id: null, key: "settings", label: "Pengaturan", kind: "menu", sort_order: 1 },
  {
    id: 3,
    parent_id: 2,
    key: "settings.roles",
    label: "Manajemen Peran",
    kind: "feature",
    sort_order: 0,
  },
];

const ROLE: RoleWithPermissions = {
  id: 10,
  role: "AUDITOR-VIEW",
  description: null,
  permissions: [
    { menu_feature_id: 1, parent_id: null, key: "dashboard", label: "Dashboard", kind: "menu" },
  ],
};

beforeEach(() => {
  updateMutateAsync.mockReset();
});

describe("PermissionEditor", () => {
  it("renders the menu→feature tree with the role's current grants checked", () => {
    render(<PermissionEditor role={ROLE} catalog={CATALOG} />);

    expect(screen.getByRole("checkbox", { name: "Dashboard" })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Pengaturan" })).not.toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Manajemen Peran" })).not.toBeChecked();
  });

  it("disables Simpan until a toggle actually changes the set", async () => {
    const user = userEvent.setup();
    render(<PermissionEditor role={ROLE} catalog={CATALOG} />);

    expect(screen.getByRole("button", { name: "Simpan Izin" })).toBeDisabled();

    await user.click(screen.getByRole("checkbox", { name: "Manajemen Peran" }));
    expect(screen.getByRole("button", { name: "Simpan Izin" })).not.toBeDisabled();
  });

  it("toggle → mutation called with the correct full set (design property 5, full-set replace)", async () => {
    updateMutateAsync.mockResolvedValue({ role_id: 10, permissions: [1, 3] });
    const user = userEvent.setup();
    render(<PermissionEditor role={ROLE} catalog={CATALOG} />);

    await user.click(screen.getByRole("checkbox", { name: "Manajemen Peran" }));
    await user.click(screen.getByRole("button", { name: "Simpan Izin" }));

    await waitFor(() =>
      expect(updateMutateAsync).toHaveBeenCalledWith({ id: 10, menuFeatureIds: [1, 3] }),
    );
  });

  it("unchecking the only granted entry submits an empty set (revoke all, not additive)", async () => {
    updateMutateAsync.mockResolvedValue({ role_id: 10, permissions: [] });
    const user = userEvent.setup();
    render(<PermissionEditor role={ROLE} catalog={CATALOG} />);

    await user.click(screen.getByRole("checkbox", { name: "Dashboard" }));
    await user.click(screen.getByRole("button", { name: "Simpan Izin" }));

    await waitFor(() =>
      expect(updateMutateAsync).toHaveBeenCalledWith({ id: 10, menuFeatureIds: [] }),
    );
  });
});
