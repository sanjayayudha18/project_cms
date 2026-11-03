import { create } from "zustand";
import { type StateStorage, createJSONStorage, persist } from "zustand/middleware";
import { type NavGroup, SIDEBAR_GROUP_ORDER } from "../config/navigation";

// ─── Types ────────────────────────────────────────────────────────────────────

export type SidebarTheme = "dark" | "light";

export interface ShellPrefsState {
  rail: boolean;
  sidebarTheme: SidebarTheme;
  collapsedGroups: NavGroup[];
}

interface ShellPrefsActions {
  toggleRail: () => void;
  toggleTheme: () => void;
  toggleGroup: (group: NavGroup) => void;
}

export type ShellPrefsStore = ShellPrefsState & ShellPrefsActions;

// ─── Constants ────────────────────────────────────────────────────────────────

export const SHELL_PREFS_KEY = "crown.shell.v1";

export const DEFAULT_SHELL_PREFS: ShellPrefsState = {
  rail: false,
  sidebarTheme: "dark",
  collapsedGroups: [],
};

// ─── Sanitize ─────────────────────────────────────────────────────────────────

/** Normalisasi nilai tak tepercaya (localStorage) menjadi prefs valid; salah tipe → default per field. */
export function sanitizeShellPrefs(raw: unknown): ShellPrefsState {
  const src = typeof raw === "object" && raw !== null ? (raw as Record<string, unknown>) : {};
  const groups = Array.isArray(src.collapsedGroups) ? src.collapsedGroups : [];
  return {
    rail: typeof src.rail === "boolean" ? src.rail : DEFAULT_SHELL_PREFS.rail,
    sidebarTheme:
      src.sidebarTheme === "dark" || src.sidebarTheme === "light"
        ? src.sidebarTheme
        : DEFAULT_SHELL_PREFS.sidebarTheme,
    // filter di atas SIDEBAR_GROUP_ORDER: buang nilai asing + dedupe sekaligus
    collapsedGroups: SIDEBAR_GROUP_ORDER.filter((g) => groups.includes(g)),
  };
}

// ─── Storage ──────────────────────────────────────────────────────────────────

/** localStorage yang menelan error akses (private mode / diblokir). */
export const safeLocalStorage: StateStorage = {
  getItem: (name) => {
    try {
      return localStorage.getItem(name);
    } catch {
      return null;
    }
  },
  setItem: (name, value) => {
    try {
      localStorage.setItem(name, value);
    } catch {
      // storage tidak tersedia: preferensi hanya berlaku di sesi ini
    }
  },
  removeItem: (name) => {
    try {
      localStorage.removeItem(name);
    } catch {
      // abaikan
    }
  },
};

// ─── Store ────────────────────────────────────────────────────────────────────

export const useShellPrefs = create<ShellPrefsStore>()(
  persist(
    (set) => ({
      ...DEFAULT_SHELL_PREFS,
      toggleRail: () => set((s) => ({ rail: !s.rail })),
      toggleTheme: () =>
        set((s) => ({ sidebarTheme: s.sidebarTheme === "dark" ? "light" : "dark" })),
      toggleGroup: (group) =>
        set((s) => ({
          collapsedGroups: s.collapsedGroups.includes(group)
            ? s.collapsedGroups.filter((g) => g !== group)
            : [...s.collapsedGroups, group],
        })),
    }),
    {
      name: SHELL_PREFS_KEY,
      version: 1,
      storage: createJSONStorage(() => safeLocalStorage),
      partialize: (s) => ({
        rail: s.rail,
        sidebarTheme: s.sidebarTheme,
        collapsedGroups: s.collapsedGroups,
      }),
      merge: (persisted, current) => ({ ...current, ...sanitizeShellPrefs(persisted) }),
    },
  ),
);
