import { usePermissionStore } from "@/lib/auth/permissionStore";
import { useAuthStore } from "@/lib/auth/store";
import { beforeEach, describe, expect, it } from "vitest";
import { SETTINGS_HUB_KEYS, settingsRoute } from "./settings";

// permission-catalog-coverage task 6.2: /settings is open to any hub-card key.

const guard = (settingsRoute as unknown as { options: { beforeLoad: () => Promise<void> } }).options
  .beforeLoad;

function signIn() {
  useAuthStore.setState({
    user: {
      id: 1,
      username: "u",
      fullName: "U",
      email: "u@cimb.local",
      role: "ATM-SPV",
      isKaryawan: true,
      vendorId: null,
    },
    accessToken: "t",
    isAuthenticated: true,
    isAuthLoading: false,
    error: null,
    rateLimitRetryAfter: null,
  });
}

beforeEach(() => {
  signIn();
});

describe("/settings guard", () => {
  it.each(SETTINGS_HUB_KEYS)("allows a user holding only %s", async (key) => {
    usePermissionStore.setState({ keys: new Set([key]), status: "ready" });
    await expect(guard()).resolves.toBeUndefined();
  });

  it("denies a user holding only the bare settings menu key", async () => {
    usePermissionStore.setState({ keys: new Set(["settings"]), status: "ready" });
    await expect(guard()).rejects.toThrow();
  });

  it("denies when no keys are granted", async () => {
    usePermissionStore.setState({ keys: new Set(), status: "ready" });
    await expect(guard()).rejects.toThrow();
  });
});
