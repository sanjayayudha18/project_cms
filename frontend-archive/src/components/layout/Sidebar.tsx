import { useEffect, useState } from 'react';
import { NavLink } from 'react-router-dom';
import { Menu, PanelLeftClose } from 'lucide-react';

import { NAV_GROUPS, SETTINGS_NAV } from '@/lib/constants';
import { formatBadgeCount } from '@/lib/formatters';

interface SidebarProps {
  collapsed: boolean;
  onToggle: () => void;
  mobileOpen: boolean;
  onMobileClose: () => void;
}

/**
 * Returns current time formatted as "HH:mm WIB".
 */
function getWibTime(): string {
  const now = new Date();
  const wib = new Date(now.toLocaleString('en-US', { timeZone: 'Asia/Jakarta' }));
  const hours = wib.getHours().toString().padStart(2, '0');
  const minutes = wib.getMinutes().toString().padStart(2, '0');
  return `${hours}:${minutes} WIB`;
}

/**
 * Collapsible sidebar navigation.
 * Expanded: 256px with icon + label. Collapsed: 64px icon-only.
 * Transitions within 300ms using transform + opacity (ease-out).
 * Auto-collapses to icon-only rail when viewport < 1024px.
 * Mobile: renders as fixed overlay when mobileOpen is true.
 */
export function Sidebar({ collapsed, onToggle, mobileOpen, onMobileClose }: SidebarProps) {
  const [currentTime, setCurrentTime] = useState(getWibTime);

  // Update time every minute
  useEffect(() => {
    const interval = setInterval(() => {
      setCurrentTime(getWibTime());
    }, 60_000);
    return () => clearInterval(interval);
  }, []);

  // Auto-collapse on viewport < 1024px
  useEffect(() => {
    const mql = window.matchMedia('(max-width: 1023px)');

    function handleChange(e: MediaQueryListEvent | MediaQueryList) {
      if (e.matches && !collapsed) {
        onToggle();
      }
    }

    // Check on mount
    handleChange(mql);

    mql.addEventListener('change', handleChange);
    return () => mql.removeEventListener('change', handleChange);
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const sidebarContent = (
    <aside
      className="
        flex h-full flex-col border-r border-[var(--n-200)]
        bg-[var(--n-0)] transition-all duration-300 ease-[cubic-bezier(0.22,1,0.36,1)]
      "
      style={{ width: collapsed ? 64 : 256 }}
    >
      {/* Brand Mark + Toggle */}
      <div className="flex h-[60px] items-center gap-[var(--space-3)] border-b border-[var(--n-200)] px-[var(--space-2)]">
        {/* Brand Mark: CN logo */}
        <div
          className="flex shrink-0 items-center justify-center rounded-[var(--radius-md)] bg-[var(--red-500)] text-[0.875rem] font-bold text-white"
          style={{ width: 38, height: 38 }}
          aria-hidden="true"
        >
          CN
        </div>

        {/* Brand text - hidden when collapsed */}
        <div
          className="overflow-hidden whitespace-nowrap transition-[opacity,width] duration-300 ease-[cubic-bezier(0.22,1,0.36,1)]"
          style={{
            opacity: collapsed ? 0 : 1,
            width: collapsed ? 0 : 'auto',
          }}
        >
          <span className="block text-sm font-semibold text-[var(--n-900)]">CodexCash</span>
          <span className="block text-[0.7rem] text-[var(--n-500)]">ATM &amp; CIT</span>
        </div>

        {/* Toggle button - pushed to end */}
        <button
          type="button"
          onClick={onToggle}
          aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
          className="
            ml-auto flex h-[32px] w-[32px] items-center justify-center
            rounded-[var(--radius-md)] text-[var(--n-600)]
            transition-all duration-150 ease-out
            hover:bg-[var(--n-50)]
            focus-visible:outline-none focus-visible:shadow-[var(--focus-ring)]
          "
          style={{
            opacity: collapsed ? 0 : 1,
            width: collapsed ? 0 : 32,
            pointerEvents: collapsed ? 'none' : 'auto',
          }}
        >
          <PanelLeftClose size={18} />
        </button>
      </div>

      {/* Expand button shown when collapsed */}
      {collapsed && (
        <div className="flex justify-center py-[var(--space-2)]">
          <button
            type="button"
            onClick={onToggle}
            aria-label="Expand sidebar"
            className="
              flex h-[32px] w-[32px] items-center justify-center
              rounded-[var(--radius-md)] text-[var(--n-600)]
              transition-all duration-150 ease-out
              hover:bg-[var(--n-50)]
              focus-visible:outline-none focus-visible:shadow-[var(--focus-ring)]
            "
          >
            <Menu size={18} />
          </button>
        </div>
      )}

      {/* Navigation */}
      <nav className="flex flex-1 flex-col gap-[var(--space-1)] overflow-y-auto p-[var(--space-2)]">
        {NAV_GROUPS.map((group) => (
          <div key={group.label} className="flex flex-col gap-[var(--space-1)]">
            {!collapsed && (
              <span className="px-[var(--space-3)] pt-[var(--space-3)] pb-[var(--space-1)] text-[0.7rem] font-[800] uppercase tracking-[0.09em] text-[var(--n-500)]">
                {group.label}
              </span>
            )}
            {group.items.map((item) => (
              <NavLink
                key={item.path}
                to={item.path}
                onClick={mobileOpen ? onMobileClose : undefined}
                className={({ isActive }) =>
                  `
                  flex h-[44px] items-center gap-[var(--space-3)]
                  rounded-[var(--radius-md)] px-[var(--space-3)]
                  transition-all duration-150 ease-out
                  focus-visible:outline-none focus-visible:shadow-[var(--focus-ring)]
                  ${
                    isActive
                      ? 'bg-[var(--red-50)] font-medium text-[var(--red-600)]'
                      : 'text-[var(--n-600)] hover:bg-[var(--n-50)]'
                  }
                `.trim()
                }
              >
                <item.icon
                  size={20}
                  className="shrink-0"
                  aria-hidden="true"
                />
                <span
                  className="
                    overflow-hidden whitespace-nowrap
                    transition-[opacity,width] duration-300 ease-[cubic-bezier(0.22,1,0.36,1)]
                  "
                  style={{
                    opacity: collapsed ? 0 : 1,
                    width: collapsed ? 0 : 'auto',
                  }}
                >
                  {item.label}
                </span>
                {!collapsed && (() => {
                  const badge = formatBadgeCount(item.badge ?? 0);
                  if (!badge) return null;
                  return (
                    <span className="ml-auto rounded-full bg-[var(--red-50)] px-2 py-0.5 text-[0.7rem] font-semibold text-[var(--red-600)]">
                      {badge}
                    </span>
                  );
                })()}
              </NavLink>
            ))}
          </div>
        ))}

        {/* Settings - separated by divider + spacing */}
        <div className="mt-auto border-t border-[var(--n-200)] pt-[var(--space-4)]">
          <NavLink
            to={SETTINGS_NAV.path}
            onClick={mobileOpen ? onMobileClose : undefined}
            className={({ isActive }) =>
              `
              flex h-[44px] items-center gap-[var(--space-3)]
              rounded-[var(--radius-md)] px-[var(--space-3)]
              transition-all duration-150 ease-out
              focus-visible:outline-none focus-visible:shadow-[var(--focus-ring)]
              ${
                isActive
                  ? 'bg-[var(--red-50)] font-medium text-[var(--red-600)]'
                  : 'text-[var(--n-600)] hover:bg-[var(--n-50)]'
              }
            `.trim()
            }
          >
            <SETTINGS_NAV.icon
              size={20}
              className="shrink-0"
              aria-hidden="true"
            />
            <span
              className="
                overflow-hidden whitespace-nowrap
                transition-[opacity,width] duration-300 ease-[cubic-bezier(0.22,1,0.36,1)]
              "
              style={{
                opacity: collapsed ? 0 : 1,
                width: collapsed ? 0 : 'auto',
              }}
            >
              {SETTINGS_NAV.label}
            </span>
          </NavLink>
        </div>
      </nav>

      {/* System status note - hidden when collapsed */}
      {!collapsed && (
        <div className="border-t border-[var(--n-200)] px-[var(--space-3)] py-[var(--space-3)]">
          <p className="text-xs text-[var(--n-500)]">
            All systems operational
          </p>
          <p className="text-xs text-[var(--n-500)]">
            {currentTime}
          </p>
        </div>
      )}
    </aside>
  );

  return (
    <>
      {/* Desktop sidebar - always visible */}
      <div className="hidden lg:block h-full">
        {sidebarContent}
      </div>

      {/* Mobile overlay */}
      {mobileOpen && (
        <div className="fixed inset-0 z-20 lg:hidden">
          {/* Backdrop */}
          <div
            className="absolute inset-0 bg-black/30"
            onClick={onMobileClose}
            aria-hidden="true"
          />
          {/* Sidebar panel */}
          <div className="relative h-full">
            {sidebarContent}
          </div>
        </div>
      )}
    </>
  );
}
