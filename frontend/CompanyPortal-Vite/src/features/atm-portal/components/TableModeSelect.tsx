/**
 * Accessible native select for ATM Portal table mode: ATM Replenish | ATM Cashpos.
 */

import type { AtmPortalMode } from "../types";

interface TableModeSelectProps {
  mode: AtmPortalMode;
  onModeChange: (mode: AtmPortalMode) => void;
}

export function TableModeSelect({ mode, onModeChange }: TableModeSelectProps) {
  return (
    <div className="flex flex-col gap-1 sm:max-w-xs">
      <label htmlFor="atm-portal-table-mode" className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]">
        Tampilan tabel
      </label>
      <select
        id="atm-portal-table-mode"
        value={mode}
        onChange={(e) => onModeChange(e.target.value as AtmPortalMode)}
        className="min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-200)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-900)] outline-none focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
      >
        <option value="replenish">ATM Replenish</option>
        <option value="cashpos">ATM Cashpos</option>
      </select>
    </div>
  );
}
