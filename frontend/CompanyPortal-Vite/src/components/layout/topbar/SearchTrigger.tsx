import { Search } from "lucide-react";
import type { Ref } from "react";

interface SearchTriggerProps {
  onOpen: () => void;
  /** Dipakai AppShell untuk mengembalikan fokus saat palette ditutup (Req 8.3). */
  ref?: Ref<HTMLButtonElement>;
}

interface NavigatorWithUAData extends Navigator {
  userAgentData?: { platform?: string };
}

/** Deteksi Mac sekali; aman bila `navigator` tidak ada (SSR/test non-DOM). */
function detectMac(): boolean {
  if (typeof navigator === "undefined") return false;
  const nav = navigator as NavigatorWithUAData;
  const platform = nav.userAgentData?.platform ?? nav.platform ?? "";
  return /mac|iphone|ipad/i.test(platform);
}

const IS_MAC = detectMac();

/** Tombol pembuka command palette di topbar (Req 8.1, 8.3). */
export function SearchTrigger({ onOpen, ref }: SearchTriggerProps) {
  return (
    <button
      ref={ref}
      type="button"
      aria-haspopup="dialog"
      aria-keyshortcuts="Control+K Meta+K"
      onClick={onOpen}
      className="ml-auto flex h-10 w-[260px] shrink-0 items-center gap-2 rounded-xl border border-[var(--shell-input-line)] bg-[var(--shell-canvas)] px-3 text-sm text-[var(--shell-muted)] outline-none hover:border-[var(--shell-muted)] focus-visible:ring-2 focus-visible:ring-[var(--shell-focus)] 2xl:w-[340px]"
    >
      <Search aria-hidden="true" className="size-4 shrink-0" />
      <span className="flex-1 truncate text-left">Cari menu atau fitur…</span>
      <kbd className="shrink-0 rounded-md border border-[var(--shell-input-line)] px-1.5 py-0.5 font-sans text-xs font-semibold text-[var(--shell-muted)]">
        {IS_MAC ? "⌘ K" : "Ctrl K"}
      </kbd>
    </button>
  );
}
