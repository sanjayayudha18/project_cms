import { useEffect, useRef } from "react";

/** Elemen tempat user mengetik: shortcut tombol tunggal tidak boleh aktif di sini. */
export function isEditableTarget(t: EventTarget | null): boolean {
  if (!(t instanceof Element)) return false;
  if (t.closest("input, textarea, select, [contenteditable]:not([contenteditable='false'])"))
    return true;
  return t.closest("[role='combobox']") !== null;
}

export type ShellShortcut = "palette" | "toggle-rail" | null;

/** Memetakan event keyboard ke shortcut shell (Req 4.1, 4.2, 8.2). */
export function matchShellShortcut(
  e: Pick<KeyboardEvent, "key" | "ctrlKey" | "metaKey" | "altKey" | "shiftKey" | "target">,
): ShellShortcut {
  if (e.key.toLowerCase() === "k" && (e.ctrlKey || e.metaKey)) return "palette";
  if (
    e.key === "[" &&
    !e.ctrlKey &&
    !e.metaKey &&
    !e.altKey &&
    !e.shiftKey &&
    !isEditableTarget(e.target)
  ) {
    return "toggle-rail";
  }
  return null;
}

/**
 * Satu listener keydown di window. `enabled=false` (palette terbuka) hanya mematikan
 * toggle-rail; Ctrl/⌘K tetap di-preventDefault agar browser tidak mengambilnya.
 */
export function useShellShortcuts(h: {
  onPalette(): void;
  onToggleRail(): void;
  enabled: boolean;
}): void {
  const ref = useRef(h);
  ref.current = h;
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      const hit = matchShellShortcut(e);
      if (hit === "palette") {
        e.preventDefault();
        ref.current.onPalette();
      } else if (hit === "toggle-rail" && ref.current.enabled) {
        ref.current.onToggleRail();
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, []);
}
