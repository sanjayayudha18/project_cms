import { Button } from "@/components/ui/Button";
import { FilterSelect } from "@/components/ui/FilterSelect";
import { useEffect, useState } from "react";
import type { AuditLogFilters } from "../types";

// ponytail: fixed lists of the actions/entities written today; a value not listed
// here can still be filtered via the URL. Serve the distinct values from the API
// if these lists start to churn.
const ACTION_OPTIONS = [
  "create",
  "submit",
  "approve",
  "reject",
  "revise",
  "cancel",
  "admin_set_hierarchy",
].map((v) => ({ value: v, label: v }));

const ENTITY_TYPE_OPTIONS = [
  "vendor_request",
  "approval_request",
  "master_data_change_request",
  "user",
].map((v) => ({ value: v, label: v }));

const INPUT_CLASS =
  "min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]";
const LABEL_CLASS = "text-xs font-medium uppercase tracking-wider text-[var(--n-600)]";
const DIGITS_ONLY = /^\d+$/;

/** True when both dates are set and from is after to (YYYY-MM-DD compares correctly as a string). */
export function isDateRangeInvalid(from: string | null, to: string | null): boolean {
  return from !== null && to !== null && from > to;
}

interface AuditFilterBarProps {
  filters: AuditLogFilters;
  onFiltersChange: (partial: Partial<AuditLogFilters>) => void;
  onReset: () => void;
}

/** Action / Entity type / Actor / date-range filters (Req 6.1-6.5). Invalid ranges never reach the URL. */
export function AuditFilterBar({ filters, onFiltersChange, onReset }: AuditFilterBarProps) {
  const [actorInput, setActorInput] = useState(filters.actor_id ?? "");
  const [dateFrom, setDateFrom] = useState(filters.date_from ?? "");
  const [dateTo, setDateTo] = useState(filters.date_to ?? "");

  useEffect(() => setActorInput(filters.actor_id ?? ""), [filters.actor_id]);
  useEffect(() => setDateFrom(filters.date_from ?? ""), [filters.date_from]);
  useEffect(() => setDateTo(filters.date_to ?? ""), [filters.date_to]);

  const rangeInvalid = isDateRangeInvalid(dateFrom || null, dateTo || null);

  function commitActor(): void {
    const value = actorInput.trim();
    if (value === (filters.actor_id ?? "")) return;
    const valid = DIGITS_ONLY.test(value);
    onFiltersChange({ actor_id: valid ? value : null });
    if (!valid) setActorInput("");
  }

  function changeDate(key: "date_from" | "date_to", value: string): void {
    const from = key === "date_from" ? value : dateFrom;
    const to = key === "date_to" ? value : dateTo;
    if (key === "date_from") setDateFrom(value);
    else setDateTo(value);
    if (isDateRangeInvalid(from || null, to || null)) return;
    onFiltersChange({ [key]: value || null });
  }

  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-wrap items-end gap-4">
        <FilterSelect
          label="Aksi"
          options={ACTION_OPTIONS}
          value={filters.action}
          onChange={(value) => onFiltersChange({ action: value })}
        />
        <FilterSelect
          label="Tipe Entitas"
          options={ENTITY_TYPE_OPTIONS}
          value={filters.entity_type}
          onChange={(value) => onFiltersChange({ entity_type: value })}
        />

        <div className="flex flex-col gap-1">
          <label htmlFor="audit-actor" className={LABEL_CLASS}>
            ID Aktor
          </label>
          <input
            id="audit-actor"
            type="text"
            inputMode="numeric"
            value={actorInput}
            onChange={(e) => setActorInput(e.target.value)}
            onBlur={commitActor}
            onKeyDown={(e) => e.key === "Enter" && commitActor()}
            placeholder="Contoh: 3"
            className={`${INPUT_CLASS} w-32 tabular-nums`}
          />
        </div>

        <div className="flex flex-col gap-1">
          <label htmlFor="audit-date-from" className={LABEL_CLASS}>
            Dari Tanggal
          </label>
          <input
            id="audit-date-from"
            type="date"
            value={dateFrom}
            aria-invalid={rangeInvalid}
            onChange={(e) => changeDate("date_from", e.target.value)}
            className={INPUT_CLASS}
          />
        </div>

        <div className="flex flex-col gap-1">
          <label htmlFor="audit-date-to" className={LABEL_CLASS}>
            Sampai Tanggal
          </label>
          <input
            id="audit-date-to"
            type="date"
            value={dateTo}
            aria-invalid={rangeInvalid}
            onChange={(e) => changeDate("date_to", e.target.value)}
            className={INPUT_CLASS}
          />
        </div>

        <Button variant="secondary" onClick={onReset}>
          Atur Ulang
        </Button>
      </div>

      {rangeInvalid && (
        <p role="alert" className="text-sm text-[var(--danger-fg)]">
          Tanggal awal tidak boleh setelah tanggal akhir.
        </p>
      )}
    </div>
  );
}
