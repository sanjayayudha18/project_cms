/** Header inputs of the Vendor Request create page: forecast date, category, vendor, replenish date, notes. */

import type { UseFormReturn } from "react-hook-form";
import type { VendorRequestCategory } from "./types";
import { CATEGORY_OPTIONS, type FormValues } from "./vendorRequestCreateForm";

interface VendorOption {
  id: number;
  name: string;
}

interface VendorRequestCreateFieldsProps {
  isManual: boolean;
  forecastDate: string;
  category: VendorRequestCategory | "";
  categoryError: string | null;
  onCategoryChange: (next: VendorRequestCategory | "") => void;
  manualVendorId: number | null;
  onManualVendorChange: (id: number | null) => void;
  vendors: VendorOption[];
  replenishDate: string;
  replenishDateError: string | null;
  todayISO: string;
  onReplenishDateChange: (value: string) => void;
  form: UseFormReturn<FormValues>;
}

const LABEL_CLASS = "text-xs font-medium uppercase tracking-wider text-[var(--n-600)]";
const SELECT_CLASS =
  "min-h-[44px] min-w-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]";

export function VendorRequestCreateFields({
  isManual,
  forecastDate,
  category,
  categoryError,
  onCategoryChange,
  manualVendorId,
  onManualVendorChange,
  vendors,
  replenishDate,
  replenishDateError,
  todayISO,
  onReplenishDateChange,
  form,
}: VendorRequestCreateFieldsProps) {
  return (
    <>
      {!isManual && (
        <div className="flex flex-col gap-1">
          <span className={LABEL_CLASS}>Tanggal Forecast</span>
          <span className="text-sm text-[var(--n-800)]">{forecastDate}</span>
        </div>
      )}

      <div className="flex flex-wrap items-end gap-4">
        <div className="flex flex-col gap-1">
          <label htmlFor="request-category" className={LABEL_CLASS}>
            Kategori Request <span className="text-[var(--red-600)]">*</span>
          </label>
          <select
            id="request-category"
            value={category}
            onChange={(e) => onCategoryChange(e.target.value as VendorRequestCategory | "")}
            className={SELECT_CLASS}
          >
            <option value="" disabled>
              Pilih Kategori
            </option>
            {CATEGORY_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
          {categoryError && (
            <span className="text-xs text-[var(--danger-fg)]">{categoryError}</span>
          )}
        </div>

        {isManual && (
          <div className="flex flex-col gap-1">
            <label htmlFor="manual-vendor" className={LABEL_CLASS}>
              Vendor <span className="text-[var(--red-600)]">*</span>
            </label>
            <select
              id="manual-vendor"
              value={manualVendorId ?? ""}
              onChange={(e) => onManualVendorChange(e.target.value ? Number(e.target.value) : null)}
              className={SELECT_CLASS}
            >
              <option value="" disabled>
                Pilih Vendor
              </option>
              {vendors.map((v) => (
                <option key={v.id} value={v.id}>
                  {v.name}
                </option>
              ))}
            </select>
          </div>
        )}

        <div className="flex flex-col gap-1">
          <label htmlFor="replenish-date" className={LABEL_CLASS}>
            Tanggal Replenish <span className="text-[var(--red-600)]">*</span>
          </label>
          {category === "additional" ? (
            <input
              id="replenish-date"
              type="date"
              value={replenishDate}
              min={todayISO}
              onChange={(e) => onReplenishDateChange(e.target.value)}
              className="min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
            />
          ) : (
            // Planned/Emergency (both paths, Req 1.3/1.4) and the
            // not-yet-chosen "" state (Req 1.2) are auto-set/locked.
            <input
              id="replenish-date"
              type="date"
              value={replenishDate}
              disabled
              className="min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-100)] px-3 text-sm text-[var(--n-600)] outline-none"
            />
          )}
          {replenishDateError && (
            <span className="text-xs text-[var(--danger-fg)]">{replenishDateError}</span>
          )}
        </div>
      </div>

      <div className="flex flex-col gap-1">
        <label htmlFor="vendor-request-notes" className={LABEL_CLASS}>
          Catatan (opsional)
        </label>
        <textarea
          id="vendor-request-notes"
          {...form.register("notes")}
          maxLength={500}
          rows={3}
          className="rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 py-2 text-sm text-[var(--n-800)] outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
        />
        {form.formState.errors.notes && (
          <span className="text-xs text-[var(--danger-fg)]">
            {form.formState.errors.notes.message}
          </span>
        )}
      </div>
    </>
  );
}
