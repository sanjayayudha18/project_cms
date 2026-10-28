import { SidebarNavItem } from "@/components/layout/sidebar/SidebarNavItem";
import type { NavItem } from "@/lib/config/navigation";
import { cn } from "@/lib/utils/cn";
import { ChevronsLeft, ChevronsRight } from "lucide-react";

interface SidebarFooterProps {
  /** Mode rail: link Pengaturan dan tombol tinggal ikon (Req 1.5). */
  rail: boolean;
  onToggleRail: () => void;
  /** `visible.settings`; undefined → link Pengaturan tidak dirender (Req 1.2). */
  settings?: NavItem;
  settingsActive: boolean;
}

/** Footer Sidebar: link Pengaturan + tombol ciutkan/lebarkan menu (Req 1.2–1.6, 3.1). */
export function SidebarFooter({
  rail,
  onToggleRail,
  settings,
  settingsActive,
}: SidebarFooterProps) {
  const Icon = rail ? ChevronsRight : ChevronsLeft;
  const label = rail ? "Lebarkan menu" : "Ciutkan menu";
  return (
    <div
      className={cn(
        "flex shrink-0 flex-col gap-2 border-t border-[var(--shell-sb-line)] p-2",
        rail && "items-center",
      )}
    >
      {settings && <SidebarNavItem item={settings} active={settingsActive} rail={rail} />}
      <button
        type="button"
        aria-label={label}
        aria-keyshortcuts="["
        title={label}
        onClick={onToggleRail}
        className={cn(
          "flex items-center gap-2 rounded-lg px-3 py-2 text-xs font-semibold text-[var(--shell-sb-text)] outline-none hover:bg-[var(--shell-sb-hover)] focus-visible:ring-2 focus-visible:ring-[var(--shell-focus)]",
          rail ? "justify-center" : "w-full",
        )}
      >
        <Icon aria-hidden="true" className="size-4 shrink-0" />
        {!rail && (
          <>
            <span>{label}</span>
            <kbd className="ml-auto rounded border border-[var(--shell-sb-line)] px-1.5 font-mono text-[10px] text-[var(--shell-sb-group)]">
              [
            </kbd>
          </>
        )}
      </button>
    </div>
  );
}
