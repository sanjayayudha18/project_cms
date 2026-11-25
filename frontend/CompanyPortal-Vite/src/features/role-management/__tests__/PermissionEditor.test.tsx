import { useAuthStore, usePermissionStore } from "@/lib/auth";
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
  { id: 1, parent_id: null, key: "monitoring", label: "Dashboard", kind: "menu", sort_order: 0 },
  { id: 2, parent_id: null, key: "settings", label: "Pengaturan", kind: "menu", sort_order: 1 },
  {
    id: 3,
    parent_id: 2,
    key: "settings.vendors",
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
    { menu_feature_id: 1, parent_id: null, key: "monitoring", label: "Dashboard", kind: "menu" },
  ],
};

const toastMock = vi.fn();
vi.mock("@/lib/hooks/useToast", () => ({ useToast: () => ({ toast: toastMock }) }));

const loadMock = vi.fn();

beforeEach(() => {
  updateMutateAsync.mockReset();
  toastMock.mockReset();
  loadMock.mockReset().mockResolvedValue(undefined);
  usePermissionStore.setState({ load: loadMock });
  useAuthStore.setState({ user: null });
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

  describe("permission catalog coverage (spec 7.2)", () => {
    const FIXED_CATALOG: CatalogEntry[] = [
      ...CATALOG,
      {
        id: 4,
        parent_id: 2,
        key: "approvals",
        label: "Persetujuan",
        kind: "feature",
        sort_order: 1,
      },
      {
        id: 5,
        parent_id: 2,
        key: "settings.roles",
        label: "Peran Terkunci",
        kind: "feature",
        sort_order: 2,
      },
      {
        id: 6,
        parent_id: 2,
        key: "settings.pks",
        label: "Daftar PKS",
        kind: "feature",
        sort_order: 3,
        assignable_roles: ["ADMIN", "ATM-SPV"],
      },
    ];

    it("shows Always_On as checked + disabled with badge, Locked disabled with badge", () => {
      render(<PermissionEditor role={ROLE} catalog={FIXED_CATALOG} />);

      const alwaysOn = screen.getByRole("checkbox", { name: /Persetujuan/ });
      expect(alwaysOn).toBeChecked();
      expect(alwaysOn).toBeDisabled();
      expect(screen.getByText("Selalu aktif")).toBeInTheDocument();

      const locked = screen.getByRole("checkbox", { name: /Peran Terkunci/ });
      expect(locked).not.toBeChecked();
      expect(locked).toBeDisabled();
      expect(screen.getByText("Hanya APPACCESS/ADMIN")).toBeInTheDocument();
    });

    it("disables a key outside assignable_roles and lists the allowed roles in the tooltip", () => {
      render(<PermissionEditor role={ROLE} catalog={FIXED_CATALOG} />);

      const box = screen.getByRole("checkbox", { name: /Daftar PKS/ });
      expect(box).toBeDisabled();
      expect(screen.getByText("Hanya: ADMIN, ATM-SPV")).toBeInTheDocument();
      expect(box.closest("label")).toHaveAttribute(
        "title",
        expect.stringContaining("ADMIN, ATM-SPV"),
      );
    });

    it("enables a key whose assignable_roles includes the role", () => {
      render(<PermissionEditor role={{ ...ROLE, role: "ATM-SPV" }} catalog={FIXED_CATALOG} />);
      expect(screen.getByRole("checkbox", { name: /Daftar PKS/ })).toBeEnabled();
    });

    it("PUT payload never contains Always_On/Locked ids, even when the role holds them", async () => {
      updateMutateAsync.mockResolvedValue({});
      const role: RoleWithPermissions = {
        ...ROLE,
        permissions: [
          ...ROLE.permissions,
          {
            menu_feature_id: 4,
            parent_id: 2,
            key: "approvals",
            label: "Persetujuan",
            kind: "feature",
          },
          {
            menu_feature_id: 5,
            parent_id: 2,
            key: "settings.roles",
            label: "Peran Terkunci",
            kind: "feature",
          },
        ],
      };
      const user = userEvent.setup();
      render(<PermissionEditor role={role} catalog={FIXED_CATALOG} />);

      // Fixed keys never make the set dirty.
      expect(screen.getByRole("button", { name: "Simpan Izin" })).toBeDisabled();

      await user.click(screen.getByRole("checkbox", { name: "Manajemen Peran" }));
      await user.click(screen.getByRole("button", { name: "Simpan Izin" }));

      await waitFor(() =>
        expect(updateMutateAsync).toHaveBeenCalledWith({ id: 10, menuFeatureIds: [1, 3] }),
      );
    });

    it("shows the 422 role_not_assignable message as an error toast and skips the reload", async () => {
      updateMutateAsync.mockRejectedValue({
        code: "role_not_assignable",
        message: "Layar Daftar PKS hanya untuk ADMIN",
      });
      useAuthStore.setState({ user: { role: "AUDITOR-VIEW" } as never });
      const user = userEvent.setup();
      render(<PermissionEditor role={ROLE} catalog={FIXED_CATALOG} />);

      await user.click(screen.getByRole("checkbox", { name: "Manajemen Peran" }));
      await user.click(screen.getByRole("button", { name: "Simpan Izin" }));

      await waitFor(() =>
        expect(toastMock).toHaveBeenCalledWith({
          type: "error",
          message: expect.stringContaining("Layar Daftar PKS hanya untuk ADMIN"),
        }),
      );
      expect(loadMock).not.toHaveBeenCalled();
    });

    it("reloads own permissions after saving the signed-in user's role", async () => {
      updateMutateAsync.mockResolvedValue({});
      useAuthStore.setState({ user: { role: "AUDITOR-VIEW" } as never });
      const user = userEvent.setup();
      render(<PermissionEditor role={ROLE} catalog={FIXED_CATALOG} />);

      await user.click(screen.getByRole("checkbox", { name: "Manajemen Peran" }));
      await user.click(screen.getByRole("button", { name: "Simpan Izin" }));

      await waitFor(() => expect(loadMock).toHaveBeenCalledTimes(1));
    });

    it("does not reload permissions after saving another role", async () => {
      updateMutateAsync.mockResolvedValue({});
      useAuthStore.setState({ user: { role: "ADMIN" } as never });
      const user = userEvent.setup();
      render(<PermissionEditor role={ROLE} catalog={FIXED_CATALOG} />);

      await user.click(screen.getByRole("checkbox", { name: "Manajemen Peran" }));
      await user.click(screen.getByRole("button", { name: "Simpan Izin" }));

      await waitFor(() =>
        expect(toastMock).toHaveBeenCalledWith(expect.objectContaining({ type: "success" })),
      );
      expect(loadMock).not.toHaveBeenCalled();
    });
  });
});
