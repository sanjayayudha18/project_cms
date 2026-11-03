import type { DbRole } from "@/lib/auth/store";
import { useAuthStore } from "@/lib/auth/store";
import { describe, expect, it } from "vitest";
import { myExportsRoute } from "./exports";

type Guard = () => { forbidden?: boolean } | undefined;
const guard = (myExportsRoute as unknown as { options: { beforeLoad: Guard } }).options.beforeLoad;

function as(role: DbRole) {
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
  ])("allows %s", (role) => {
    as(role);
    expect(guard()?.forbidden).toBeUndefined();
  });

  it.each<DbRole>(["BRANCH-USER", "VENDOR-USER", "ACM-USER"])("forbids %s", (role) => {
    as(role);
    expect(guard()).toEqual({ forbidden: true });
  });
});
