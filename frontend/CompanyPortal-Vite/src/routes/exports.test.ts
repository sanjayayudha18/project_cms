import { usePermissionStore } from "@/lib/auth/permissionStore";
import type { DbRole } from "@/lib/auth/store";
import { useAuthStore } from "@/lib/auth/store";
import { describe, expect, it } from "vitest";
import { ForbiddenError } from "./_protected";
import { myExportsRoute } from "./exports";

type Guard = () => Promise<void>;
const guard = (myExportsRoute as unknown as { options: { beforeLoad: Guard } }).options.beforeLoad;

function as(role: DbRole, keys: string[] = []) {
  // No-op load: signing in triggers the store's auto-load, which would overwrite these keys.
  usePermissionStore.setState({ keys: new Set(keys), status: "ready", load: async () => {} });
  useAuthStore.setState({
    user: {
      id: 1,
      username: "u",
      fullName: "U",
      email: "u@x",
      role,
      isKaryawan: true,
      vendorId: null,
    },
    accessToken: "t",
    isAuthenticated: true,
    isAuthLoading: false,
  });
}

describe("/exports route guard", () => {
  it("is mounted at /exports", () =>
    expect((myExportsRoute as unknown as { options: { path: string } }).options.path).toBe(
      "/exports",
    ));

  it.each<DbRole>([
    "ATM-USER",
    "ATM-SPV",
    "BRANCH-ATM-USER",
    "BRANCH-ATM-SPV",
    "ADMIN",
    "ADMIN_PARAM",
  ])("allows %s holding the exports key", async (role) => {
    as(role, ["exports"]);
    await expect(guard()).resolves.toBeUndefined();
  });

  it.each<DbRole>(["BRANCH-USER", "VENDOR-USER", "ACM-USER", "ADMIN"])(
    "forbids %s without the exports key",
    async (role) => {
      as(role);
      await expect(guard()).rejects.toBeInstanceOf(ForbiddenError);
    },
  );
});
