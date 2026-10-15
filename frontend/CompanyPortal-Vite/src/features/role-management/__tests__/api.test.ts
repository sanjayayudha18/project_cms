import { beforeEach, describe, expect, it, vi } from "vitest";

// fe-coverage C2: role-management fetchers (hooks.test.tsx mocks this module).
const client = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn() }));
vi.mock("@/lib/api/client", () => ({ api: client }));

import { createRole, getCatalog, getRoles, updateRolePermissions } from "../api";
import type { CreateRoleFormValues } from "../types";

beforeEach(() => {
  client.get.mockReset();
  client.post.mockReset();
  client.put.mockReset();
});

describe("role-management api", () => {
  it("reads roles and the permission catalog", async () => {
    client.get.mockResolvedValueOnce({ data: { roles: [] } });
    client.get.mockResolvedValueOnce({ data: { menus: [] } });

    await expect(getRoles()).resolves.toEqual({ roles: [] });
    await expect(getCatalog()).resolves.toEqual({ menus: [] });

    expect(client.get.mock.calls.map((c) => c[0])).toEqual([
      "/admin/roles",
      "/admin/roles/catalog",
    ]);
  });

  it("creates a role with the form values", async () => {
    const values = { role: "AUDITOR", description: "Audit" } as unknown as CreateRoleFormValues;
    client.post.mockResolvedValue({ data: { id: 11, role: "AUDITOR" } });

    await expect(createRole(values)).resolves.toEqual({ id: 11, role: "AUDITOR" });
    expect(client.post).toHaveBeenCalledWith("/admin/roles", values);
  });

  it("replaces a role's permissions with menu_feature_ids", async () => {
    client.put.mockResolvedValue({ data: { role_id: 7, permissions: [1, 2] } });

    await expect(updateRolePermissions(7, [1, 2])).resolves.toEqual({
      role_id: 7,
      permissions: [1, 2],
    });
    expect(client.put).toHaveBeenCalledWith("/admin/roles/7/permissions", {
      menu_feature_ids: [1, 2],
    });
  });
});
