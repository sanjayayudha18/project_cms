import type { NavItem } from "@/lib/config/navigation";
import type { SidebarTheme } from "@/lib/shell/prefs";
import { cn } from "@/lib/utils/cn";
import { Link } from "@tanstack/react-router";
import { Moon, Settings2, Sun } from "lucide-react";
import type { ComponentType, ReactNode } from "react";

type IconType = ComponentType<{ className?: string; "aria-hidden"?: boolean }>;

interface TopbarIconButtonProps {
  /** Nama aksesibel wajib: tombol hanya berisi ikon (Req 10.1). */
  label: string;
  icon: IconType;
  /** Diisi → dirender sebagai `Link` (Pengaturan); kosong → `button`. */
  to?: string;
  onClick?: () => void;
  /** Hanya untuk varian link: tint 8% + `aria-current="page"` (Req 10.3). */
  active?: boolean;
}

const base =
  "flex size-10 shrink-0 items-center justify-center rounded-xl outline-none focus-visible:ring-2 focus-visible:ring-[var(--shell-focus)]";
const idle =
  "text-[var(--shell-muted)] hover:bg-[var(--shell-canvas)] hover:text-[var(--shell-ink)]";
const activeStyle = "bg-[var(--shell-accent-tint)] text-[var(--shell-accent)]";

/** Tombol ikon 40×40 topbar (Req 10.1); varian link untuk Pengaturan. */
export function TopbarIconButton({
  label,
  icon: Icon,
  to,
  onClick,
  active,
}: TopbarIconButtonProps) {
  const className = cn(base, active ? activeStyle : idle);
  const icon = <Icon aria-hidden={true} className="size-5" />;

  if (to) {
    return (
      <Link
        to={to}
        aria-label={label}
        aria-current={active ? "page" : undefined}
        className={className}
      >
        {icon}
      </Link>
    );
  }
  return (
    <button type="button" aria-label={label} onClick={onClick} className={className}>
      {icon}
    </button>
  );
}

interface TopbarIconGroupProps {
  sidebarTheme: SidebarTheme;
  toggleTheme: () => void;
  /** `visible.settings`; undefined → tombol Pengaturan tidak dirender (Req 10.2). */
  settings: NavItem | undefined;
  settingsActive: boolean;
  /** Slot `NotificationBell` (Req 10.4). */
  children?: ReactNode;
}

/** Grup ikon topbar: tema, Pengaturan, notifikasi, lalu divider 1×32px (Req 6.2, 10.1–10.3). */
export function TopbarIconGroup({
  sidebarTheme,
  toggleTheme,
  settings,
  settingsActive,
  children,
}: TopbarIconGroupProps) {
  const isDark = sidebarTheme === "dark";
  return (
    <div className="flex items-center gap-1">
      <TopbarIconButton
        label={isDark ? "Ganti ke sidebar terang" : "Ganti ke sidebar gelap"}
        icon={isDark ? Sun : Moon}
        onClick={toggleTheme}
      />
      {settings && (
        <TopbarIconButton
          label={settings.label}
          icon={Settings2}
          to={settings.href}
          active={settingsActive}
        />
      )}
      {children}
      <span aria-hidden="true" className="mx-2 h-8 w-px bg-[var(--shell-topbar-line)]" />
    </div>
  );
}
