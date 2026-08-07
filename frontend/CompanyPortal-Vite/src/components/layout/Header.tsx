import type { AuthUser } from "@/lib/auth";
import { Bell, ChevronDown, LogOut, Menu, Search, User } from "lucide-react";
import { useState } from "react";

export interface HeaderProps {
  user: AuthUser;
  onLogout: () => void;
  onSidebarToggle: () => void;
  sidebarCollapsed: boolean;
}

/**
 * Header — top bar with search, notifications, and user dropdown.
 * Clean, functional design. No unnecessary decoration.
 */
export function Header({ user, onLogout, onSidebarToggle, sidebarCollapsed }: HeaderProps) {
  const [dropdownOpen, setDropdownOpen] = useState(false);

  return (
    <div className="flex flex-1 items-center justify-between gap-[var(--space-4)]">
      {/* Left section: mobile hamburger + search */}
      <div className="flex items-center gap-[var(--space-3)]">
        {/* Mobile hamburger */}
        <button
          type="button"
          onClick={onSidebarToggle}
          className="flex h-8 w-8 items-center justify-center rounded-[var(--radius-md)] lg:hidden"
          style={{ color: "var(--n-600)" }}
          aria-label={sidebarCollapsed ? "Buka menu navigasi" : "Tutup menu navigasi"}
          aria-expanded={!sidebarCollapsed}
          onMouseEnter={(e) => { e.currentTarget.style.backgroundColor = "var(--n-100)"; }}
          onMouseLeave={(e) => { e.currentTarget.style.backgroundColor = "transparent"; }}
        >
          <Menu size={20} aria-hidden="true" />
        </button>

        {/* Search bar (desktop only) */}
        <div
          className="hidden items-center gap-[var(--space-2)] rounded-[var(--radius-md)] border px-[var(--space-3)] py-[var(--space-2)] md:flex"
          style={{
            borderColor: "var(--n-200)",
            backgroundColor: "var(--n-50)",
            width: "280px",
          }}
        >
          <Search size={15} style={{ color: "var(--n-400)" }} aria-hidden="true" />
          <input
            type="text"
            placeholder="Cari menu, fitur..."
            className="w-full bg-transparent text-sm outline-none"
            style={{ color: "var(--n-700)" }}
            aria-label="Pencarian"
          />
        </div>
      </div>

      {/* Right section: notifications + user */}
      <div className="flex items-center gap-[var(--space-2)]">
        {/* Notification bell */}
        <button
          type="button"
          className="relative flex h-9 w-9 items-center justify-center rounded-[var(--radius-md)]"
          style={{ color: "var(--n-600)" }}
          aria-label="Notifikasi"
          onMouseEnter={(e) => { e.currentTarget.style.backgroundColor = "var(--n-100)"; }}
          onMouseLeave={(e) => { e.currentTarget.style.backgroundColor = "transparent"; }}
        >
          <Bell size={18} aria-hidden="true" />
          {/* Notification dot */}
          <span
            className="absolute right-[6px] top-[6px] h-2 w-2 rounded-full"
            style={{ backgroundColor: "var(--red-500)" }}
            aria-label="Ada notifikasi baru"
          />
        </button>

        {/* Divider */}
        <div
          className="mx-[var(--space-1)] h-6 w-px"
          style={{ backgroundColor: "var(--n-200)" }}
          aria-hidden="true"
        />

        {/* User dropdown */}
        <div className="relative">
          <button
            type="button"
            onClick={() => setDropdownOpen((prev) => !prev)}
            onBlur={() => setTimeout(() => setDropdownOpen(false), 150)}
            className="flex items-center gap-[var(--space-2)] rounded-[var(--radius-md)] px-[var(--space-2)] py-[var(--space-1)]"
            style={{ color: "var(--n-700)" }}
            onMouseEnter={(e) => { e.currentTarget.style.backgroundColor = "var(--n-100)"; }}
            onMouseLeave={(e) => { e.currentTarget.style.backgroundColor = "transparent"; }}
            aria-expanded={dropdownOpen}
            aria-haspopup="true"
            data-testid="header-user-name"
          >
            {/* Avatar circle */}
            <div
              className="flex h-8 w-8 items-center justify-center rounded-full"
              style={{ backgroundColor: "var(--red-50)", color: "var(--red-600)" }}
            >
              <User size={15} aria-hidden="true" />
            </div>

            {/* Name + role (hidden on small screens) */}
            <div className="hidden flex-col items-start sm:flex">
              <span className="text-sm font-medium" style={{ color: "var(--n-800)" }}>
                {user.fullName}
              </span>
              <span className="text-[11px]" style={{ color: "var(--n-500)" }}>
                {user.role}
              </span>
            </div>

            <ChevronDown
              size={14}
              className="hidden sm:block"
              style={{ color: "var(--n-400)" }}
              aria-hidden="true"
            />
          </button>

          {/* Dropdown menu */}
          {dropdownOpen && (
            <div
              className="absolute right-0 top-full z-50 mt-[var(--space-1)] w-48 rounded-[var(--radius-md)] border py-[var(--space-1)]"
              style={{
                backgroundColor: "var(--n-0)",
                borderColor: "var(--n-200)",
                boxShadow: "var(--shadow-md)",
              }}
              role="menu"
            >
              {/* User info (visible on mobile where name is hidden) */}
              <div
                className="flex flex-col px-[var(--space-3)] py-[var(--space-2)] sm:hidden"
                style={{ borderBottom: "1px solid var(--n-100)" }}
              >
                <span className="text-sm font-medium" style={{ color: "var(--n-800)" }}>
                  {user.fullName}
                </span>
                <span className="text-xs" style={{ color: "var(--n-500)" }}>
                  {user.role}
                </span>
              </div>

              {/* Role badge row */}
              <div
                className="flex items-center gap-[var(--space-2)] px-[var(--space-3)] py-[var(--space-2)]"
                style={{ borderBottom: "1px solid var(--n-100)" }}
              >
                <span
                  className="inline-flex items-center rounded-full px-[var(--space-2)] py-[2px] text-[11px] font-medium"
                  style={{ backgroundColor: "var(--red-50)", color: "var(--red-700)" }}
                  data-testid="header-role-badge"
                >
                  {user.role}
                </span>
                <span className="text-xs" style={{ color: "var(--n-500)" }}>{user.email}</span>
              </div>

              {/* Logout */}
              <button
                type="button"
                onClick={onLogout}
                className="flex w-full items-center gap-[var(--space-2)] px-[var(--space-3)] py-[var(--space-2)] text-sm"
                style={{ color: "var(--danger-fg)" }}
                onMouseEnter={(e) => { e.currentTarget.style.backgroundColor = "var(--danger-bg)"; }}
                onMouseLeave={(e) => { e.currentTarget.style.backgroundColor = "transparent"; }}
                role="menuitem"
                data-testid="header-logout-button"
                aria-label="Keluar dari sistem"
              >
                <LogOut size={15} aria-hidden="true" />
                Keluar dari sistem
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
