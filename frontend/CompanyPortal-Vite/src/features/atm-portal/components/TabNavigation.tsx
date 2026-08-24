/**
 * WAI-ARIA tabs pattern (role="tablist"/"tab"/"tabpanel") for the ATM
 * Profile Replenish/Cashpos tabs. Active tab is fully controlled by the
 * parent (synced to the ?tab= URL param there) — this component only owns
 * keyboard navigation (ArrowLeft/Right, Home, End) and focus movement.
 * https://www.w3.org/WAI/ARIA/apg/patterns/tabs/
 */

import { useRef } from "react";
import type { AtmProfileTab } from "../types";

interface Tab {
  readonly id: AtmProfileTab;
  readonly label: string;
}

// Fixed-size tuple (not `Tab[]`) so indexed access below type-checks under
// noUncheckedIndexedAccess without needing non-null assertions — TS knows
// a tuple's length statically, unlike a general array.
const TABS: readonly [Tab, Tab] = [
  { id: "replenish", label: "Replenish" },
  { id: "cashpos", label: "Cashpos" },
];

interface TabNavigationProps {
  activeTab: AtmProfileTab;
  onTabChange: (tab: AtmProfileTab) => void;
}

export function TabNavigation({ activeTab, onTabChange }: TabNavigationProps) {
  const tabRefs = useRef<Map<AtmProfileTab, HTMLButtonElement>>(new Map());

  function focusAndActivate(tab: AtmProfileTab): void {
    onTabChange(tab);
    tabRefs.current.get(tab)?.focus();
  }

  function handleKeyDown(event: React.KeyboardEvent<HTMLButtonElement>, index: number): void {
    switch (event.key) {
      case "ArrowRight":
        event.preventDefault();
        // biome-ignore lint/style/noNonNullAssertion: TABS is a fixed 2-tuple; (index+1) % TABS.length is always 0 or 1.
        focusAndActivate(TABS[(index + 1) % TABS.length]!.id);
        break;
      case "ArrowLeft":
        event.preventDefault();
        // biome-ignore lint/style/noNonNullAssertion: TABS is a fixed 2-tuple; the mod result is always 0 or 1.
        focusAndActivate(TABS[(index - 1 + TABS.length) % TABS.length]!.id);
        break;
      case "Home":
        event.preventDefault();
        focusAndActivate(TABS[0].id);
        break;
      case "End":
        event.preventDefault();
        // biome-ignore lint/style/noNonNullAssertion: TABS is a fixed 2-tuple; TABS.length - 1 is always a valid index.
        focusAndActivate(TABS[TABS.length - 1]!.id);
        break;
      case "Enter":
      case " ":
        event.preventDefault();
        // biome-ignore lint/style/noNonNullAssertion: index is always 0 or 1 — one iteration per TABS entry in the map below.
        onTabChange(TABS[index]!.id);
        break;
      default:
        break;
    }
  }

  return (
    <div
      role="tablist"
      aria-label="Riwayat ATM"
      className="flex gap-1 border-[var(--n-200)] border-b"
    >
      {TABS.map((tab, index) => {
        const isActive = tab.id === activeTab;
        return (
          <button
            key={tab.id}
            ref={(el) => {
              if (el) {
                tabRefs.current.set(tab.id, el);
              }
            }}
            type="button"
            role="tab"
            id={`atm-profile-tab-${tab.id}`}
            aria-selected={isActive}
            aria-controls={`atm-profile-tabpanel-${tab.id}`}
            tabIndex={isActive ? 0 : -1}
            onClick={() => onTabChange(tab.id)}
            onKeyDown={(e) => handleKeyDown(e, index)}
            className={`min-h-[44px] min-w-[44px] border-b-2 px-4 text-sm font-medium outline-none transition-colors focus-visible:ring-2 focus-visible:ring-[var(--red-100)] ${
              isActive
                ? "border-[var(--red-500)] text-[var(--red-600)]"
                : "border-transparent text-[var(--n-600)] hover:text-[var(--n-900)]"
            }`}
          >
            {tab.label}
          </button>
        );
      })}
    </div>
  );
}
