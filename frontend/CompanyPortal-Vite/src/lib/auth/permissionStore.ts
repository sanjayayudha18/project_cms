import { api } from "@/lib/api/client";
import { create } from "zustand";
import { ALWAYS_ON_KEYS } from "./permissionKeys";
import { useAuthStore } from "./store";

export type PermissionStatus = "idle" | "loading" | "ready" | "error";

export interface PermissionState {
  keys: ReadonlySet<string>;
  status: PermissionStatus;
  load: () => Promise<void>;
  reset: () => void;
  has: (key: string) => boolean;
  hasAny: (keys: readonly string[]) => boolean;
}

const ALWAYS_ON: ReadonlySet<string> = new Set(ALWAYS_ON_KEYS);
const EMPTY: ReadonlySet<string> = new Set();

// Single-flight, like the auth store: session restore + login can both ask for a load.
let loadPromise: Promise<void> | null = null;

export const usePermissionStore = create<PermissionState>((set, get) => ({
  keys: EMPTY,
  status: "idle",

  load: () => {
    if (loadPromise) return loadPromise;
    set({ status: "loading" });
    loadPromise = (async () => {
      try {
        const res = await api.get<{ permissions: string[] }>("/auth/me/permissions");
        set({ keys: new Set(res.data.permissions), status: "ready" });
      } catch {
        // Only Always_On keys are honoured while in error (Req 5.5).
        set({ keys: EMPTY, status: "error" });
      }
    })().finally(() => {
      loadPromise = null;
    });
    return loadPromise;
  },

  reset: () => set({ keys: EMPTY, status: "idle" }),

  has: (key) => {
    const { status, keys } = get();
    if (status === "ready") return keys.has(key);
    return ALWAYS_ON.has(key);
  },

  hasAny: (list) => list.some((k) => get().has(k)),
}));

// Wire to the auth session without touching the auth store: load once the user is
// authenticated (login or session restore), clear on logout.
useAuthStore.subscribe((s, prev) => {
  if (s.isAuthenticated && !prev.isAuthenticated) void usePermissionStore.getState().load();
  else if (!s.isAuthenticated && prev.isAuthenticated) usePermissionStore.getState().reset();
});
