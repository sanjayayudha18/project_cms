import { X } from 'lucide-react';

interface DatePickerProps {
  readonly startDate: string | null;
  readonly endDate: string | null;
  readonly onStartDateChange: (date: string | null) => void;
  readonly onEndDateChange: (date: string | null) => void;
}

/**
 * Date range picker with two native date inputs (start and end) side by side.
 * Includes labels and clear buttons. Enforces 44px minimum touch target.
 */
export function DatePicker({
  startDate,
  endDate,
  onStartDateChange,
  onEndDateChange,
}: DatePickerProps) {
  return (
    <div className="flex flex-wrap items-end gap-3">
      <div className="flex flex-col gap-1">
        <label
          htmlFor="date-start"
          className="text-xs font-medium uppercase tracking-wide text-neutral-500"
        >
          Dari
        </label>
        <div className="relative flex items-center">
          <input
            id="date-start"
            type="date"
            value={startDate ?? ''}
            onChange={(e) =>
              onStartDateChange(e.target.value || null)
            }
            className="min-h-[44px] min-w-[44px] rounded-md border border-neutral-300 bg-white px-3 py-2 text-sm
                       text-surface-text
                       focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-sidebar-active
                       focus-visible:border-sidebar-active"
          />
          {startDate && (
            <button
              type="button"
              onClick={() => onStartDateChange(null)}
              aria-label="Hapus tanggal mulai"
              className="ml-1 inline-flex min-h-[44px] min-w-[44px] items-center justify-center
                         rounded-md text-neutral-400 hover:text-surface-text
                         focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-sidebar-active"
            >
              <X className="size-4" aria-hidden="true" />
            </button>
          )}
        </div>
      </div>

      <div className="flex flex-col gap-1">
        <label
          htmlFor="date-end"
          className="text-xs font-medium uppercase tracking-wide text-neutral-500"
        >
          Sampai
        </label>
        <div className="relative flex items-center">
          <input
            id="date-end"
            type="date"
            value={endDate ?? ''}
            onChange={(e) =>
              onEndDateChange(e.target.value || null)
            }
            className="min-h-[44px] min-w-[44px] rounded-md border border-neutral-300 bg-white px-3 py-2 text-sm
                       text-surface-text
                       focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-sidebar-active
                       focus-visible:border-sidebar-active"
          />
          {endDate && (
            <button
              type="button"
              onClick={() => onEndDateChange(null)}
              aria-label="Hapus tanggal akhir"
              className="ml-1 inline-flex min-h-[44px] min-w-[44px] items-center justify-center
                         rounded-md text-neutral-400 hover:text-surface-text
                         focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-sidebar-active"
            >
              <X className="size-4" aria-hidden="true" />
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
