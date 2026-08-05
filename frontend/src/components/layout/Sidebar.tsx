import { useCallback, useMemo, useRef, useState } from "react";
import { useAuthStore } from "@/lib/auth/store";
import {
  type NavGroup,
  type NavItem,
  NAV_CONFIG,
  GROUP_LABELS,
  filterNavByRoles,
} from "@/lib/config/navigation";

// ─── Types ────────────────────────────────────────────────────────────────────

interface SidebarProps {
  collapsed: boolean;
  onToggle: () => void;
  currentPath?: string;
  onNavigate?: (href: string) => void;
}

// ─── Group ordering ───────────────────────────────────────────────────────────

const GROUP_ORDER: NavGroup[] = [
  "general",
  "forecasting",
  "invoice",
  "cash-count",
];

// ─── Sidebar Component ────────────────────────────────────────────────────────

export function Sidebar({
  collapsed,
  onToggle: _onToggle,
  currentPath = "/",
  onNavigate,
}: SidebarProps) {
  const user = useAuthStore((s) => s.user);
  const userRoles = user?.roles ?? [];

  const visibleItems = useMemo(
    () => filterNavByRoles(NAV_CONFIG, userRoles),
    [userRoles],
  );

  const groupedItems = useMemo(() => {
    const groups = new Map<NavGroup, NavItem[]>();
    for (const group of GROUP_ORDER) {
      const items = visibleItems.filter((item) => item.group === group);
      if (items.length > 0) {
        groups.set(group, items);
      }
    }
    return groups;
  }, [visibleItems]);

  const flatItems = useMemo(() => {
    const result: NavItem[] = [];
    for (const group of GROUP_ORDER) {
      const items = groupedItems.get(group);
      if (items) result.push(...items);
    }
    return result;
  }, [groupedItems]);

  const [focusedIndex, setFocusedIndex] = useState(-1);
  const navRef = useRef<HTMLElement>(null);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (flatItems.length === 0) return;

      switch (e.key) {
        case "ArrowDown": {
          e.preventDefault();
          setFocusedIndex((prev) => {
            const next = prev < flatItems.length - 1 ? prev + 1 : 0;
            focusItemAtIndex(next);
            return next;
          });
          break;
        }
        case "ArrowUp": {
          e.preventDefault();
          setFocusedIndex((prev) => {
            const next = prev > 0 ? prev - 1 : flatItems.length - 1;
            focusItemAtIndex(next);
            return next;
          });
          break;
        }
        case "Enter":
        case " ": {
          e.preventDefault();
          if (focusedIndex >= 0 && focusedIndex < flatItems.length) {
            const item = flatItems[focusedIndex];
            if (item && !item.disabled) {
              onNavigate?.(item.href);
            }
          }
          break;
        }
      }
    },
    [flatItems, focusedIndex, onNavigate],
  );

  const focusItemAtIndex = (index: number) => {
    const nav = navRef.current;
    if (!nav) return;
    const buttons = nav.querySelectorAll<HTMLElement>("[data-nav-item]");
    buttons[index]?.focus();
  };

  const handleItemClick = useCallback(
    (item: NavItem) => {
      if (item.disabled) return;
      onNavigate?.(item.href);
    },
    [onNavigate],
  );

  return (
    <nav
      ref={navRef}
      className="flex flex-1 flex-col gap-[var(--space-2)] px-[var(--space-3)] py-[var(--space-4)]"
      aria-label="Menu navigasi"
      onKeyDown={handleKeyDown}
    >
      {GROUP_ORDER.map((group, groupIdx) => {
        const items = groupedItems.get(group);
        if (!items) return null;

        return (
          <div
            key={group}
            role="group"
            aria-label={GROUP_LABELS[group]}
            className={groupIdx > 0 ? "mt-[var(--space-2)]" : ""}
          >
            {/* Group header */}
            {!collapsed && (
              <span
                className="block px-[var(--space-3)] pb-[var(--space-2)] pt-[var(--space-3)] text-[11px] font-semibold uppercase tracking-[0.08em]"
                style={{ color: "var(--n-400)" }}
                aria-hidden="true"
              >
                {GROUP_LABELS[group]}
              </span>
            )}

            {/* Collapsed: thin divider between groups */}
            {collapsed && groupIdx > 0 && (
              <div
                className="mx-auto mb-[var(--space-2)] w-6"
                style={{ height: "1px", backgroundColor: "var(--n-200)" }}
                aria-hidden="true"
              />
            )}

            {/* Nav items */}
            <div className="flex flex-col gap-[2px]">
              {items.map((item) => {
                const isActive = currentPath === item.href;
                const globalIndex = flatItems.indexOf(item);

                return (
                  <NavItemButton
                    key={item.id}
                    item={item}
                    isActive={isActive}
                    collapsed={collapsed}
                    tabIndex={globalIndex === focusedIndex ? 0 : -1}
                    onClick={() => handleItemClick(item)}
                    onFocus={() => setFocusedIndex(globalIndex)}
                  />
                );
              })}
            </div>
          </div>
        );
      })}
    </nav>
  );
}

// ─── NavItem Button ───────────────────────────────────────────────────────────

interface NavItemButtonProps {
  item: NavItem;
  isActive: boolean;
  collapsed: boolean;
  tabIndex: number;
  onClick: () => void;
  onFocus: () => void;
}

function NavItemButton({
  item,
  isActive,
  collapsed,
  tabIndex,
  onClick,
  onFocus,
}: NavItemButtonProps) {
  const Icon = item.icon;
  const [showTooltip, setShowTooltip] = useState(false);

  return (
    <div className="relative">
      <button
        type="button"
        data-nav-item
        tabIndex={tabIndex}
        onClick={onClick}
        onFocus={onFocus}
        onMouseEnter={() => collapsed && setShowTooltip(true)}
        onMouseLeave={() => setShowTooltip(false)}
        disabled={item.disabled}
        aria-current={isActive ? "page" : undefined}
        aria-disabled={item.disabled}
        className={[
          "flex w-full items-center gap-[var(--space-3)] rounded-[var(--radius-md)] text-[13px] font-medium",
          collapsed ? "justify-center h-10 w-10 mx-auto" : "px-[var(--space-3)] py-[var(--space-2)]",
        ].join(" ")}
        style={{
          color: isActive
            ? "var(--red-600)"
            : item.disabled
              ? "var(--n-400)"
              : "var(--n-600)",
          backgroundColor: isActive ? "var(--red-50)" : "transparent",
          cursor: item.disabled ? "not-allowed" : "pointer",
          borderLeft: isActive && !collapsed ? "3px solid var(--red-500)" : "3px solid transparent",
        }}
        onMouseOver={(e) => {
          if (!isActive && !item.disabled) {
            e.currentTarget.style.backgroundColor = "var(--n-100)";
            e.currentTarget.style.color = "var(--n-800)";
          }
        }}
        onMouseOut={(e) => {
          if (!isActive && !item.disabled) {
            e.currentTarget.style.backgroundColor = "transparent";
            e.currentTarget.style.color = "var(--n-600)";
          }
        }}
      >
        <Icon
          size={18}
          aria-hidden="true"
          style={{
            flexShrink: 0,
            color: isActive
              ? "var(--red-500)"
              : item.disabled
                ? "var(--n-300)"
                : "currentColor",
          }}
        />
        {!collapsed && (
          <span className="flex flex-1 items-center justify-between truncate">
            <span>{item.label}</span>
            {item.disabled && (
              <span
                className="rounded-full px-[5px] py-[1px] text-[10px] font-medium"
                style={{
                  backgroundColor: "var(--n-100)",
                  color: "var(--n-400)",
                }}
              >
                Soon
              </span>
            )}
          </span>
        )}
      </button>

      {/* Tooltip for collapsed state */}
      {collapsed && showTooltip && (
        <div
          role="tooltip"
          className="absolute left-full top-1/2 z-50 ml-[var(--space-2)] -translate-y-1/2 whitespace-nowrap rounded-[var(--radius-md)] px-[var(--space-3)] py-[var(--space-1)] text-xs font-medium"
          style={{
            backgroundColor: "var(--n-800)",
            color: "var(--n-0)",
            boxShadow: "var(--shadow-md)",
          }}
        >
          {item.label}
          {item.disabled && " (Segera Hadir)"}
        </div>
      )}
    </div>
  );
}
