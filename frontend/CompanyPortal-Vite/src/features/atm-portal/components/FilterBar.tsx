/**
 * Search input + status/machine_type/brand multi-selects + deployment_type
 * select + date range + active filter count / "Clear All".
 * Same filter surface for both ATM Replenish and ATM Cashpos modes.
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
  dateFrom: string;
  dateTo: string;
  onFilterChange: (
    partial: Partial<
      Pick<
        AtmPortalParams,
        "status" | "machine_type" | "brand" | "deployment_type" | "date_from" | "date_to" | "page"
      >
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
  dateFrom,
  dateTo,
  onFilterChange,
  onClearAll,
}: FilterBarProps) {
  const activeFilterCount = [
    status !== "all" && status !== "",
    machineType !== "",
    brand !== "",
    deploymentType !== "",
    dateFrom !== "",
    dateTo !== "",
  ].filter(Boolean).length;

  const searchLabel = "Cari berdasarkan Terminal ID atau lokasi";

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-1">
        <label
          htmlFor="atm-portal-search"
          className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]"
        >
          {searchLabel}
        </label>
        <input
          id="atm-portal-search"
          type="text"
          value={searchInput}
          onChange={(e) => onSearchInputChange(e.target.value.slice(0, SEARCH_MAX_LENGTH))}
          maxLength={SEARCH_MAX_LENGTH}
          placeholder={searchLabel}
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

      <div className="flex flex-wrap gap-4">
        <div className="flex flex-col gap-1">
          <label
            htmlFor="atm-portal-date-from"
            className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]"
          >
            Dari tanggal
          </label>
          <input
            id="atm-portal-date-from"
            type="date"
            value={dateFrom}
            onChange={(e) => onFilterChange({ date_from: e.target.value, page: 1 })}
            className="min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
          />
        </div>
        <div className="flex flex-col gap-1">
          <label
            htmlFor="atm-portal-date-to"
            className="text-xs font-medium uppercase tracking-wider text-[var(--n-600)]"
          >
            Sampai tanggal
          </label>
          <input
            id="atm-portal-date-to"
            type="date"
            value={dateTo}
            onChange={(e) => onFilterChange({ date_to: e.target.value, page: 1 })}
            className="min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
          />
        </div>
      </div>

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
