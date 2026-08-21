/**
 * Search input + status/machine_type/brand multi-selects + deployment_type
 * select + active filter count / "Clear All" (Req 5.1-5.3, 5.5, 10.4, 11.5).
 *
 * Purely presentational/controlled — receives current values and change
 * callbacks as props rather than calling useAtmPortalUrlState() itself, so
 * the parent screen (Task 10.1) is the single owner of URL state and this
 * component stays easy to test in isolation.
 *
 * `search`'s 300ms debounce already lives in useAtmPortalUrlState (Task
 * 8.2) — this component just binds to onSearchInputChange, it doesn't
 * debounce anything itself.
 *
 * Multi-select is implemented as a row of toggleable chip buttons (native
 * <button aria-pressed>), not a dropdown-with-checkboxes widget — there's
 * no existing multi-select component in this codebase to reuse, and chips
 * avoid needing to build focus-trap/outside-click handling for a custom
 * popover just for this task. deployment_type reuses the existing
 * FilterSelect (single-select, already has the 44x44 touch target baked
 * in).
 *
 * Brand options are exactly the three requirements.md Req 89.3 lists
 * (Hyosung, Wincor, Diebold) verbatim, even though the real atms table has
 * six distinct brand values (also Hitachi, OKI, NCR) — matching the quoted
 * requirement, not the live data, per this project's convention of
 * following requirements.md acceptance criteria verbatim where quoted.
 */

import { FilterSelect } from "@/components/ui/FilterSelect";
import { STATUS_BADGE_CONFIG } from "../constants";
import type { AtmPortalParams, ReplenishmentStatus } from "../types";

const SEARCH_MAX_LENGTH = 100;

const STATUS_OPTIONS: ReplenishmentStatus[] = [
  "critical",
  "low",
  "normal",
  "unconfigured",
  "no_data",
];
const MACHINE_TYPE_OPTIONS = ["ATM", "CRM", "CDM"];
const BRAND_OPTIONS = ["Hyosung", "Wincor", "Diebold"];
const DEPLOYMENT_TYPE_OPTIONS = [
  { value: "ONSITE", label: "ONSITE" },
  { value: "OFFSITE", label: "OFFSITE" },
];

function toggleCommaValue(current: string, option: string): string {
  const values = current === "" ? [] : current.split(",");
  const next = values.includes(option) ? values.filter((v) => v !== option) : [...values, option];
  return next.join(",");
}

interface ChipGroupProps {
  legend: string;
  options: readonly string[];
  labelFor: (option: string) => string;
  value: string;
  onChange: (next: string) => void;
}

function ChipGroup({ legend, options, labelFor, value, onChange }: ChipGroupProps) {
  const selected = value === "" ? [] : value.split(",");
  return (
    <fieldset className="flex flex-col gap-1">
      <legend className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]">
        {legend}
      </legend>
      <div className="flex flex-wrap gap-2">
        {options.map((option) => {
          const isSelected = selected.includes(option);
          return (
            <button
              key={option}
              type="button"
              aria-pressed={isSelected}
              onClick={() => onChange(toggleCommaValue(value, option))}
              className={`min-h-[44px] rounded-[var(--radius-md)] border px-3 text-sm font-medium outline-none focus-visible:ring-2 focus-visible:ring-[var(--red-100)] ${
                isSelected
                  ? "border-[var(--red-400)] bg-[var(--red-50)] text-[var(--red-700)]"
                  : "border-[var(--n-300)] bg-[var(--n-0)] text-[var(--n-800)]"
              }`}
            >
              {labelFor(option)}
            </button>
          );
        })}
      </div>
    </fieldset>
  );
}

interface FilterBarProps {
  searchInput: string;
  onSearchInputChange: (value: string) => void;
  status: string;
  machineType: string;
  brand: string;
  deploymentType: string;
  onFilterChange: (
    partial: Partial<
      Pick<AtmPortalParams, "status" | "machine_type" | "brand" | "deployment_type">
    >,
  ) => void;
  onClearAll: () => void;
}

export function FilterBar({
  searchInput,
  onSearchInputChange,
  status,
  machineType,
  brand,
  deploymentType,
  onFilterChange,
  onClearAll,
}: FilterBarProps) {
  const activeFilterCount = [
    status !== "all" && status !== "",
    machineType !== "",
    brand !== "",
    deploymentType !== "",
  ].filter(Boolean).length;

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-1">
        <label
          htmlFor="atm-portal-search"
          className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]"
        >
          Cari berdasarkan Terminal ID atau lokasi
        </label>
        <input
          id="atm-portal-search"
          type="text"
          value={searchInput}
          onChange={(e) => onSearchInputChange(e.target.value.slice(0, SEARCH_MAX_LENGTH))}
          maxLength={SEARCH_MAX_LENGTH}
          placeholder="Cari berdasarkan Terminal ID atau lokasi"
          className="min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
        />
      </div>

      <ChipGroup
        legend="Status"
        options={STATUS_OPTIONS}
        labelFor={(option) => STATUS_BADGE_CONFIG[option as ReplenishmentStatus].label}
        value={status === "all" ? "" : status}
        onChange={(next) => onFilterChange({ status: next === "" ? "all" : next })}
      />

      <ChipGroup
        legend="Machine Type"
        options={MACHINE_TYPE_OPTIONS}
        labelFor={(option) => option}
        value={machineType}
        onChange={(next) => onFilterChange({ machine_type: next })}
      />

      <ChipGroup
        legend="Brand"
        options={BRAND_OPTIONS}
        labelFor={(option) => option}
        value={brand}
        onChange={(next) => onFilterChange({ brand: next })}
      />

      <FilterSelect
        label="Deployment Type"
        options={DEPLOYMENT_TYPE_OPTIONS}
        value={deploymentType === "" ? null : deploymentType}
        onChange={(value) => onFilterChange({ deployment_type: value ?? "" })}
      />

      <div className="flex items-center gap-3">
        <span className="text-sm text-[var(--n-600)]">
          {activeFilterCount > 0 ? `${activeFilterCount} filter aktif` : "Tidak ada filter aktif"}
        </span>
        {activeFilterCount > 0 && (
          <button
            type="button"
            onClick={onClearAll}
            className="min-h-[44px] rounded-[var(--radius-md)] px-3 text-sm font-medium text-[var(--red-600)] outline-none focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
          >
            Clear All
          </button>
        )}
      </div>
    </div>
  );
}
