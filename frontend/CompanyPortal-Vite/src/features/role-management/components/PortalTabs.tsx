import { type KeyboardEvent, useRef } from "react";
import type { Portal } from "../types";

export const PORTAL_TABS: { portal: Portal; label: string }[] = [
  { portal: "main", label: "Portal Utama" },
  { portal: "vendor", label: "Portal Cabang" },
];

export const tabId = (p: Portal) => `portal-tab-${p}`;
export const panelId = (p: Portal) => `portal-panel-${p}`;

interface PortalTabsProps {
  value: Portal;
  counts: Record<Portal, number>;
  onChange: (portal: Portal) => void;
}

/** ARIA tablist with roving tabindex; ←/→/Home/End move and activate (Req 5.1). */
export function PortalTabs({ value, counts, onChange }: PortalTabsProps) {
  const refs = useRef<Record<string, HTMLButtonElement | null>>({});

  function handleKeyDown(e: KeyboardEvent<HTMLDivElement>) {
    const idx = PORTAL_TABS.findIndex((t) => t.portal === value);
    let next = idx;
    if (e.key === "ArrowRight") next = (idx + 1) % PORTAL_TABS.length;
    else if (e.key === "ArrowLeft") next = (idx - 1 + PORTAL_TABS.length) % PORTAL_TABS.length;
    else if (e.key === "Home") next = 0;
    else if (e.key === "End") next = PORTAL_TABS.length - 1;
    else return;
    e.preventDefault();
    const target = PORTAL_TABS[next];
    if (!target) return;
    onChange(target.portal);
    refs.current[target.portal]?.focus();
  }

  return (
    <div
      role="tablist"
      aria-label="Portal"
      onKeyDown={handleKeyDown}
      className="flex gap-1 border-b border-[var(--n-200)]"
    >
      {PORTAL_TABS.map((tab) => {
        const selected = tab.portal === value;
        return (
          <button
            key={tab.portal}
            ref={(el) => {
              refs.current[tab.portal] = el;
            }}
            type="button"
            role="tab"
            id={tabId(tab.portal)}
            aria-selected={selected}
            aria-controls={panelId(tab.portal)}
            tabIndex={selected ? 0 : -1}
            onClick={() => onChange(tab.portal)}
            className={`-mb-px inline-flex min-h-[44px] items-center gap-2 border-b-2 px-4 text-sm font-medium outline-none transition-colors focus-visible:ring-2 focus-visible:ring-[var(--red-100)] ${
              selected
                ? "border-[var(--red-500)] text-[var(--n-900)]"
                : "border-transparent text-[var(--n-600)] hover:text-[var(--n-900)]"
            }`}
          >
            {tab.label}
            <span className="rounded-full bg-[var(--n-100)] px-2 text-xs tabular-nums text-[var(--n-700)]">
              {counts[tab.portal]}
            </span>
          </button>
        );
      })}
    </div>
  );
}
