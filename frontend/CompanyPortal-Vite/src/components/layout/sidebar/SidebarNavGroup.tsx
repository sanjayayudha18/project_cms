import { cn } from "@/lib/utils/cn";
import { ChevronDown } from "lucide-react";
import { type ReactNode, useId } from "react";

interface SidebarNavGroupProps<T> {
  label: string;
  items: readonly T[];
  getKey: (item: T) => string;
  isActive: (item: T) => boolean;
  renderItem: (item: T) => ReactNode;
  /** Mode rail: label diganti divider 24px, grup tidak bisa dilipat (Req 4.3). */
  rail: boolean;
  /** Status lipat datang dari prefs (disambungkan di Sidebar.tsx). */
  collapsed: boolean;
  onToggle: () => void;
}

/** Grup menu Sidebar: label bisa dilipat, item aktif tetap tampil saat terlipat (Req 2.5, 2.6, 4.3). */
export function SidebarNavGroup<T>({
  label,
  items,
  getKey,
  isActive,
  renderItem,
  rail,
  collapsed,
  onToggle,
}: SidebarNavGroupProps<T>) {
  const panelId = useId();

  if (rail) {
    return (
      <div className="py-1">
        <hr className="mx-auto my-2 w-6 border-0 border-t border-[var(--shell-sb-line)]" />
        <ul className="space-y-0.5">
          {items.map((item) => (
            <li key={getKey(item)}>{renderItem(item)}</li>
          ))}
        </ul>
      </div>
    );
  }

  // Terlipat: item aktif dirender di luar wrapper 0fr agar tidak ikut tersembunyi (Req 2.6).
  const pinned = collapsed ? items.filter(isActive) : [];
  const folded = collapsed ? items.filter((item) => !isActive(item)) : items;

  return (
    <div className="py-1">
      <button
        type="button"
        aria-expanded={!collapsed}
        aria-controls={panelId}
        onClick={onToggle}
        className="flex w-full items-center justify-between rounded-lg px-3 py-2 text-xs font-bold tracking-[0.12em] text-[var(--shell-sb-group)] uppercase outline-none hover:bg-[var(--shell-sb-hover)] focus-visible:ring-2 focus-visible:ring-[var(--shell-focus)]"
      >
        <span>{label}</span>
        <ChevronDown
          aria-hidden="true"
          className={cn(
            "shell-chevron size-4 shrink-0 transition-transform duration-200",
            collapsed && "-rotate-90",
          )}
        />
      </button>
      {pinned.length > 0 && (
        <ul className="space-y-0.5">
          {pinned.map((item) => (
            <li key={getKey(item)}>{renderItem(item)}</li>
          ))}
        </ul>
      )}
      <div
        id={panelId}
        inert={collapsed}
        className={cn(
          "grid transition-[grid-template-rows] duration-200 ease-out motion-reduce:transition-none",
          collapsed ? "grid-rows-[0fr]" : "grid-rows-[1fr]",
        )}
      >
        <ul className="min-h-0 space-y-0.5 overflow-hidden">
          {folded.map((item) => (
            <li key={getKey(item)}>{renderItem(item)}</li>
          ))}
        </ul>
      </div>
    </div>
  );
}
