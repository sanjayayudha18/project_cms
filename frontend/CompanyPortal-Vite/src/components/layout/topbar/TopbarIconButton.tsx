import type { SidebarTheme } from "@/lib/shell/prefs";
import { cn } from "@/lib/utils/cn";
import { Moon, Sun } from "lucide-react";
import type { ComponentType, ReactNode } from "react";

type IconType = ComponentType<{ className?: string; "aria-hidden"?: boolean }>;

interface TopbarIconButtonProps {
  /** Nama aksesibel wajib: tombol hanya berisi ikon (Req 10.1). */
  label: string;
  icon: IconType;
  onClick?: () => void;
}

const base =
  "flex size-10 shrink-0 items-center justify-center rounded-xl outline-none focus-visible:ring-2 focus-visible:ring-[var(--shell-focus)]";
const idle =
  "text-[var(--shell-muted)] hover:bg-[var(--shell-canvas)] hover:text-[var(--shell-ink)]";

/** Tombol ikon 40×40 topbar (Req 10.1). */
export function TopbarIconButton({ label, icon: Icon, onClick }: TopbarIconButtonProps) {
  return (
    <button type="button" aria-label={label} onClick={onClick} className={cn(base, idle)}>
      <Icon aria-hidden={true} className="size-5" />
    </button>
  );
}

interface TopbarIconGroupProps {
  sidebarTheme: SidebarTheme;
  toggleTheme: () => void;
  /** Slot `NotificationBell` (Req 10.4). */
  children?: ReactNode;
}

/** Grup ikon topbar: tema, notifikasi, lalu divider 1×32px (Req 6.2, 10.1). */
export function TopbarIconGroup({ sidebarTheme, toggleTheme, children }: TopbarIconGroupProps) {
  const isDark = sidebarTheme === "dark";
  return (
    <div className="flex items-center gap-1">
      <TopbarIconButton
        label={isDark ? "Ganti ke sidebar terang" : "Ganti ke sidebar gelap"}
        icon={isDark ? Sun : Moon}
        onClick={toggleTheme}
      />
      {children}
      <span aria-hidden="true" className="mx-2 h-8 w-px bg-[var(--shell-topbar-line)]" />
    </div>
  );
}
