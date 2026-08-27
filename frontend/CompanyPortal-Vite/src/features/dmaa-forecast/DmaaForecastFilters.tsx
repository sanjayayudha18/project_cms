/**
 * DMAA Forecast filters: terminal ID search (debounced, resets page to 1)
 * and periode_pred date range (Req 4.6-4.7, 5.3-5.4).
 */

import type { DmaaForecastParams } from "./types";

const TERMINAL_MAX_LENGTH = 100;

const INPUT_CLASS =
  "min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]";

const LABEL_CLASS = "text-xs font-medium uppercase tracking-wider text-[var(--n-600)]";

interface DmaaForecastFiltersProps {
  terminalInput: string;
  onTerminalInputChange: (value: string) => void;
  dateFrom: string;
  dateTo: string;
  onFilterChange: (partial: Partial<DmaaForecastParams>) => void;
}

export function DmaaForecastFilters({
  terminalInput,
  onTerminalInputChange,
  dateFrom,
  dateTo,
  onFilterChange,
}: DmaaForecastFiltersProps) {
  return (
    <div className="flex flex-wrap items-end gap-4">
      <div className="flex flex-col gap-1">
        <label htmlFor="dmaa-terminal-search" className={LABEL_CLASS}>
          Cari Terminal ID
        </label>
        <input
          id="dmaa-terminal-search"
          type="text"
          value={terminalInput}
          onChange={(e) => onTerminalInputChange(e.target.value.slice(0, TERMINAL_MAX_LENGTH))}
          maxLength={TERMINAL_MAX_LENGTH}
          placeholder="Cari Terminal ID..."
          className={INPUT_CLASS}
        />
      </div>

      <div className="flex flex-col gap-1">
        <label htmlFor="dmaa-date-from" className={LABEL_CLASS}>
          Periode dari
        </label>
        <input
          id="dmaa-date-from"
          type="date"
          value={dateFrom}
          onChange={(e) => onFilterChange({ dateFrom: e.target.value, page: 1 })}
          className={INPUT_CLASS}
        />
      </div>

      <div className="flex flex-col gap-1">
        <label htmlFor="dmaa-date-to" className={LABEL_CLASS}>
          Periode sampai
        </label>
        <input
          id="dmaa-date-to"
          type="date"
          value={dateTo}
          onChange={(e) => onFilterChange({ dateTo: e.target.value, page: 1 })}
          className={INPUT_CLASS}
        />
      </div>
    </div>
  );
}
