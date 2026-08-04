import { useRef, useEffect } from 'react';
import { Search, Bell, Menu } from 'lucide-react';

import { getInitials } from '@/lib/formatters';

interface TopBarProps {
  onMenuClick: () => void;
}

/**
 * Top navigation bar with search, notifications, and profile section.
 * Desktop (≥760px): 72px height, full profile display.
 * Mobile (<760px): 64px height, hamburger menu, avatar only.
 *
 * @validates Requirements 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7
 */
export function TopBar({ onMenuClick }: TopBarProps) {
  const searchInputRef = useRef<HTMLInputElement>(null);

  // Hard-coded user data for prototype
  const userName = 'Radens';
  const userRole = 'Operations Lead';
  const hasUnread = true;

  // Cmd+K / Ctrl+K keyboard shortcut to focus search
  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        searchInputRef.current?.focus();
      }
    }

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, []);

  return (
    <header
      className="
        flex h-16 min-[760px]:h-[72px] items-center justify-between
        border-b border-[var(--n-200)] bg-[var(--n-0)]
        px-4 min-[760px]:px-6
      "
    >
      {/* Left section: hamburger (mobile) + search */}
      <div className="flex items-center gap-3">
        {/* Hamburger - mobile only */}
        <button
          type="button"
          onClick={onMenuClick}
          aria-label="Open menu"
          className="
            flex min-[760px]:hidden h-[44px] w-[44px] items-center justify-center
            rounded-[var(--radius-md)] text-[var(--n-600)]
            transition-colors duration-150 ease-out
            hover:bg-[var(--n-50)]
            focus-visible:outline-none focus-visible:[box-shadow:var(--focus-ring)]
          "
        >
          <Menu size={20} aria-hidden="true" />
        </button>

        {/* Search input */}
        <div className="relative">
          <Search
            size={16}
            className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--n-400)] pointer-events-none"
            aria-hidden="true"
          />
          <input
            ref={searchInputRef}
            type="search"
            placeholder="Search ATM, vendor, invoice..."
            aria-label="Search ATM, vendor, invoice"
            className="
              h-[44px] min-w-[180px] min-[760px]:min-w-[240px] w-full
              rounded-[var(--radius-md)] border border-[var(--n-300)]
              bg-[var(--n-0)] pl-9 pr-14 text-sm text-[var(--n-800)]
              placeholder:text-[var(--n-400)]
              transition-all duration-150 ease-out
              focus:border-[var(--red-400)] focus:outline-none focus:[box-shadow:var(--focus-ring)]
            "
          />
          {/* Shortcut indicator - hidden on mobile */}
          <kbd
            className="
              hidden min-[760px]:flex
              absolute right-3 top-1/2 -translate-y-1/2
              items-center rounded border border-[var(--n-300)]
              bg-[var(--n-50)] px-1.5 py-0.5
              text-[11px] font-medium text-[var(--n-500)]
              pointer-events-none select-none
            "
          >
            ⌘K
          </kbd>
        </div>
      </div>

      {/* Right section: notifications + profile */}
      <div className="flex items-center gap-2 min-[760px]:gap-3">
        {/* Notification bell */}
        <button
          type="button"
          aria-label={hasUnread ? 'Notifications (unread)' : 'Notifications'}
          className="
            relative flex h-[44px] w-[44px] items-center justify-center
            rounded-[var(--radius-md)] text-[var(--n-600)]
            transition-colors duration-150 ease-out
            hover:bg-[var(--n-50)]
            focus-visible:outline-none focus-visible:[box-shadow:var(--focus-ring)]
          "
        >
          <Bell size={20} aria-hidden="true" />
          {hasUnread && (
            <span
              className="absolute top-2.5 right-2.5 h-2 w-2 rounded-full bg-[var(--red-500)]"
              aria-hidden="true"
            />
          )}
        </button>

        {/* Profile section */}
        <div className="flex items-center gap-2.5 min-[760px]:gap-3">
          {/* Avatar with initials */}
          <div
            className="
              flex h-9 w-9 shrink-0 items-center justify-center
              rounded-full bg-[var(--red-50)] text-xs font-semibold text-[var(--red-600)]
              select-none
            "
            aria-label={`Avatar for ${userName}`}
          >
            {getInitials(userName)}
          </div>

          {/* Name + role - hidden on mobile */}
          <div className="hidden min-[760px]:flex flex-col">
            <span className="text-sm font-semibold leading-tight text-[var(--n-800)]">
              {userName}
            </span>
            <span className="text-xs leading-tight text-[var(--n-500)]">
              {userRole}
            </span>
          </div>
        </div>
      </div>
    </header>
  );
}
