import { SidebarBrand } from "@/components/layout/sidebar/SidebarBrand";
import { SidebarFooter } from "@/components/layout/sidebar/SidebarFooter";
import { SidebarNavGroup } from "@/components/layout/sidebar/SidebarNavGroup";
import { SidebarNavItem } from "@/components/layout/sidebar/SidebarNavItem";
import type { NavBadge } from "@/components/layout/useNavBadges";
import {
  GROUP_LABELS,
  type NavBadgeSource,
  type NavGroup,
  type NavItem,
  SIDEBAR_GROUP_ORDER,
  type VisibleNav,
} from "@/lib/config/navigation";
import type { SidebarTheme } from "@/lib/shell/prefs";
import { cn } from "@/lib/utils/cn";
import { type CSSProperties, type KeyboardEvent, useRef, useState } from "react";

export const SIDEBAR_WIDTH_EXPANDED = "264px";
export const SIDEBAR_WIDTH_RAIL = "76px";

export interface SidebarProps {
  visible: VisibleNav;
  active?: NavItem;
  badges: Partial<Record<NavBadgeSource, NavBadge>>;
  /** Rail efektif (prefs.rail || viewport sempit), dihitung AppShell. */
  rail: boolean;
  theme: SidebarTheme;
  collapsedGroups: readonly NavGroup[];
  onToggleGroup: (group: NavGroup) => void;
  onToggleRail: () => void;
}

/** Sidebar CROWN: merakit brand, grup menu, dan footer; semua data datang lewat props (Req 2.1, 4.1, 6.1). */
export function Sidebar({
  visible,
  active,
  badges,
  rail,
  theme,
  collapsedGroups,
  onToggleGroup,
  onToggleRail,
}: SidebarProps) {
  const navRef = useRef<HTMLElement>(null);
  const [focusedId, setFocusedId] = useState<string | null>(null);

  const groups = SIDEBAR_GROUP_ORDER.flatMap((group) => {
    const items = visible.sidebar.get(group);
    return items && items.length > 0 ? [{ group, items }] : [];
  });

  // Hanya item yang benar-benar bisa difokus (grup terlipat hanya menyisakan item aktif).
  const focusableIds = groups.flatMap(({ group, items }) =>
    !rail && collapsedGroups.includes(group)
      ? items.filter((i) => i.id === active?.id).map((i) => i.id)
      : items.map((i) => i.id),
  );
  const tabStopId =
    focusedId && focusableIds.includes(focusedId)
      ? focusedId
      : active && focusableIds.includes(active.id)
        ? active.id
        : focusableIds[0];

  const handleKeyDown = (e: KeyboardEvent<HTMLElement>) => {
    if (!["ArrowDown", "ArrowUp", "Home", "End"].includes(e.key)) return;
    const links = Array.from(
      navRef.current?.querySelectorAll<HTMLElement>("[data-nav-item]") ?? [],
    ).filter((el) => !el.closest("[inert]"));
    if (links.length === 0) return;
    e.preventDefault();
    const current = links.indexOf(document.activeElement as HTMLElement);
    const last = links.length - 1;
    let next: number;
    if (e.key === "Home") next = 0;
    else if (e.key === "End") next = last;
    else if (e.key === "ArrowDown") next = current >= last ? 0 : current + 1;
    else next = current <= 0 ? last : current - 1;
    links[next]?.focus();
  };

  return (
    <aside
      data-sidebar-theme={theme}
      aria-label="Navigasi utama"
      style={
        {
          "--sb-w": rail ? SIDEBAR_WIDTH_RAIL : SIDEBAR_WIDTH_EXPANDED,
          width: "var(--sb-w)",
        } as CSSProperties
      }
      className={cn(
        "shell-sidebar row-span-2 flex h-full min-h-0 flex-col overflow-hidden",
        "border-r border-[var(--shell-sb-line)] bg-[var(--shell-sb-bg)]",
        "transition-[width] duration-300 ease-[cubic-bezier(0.22,1,0.36,1)] motion-reduce:transition-none",
      )}
    >
      <SidebarBrand rail={rail} />
      <nav
        ref={navRef}
        aria-label="Menu navigasi"
        onKeyDown={handleKeyDown}
        className="shell-scroll min-h-0 flex-1 overflow-y-auto px-3 py-2"
      >
        {groups.map(({ group, items }) => (
          <SidebarNavGroup
            key={group}
            label={GROUP_LABELS[group]}
            items={items}
            getKey={(item) => item.id}
            isActive={(item) => item.id === active?.id}
            rail={rail}
            collapsed={collapsedGroups.includes(group)}
            onToggle={() => onToggleGroup(group)}
            renderItem={(item) => (
              <SidebarNavItem
                item={item}
                active={item.id === active?.id}
                rail={rail}
                badge={item.badge ? badges[item.badge] : undefined}
                tabIndex={item.id === tabStopId ? 0 : -1}
                onFocus={() => setFocusedId(item.id)}
              />
            )}
          />
        ))}
      </nav>
      <SidebarFooter rail={rail} onToggleRail={onToggleRail} />
    </aside>
  );
}
