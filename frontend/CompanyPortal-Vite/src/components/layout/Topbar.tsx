import type { NavItem } from "@/lib/config/navigation";
import type { SidebarTheme } from "@/lib/shell/prefs";
import type { ReactNode, Ref } from "react";
import { Breadcrumb } from "./topbar/Breadcrumb";
import { OpsStatusStrip } from "./topbar/OpsStatusStrip";
import { SearchTrigger } from "./topbar/SearchTrigger";
import { TopbarIconGroup } from "./topbar/TopbarIconButton";
import { UserMenu } from "./topbar/UserMenu";

export interface TopbarProps {
  /** `findActiveNavItem(visible.all, pathname)`; undefined → breadcrumb tidak dirender. */
  active: NavItem | undefined;
  sidebarTheme: SidebarTheme;
  onToggleTheme: () => void;
  onOpenPalette: () => void;
  /** Ref SearchTrigger agar AppShell bisa mengembalikan fokus saat palette ditutup (Req 8.3). */
  searchRef?: Ref<HTMLButtonElement>;
  /** Slot `NotificationBell` (Req 10.4); Topbar tidak butuh data provider. */
  notifications?: ReactNode;
}

/** Topbar 64px menggantikan `Header` (Req 7–11, D6: solid, tanpa blur). */
export function Topbar({
  active,
  sidebarTheme,
  onToggleTheme,
  onOpenPalette,
  searchRef,
  notifications,
}: TopbarProps) {
  return (
    <header className="sticky top-0 z-30 flex h-16 shrink-0 items-center gap-3 border-b border-[var(--shell-topbar-line)] bg-[var(--shell-topbar-bg)] px-6">
      <Breadcrumb active={active} />
      <SearchTrigger ref={searchRef} onOpen={onOpenPalette} />
      <OpsStatusStrip />
      <TopbarIconGroup sidebarTheme={sidebarTheme} toggleTheme={onToggleTheme}>
        {notifications}
      </TopbarIconGroup>
      <UserMenu />
    </header>
  );
}
