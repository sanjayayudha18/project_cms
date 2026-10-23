import type { NavBadge } from "@/components/layout/useNavBadges";
import type { NavItem } from "@/lib/config/navigation";
import { cn } from "@/lib/utils/cn";
import { Link } from "@tanstack/react-router";
import { useId, useState } from "react";

interface SidebarNavItemProps {
  item: NavItem;
  active: boolean;
  /** Mode rail: hanya ikon, badge jadi dot di pojok, label pindah ke tooltip (Req 4.3, 4.4). */
  rail: boolean;
  badge?: NavBadge;
  /** Roving tabindex dipegang `<nav>` di Sidebar.tsx. */
  tabIndex?: number;
  onFocus?: () => void;
}

/** Nama aksesibel gabungan label + badge (Req 3.6, 3.7). */
function accessibleName(label: string, badge?: NavBadge): string | undefined {
  if (!badge) return undefined;
  return badge.kind === "count" ? `${label}, ${badge.raw} menunggu` : `${label}, perlu perhatian`;
}

function BadgePill({ badge, active }: { badge: NavBadge; active: boolean }) {
  if (badge.kind === "dot") {
    return (
      <span
        aria-hidden="true"
        className="size-2 shrink-0 rounded-full bg-[var(--warning-solid)] motion-safe:animate-pulse"
      />
    );
  }
  return (
    <span
      aria-hidden="true"
      className={cn(
        "ml-auto shrink-0 rounded-full px-2 font-mono text-[12px] leading-5 tabular-nums",
        active
          ? "bg-[var(--shell-accent)] text-[var(--shell-on-accent)]"
          : "bg-[var(--shell-badge-bg)] text-[var(--shell-sb-text)]",
      )}
    >
      {badge.value}
    </span>
  );
}

/** Satu item menu Sidebar: link, state aktif, badge, dan tooltip mode rail (Req 3, 4.3, 4.4). */
export function SidebarNavItem({
  item,
  active,
  rail,
  badge,
  tabIndex,
  onFocus,
}: SidebarNavItemProps) {
  const tooltipId = useId();
  const [tipOpen, setTipOpen] = useState(false);
  const Icon = item.icon;
  const name = accessibleName(item.label, badge);

  return (
    <div className="relative">
      <Link
        to={item.href}
        data-nav-item
        tabIndex={tabIndex}
        disabled={item.disabled}
        aria-current={active ? "page" : undefined}
        aria-label={name ?? (rail ? item.label : undefined)}
        aria-describedby={rail && tipOpen ? tooltipId : undefined}
        onFocus={() => {
          onFocus?.();
          if (rail) setTipOpen(true);
        }}
        onBlur={() => setTipOpen(false)}
        onMouseEnter={() => rail && setTipOpen(true)}
        onMouseLeave={() => setTipOpen(false)}
        className={cn(
          "relative flex h-10 items-center rounded-xl text-[14px] font-medium outline-none transition-colors duration-200 motion-reduce:transition-none",
          "focus-visible:ring-2 focus-visible:ring-[var(--shell-focus)]",
          rail ? "mx-auto w-10 justify-center" : "w-full gap-3 px-3",
          active
            ? "bg-[image:var(--shell-active-bg)] font-semibold text-[var(--shell-sb-text)]"
            : "text-[var(--shell-sb-text)] hover:bg-[var(--shell-sb-hover)]",
          item.disabled && "pointer-events-none opacity-50",
        )}
      >
        {active && !rail && (
          <span
            aria-hidden="true"
            className="size-1.5 shrink-0 rounded-full bg-[var(--shell-accent)]"
          />
        )}
        <Icon
          aria-hidden="true"
          size={18}
          strokeWidth={active ? 2.5 : 2}
          className={cn("shrink-0", active && "text-[var(--shell-icon-active)]")}
        />
        {!rail && <span className="min-w-0 flex-1 truncate">{item.label}</span>}
        {!rail && badge && <BadgePill badge={badge} active={active} />}
        {rail && badge && (
          <span
            aria-hidden="true"
            className="absolute top-1.5 right-1.5 size-1.5 rounded-full bg-[var(--shell-accent)]"
          />
        )}
      </Link>
      {rail && tipOpen && (
        <span
          role="tooltip"
          id={tooltipId}
          className="pointer-events-none absolute top-1/2 left-full z-50 ml-3 -translate-y-1/2 rounded-lg bg-[var(--shell-tooltip-bg)] px-2.5 py-1.5 text-[12px] font-medium whitespace-nowrap text-[var(--shell-on-accent)]"
        >
          {name ?? item.label}
        </span>
      )}
    </div>
  );
}
