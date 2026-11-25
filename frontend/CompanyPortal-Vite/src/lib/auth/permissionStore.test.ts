import { beforeEach, describe, expect, it, vi } from "vitest";

const get = vi.fn();
vi.mock("@/lib/api/client", () => ({ api: { get: (...a: unknown[]) => get(...a) } }));

import { ALWAYS_ON_KEYS, LOCKED_KEYS, SCREEN_KEYS } from "./permissionKeys";
import { usePermissionStore } from "./permissionStore";
import { useAuthStore } from "./store";

describe("permissionKeys", () => {
  it("has unique keys and Always_On/Locked are catalog keys", () => {
    expect(new Set(SCREEN_KEYS).size).toBe(SCREEN_KEYS.length);
    for (const k of [...ALWAYS_ON_KEYS, ...LOCKED_KEYS]) expect(SCREEN_KEYS).toContain(k);
  });
});

describe("permissionStore", () => {
  beforeEach(() => {
    get.mockReset();
    usePermissionStore.getState().reset();
  });

  it("only Always_On is allowed before load", () => {
    const s = usePermissionStore.getState();
    expect(s.has("dashboard")).toBe(true);
    expect(s.has("monitoring.audit-logs")).toBe(false);
  });

  it("load → ready, has/hasAny follow the loaded keys", async () => {
    get.mockResolvedValue({ data: { permissions: ["monitoring.audit-logs"] } });
    await usePermissionStore.getState().load();
    const s = usePermissionStore.getState();
    expect(get).toHaveBeenCalledWith("/auth/me/permissions");
    expect(s.status).toBe("ready");
    expect(s.has("monitoring.audit-logs")).toBe(true);
    expect(s.hasAny(["settings.atms", "monitoring.audit-logs"])).toBe(true);
    expect(s.hasAny(["settings.atms"])).toBe(false);
  });

  it("error → only Always_On, and load can be retried", async () => {
    get.mockRejectedValueOnce(new Error("boom"));
    await usePermissionStore.getState().load();
    expect(usePermissionStore.getState().status).toBe("error");
    expect(usePermissionStore.getState().has("approvals")).toBe(true);
    expect(usePermissionStore.getState().has("settings.atms")).toBe(false);

    get.mockResolvedValueOnce({ data: { permissions: ["settings.atms"] } });
    await usePermissionStore.getState().load();
    expect(usePermissionStore.getState().has("settings.atms")).toBe(true);
  });

  it("login loads and logout clears the store", async () => {
    get.mockResolvedValue({ data: { permissions: ["settings.atms"] } });
    useAuthStore.setState({ isAuthenticated: true });
    await vi.waitFor(() => expect(usePermissionStore.getState().status).toBe("ready"));
    useAuthStore.setState({ isAuthenticated: false });
    expect(usePermissionStore.getState().status).toBe("idle");
    expect(usePermissionStore.getState().keys.size).toBe(0);
  });
});
